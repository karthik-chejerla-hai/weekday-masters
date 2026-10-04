package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// LedgerEntryView is one member-account movement. All-member history uses the
// same rows and per-account balances, without duplicating asset/surplus legs.
type LedgerEntryView struct {
	AccountID         uuid.UUID              `json:"-"`
	TransactionID     uuid.UUID              `json:"-"`
	CreatedAt         time.Time              `json:"-"`
	ID                uuid.UUID              `json:"id"`
	OccurredAt        time.Time              `json:"occurred_at"`
	Kind              models.TransactionKind `json:"kind"`
	Description       string                 `json:"description"`
	Category          string                 `json:"category"`
	Source            string                 `json:"source,omitempty"`
	MemberName        string                 `json:"member_name"`
	UserID            *uuid.UUID             `json:"user_id,omitempty"`
	Inactive          bool                   `json:"inactive"`
	AmountCents       int64                  `json:"amount_cents"`
	BalanceAfterCents int64                  `json:"balance_after_cents"`
	SessionID         *uuid.UUID             `json:"session_id,omitempty"`
	Reversed          bool                   `json:"reversed"`
}

// Source facts distinguish deposits from member-funded expenses and transfers.
// Ambiguous titles remain "other" instead of guessing from a positive amount.
const ledgerHistorySQL = `WITH history AS (
	SELECT e.id, e.account_id, e.transaction_id, e.created_at, t.occurred_at, t.kind, t.description,
	       a.user_id,
	       COALESCE(NULLIF(u.nickname, ''), split_part(u.name, ' ', 1),
	                regexp_replace(a.name, ' \((removed|inactive)\)$', '')) AS member_name,
	       (u.id IS NULL OR u.membership_status = 'removed') AS inactive,
	       CASE WHEN t.kind = 'splitwise_import' THEN 'splitwise' ELSE '' END AS source,
	       CASE
	         WHEN t.kind = 'reversal' THEN 'reversal'
	         WHEN t.kind = 'player_topup' THEN 'topup'
	         WHEN t.kind = 'session_settlement' OR r.is_session THEN 'session'
	         WHEN t.kind = 'withdrawal' THEN 'withdrawal'
	         WHEN t.kind = 'court_credit_purchase' THEN 'court_credit'
	         WHEN t.kind = 'shuttle_purchase' THEN 'shuttles'
	         WHEN t.kind = 'opening_balance' THEN 'opening_balance'
	         WHEN r.id IS NOT NULL THEN
	           CASE
	             WHEN lower(r.category) = 'payment' THEN
	               CASE WHEN r.club_cents < 0 AND e.amount_cents > 0 THEN 'topup'
	                    WHEN r.club_cents > 0 THEN 'withdrawal'
	                    ELSE 'transfer' END
	             WHEN lower(r.description) ~ '(^|[^a-z])top[ -]*up([^a-z]|$)'
	                  AND e.amount_cents > 0 THEN 'topup'
	             WHEN lower(r.category) IN ('dining out', 'groceries', 'liquor')
	                  OR lower(r.description) ~ '(chubby buns|f&b)' THEN 'food'
	             WHEN lower(r.category) = 'movies' THEN 'entertainment'
	             WHEN lower(r.description) LIKE '%shuttle%' THEN 'shuttles'
	             WHEN lower(r.description) LIKE '%court%' THEN 'court_credit'
	             ELSE 'other'
	           END
	         ELSE 'other'
	       END AS category,
	       e.amount_cents,
	       SUM(e.amount_cents) OVER (
	         PARTITION BY e.account_id
	         ORDER BY t.occurred_at, e.created_at, e.id
	         ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
	       ) AS balance_after_cents,
	       t.session_id,
	       EXISTS (SELECT 1 FROM transactions rev WHERE rev.reverses_transaction_id = t.id) AS reversed
	FROM ledger_entries e
	JOIN transactions t ON t.id = e.transaction_id
	JOIN accounts a ON a.id = e.account_id
	LEFT JOIN users u ON u.id = a.user_id
	LEFT JOIN splitwise_records r ON r.transaction_id = t.id
	WHERE a.kind = 'player'
)
`

type LedgerHistoryFilter struct {
	UserID     *uuid.UUID // nil includes all player accounts, including inactive ones.
	TopupsOnly bool
	Limit      int
	Offset     int
}

// Entries computes full balances first, filters second, and pages last. The
// descending page order is the exact reverse of the running-balance order.
func (s *LedgerService) Entries(filter LedgerHistoryFilter) ([]LedgerEntryView, int64, error) {
	if filter.Limit <= 0 || filter.Limit > 200 {
		filter.Limit = 50
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	where := " WHERE true"
	args := []interface{}{}
	if filter.UserID != nil {
		where += " AND user_id = ?"
		args = append(args, *filter.UserID)
	}
	if filter.TopupsOnly {
		where += " AND category = 'topup'"
	}
	var total int64
	if err := database.DB.Raw(ledgerHistorySQL+"SELECT COUNT(*) FROM history"+where, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	views := []LedgerEntryView{}
	args = append(args, filter.Limit, filter.Offset)
	err := database.DB.Raw(ledgerHistorySQL+"SELECT * FROM history"+where+
		" ORDER BY occurred_at DESC, created_at DESC, id DESC LIMIT ? OFFSET ?", args...).Scan(&views).Error
	return views, total, err
}

func (s *LedgerService) MyEntries(userID uuid.UUID, limit, offset int) ([]LedgerEntryView, int64, error) {
	return s.Entries(LedgerHistoryFilter{UserID: &userID, Limit: limit, Offset: offset})
}
