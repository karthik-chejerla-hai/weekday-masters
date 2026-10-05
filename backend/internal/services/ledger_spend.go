package services

import (
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/utils"
)

type SpendMonth struct {
	Month       int   `json:"month"`
	AmountCents int64 `json:"amount_cents"`
}

// PersonalSpend measures recorded session charges, not cash paid into the club.
type PersonalSpend struct {
	Year         int          `json:"year"`
	AsOf         string       `json:"as_of"`
	RecordedFrom *string      `json:"recorded_from"`
	YTDCents     int64        `json:"ytd_cents"`
	AllTimeCents int64        `json:"all_time_cents"`
	Months       []SpendMonth `json:"months"`
}

// Gross imported shares matter: a member can pay for the group and receive a
// net credit while still incurring their own session charge. Native charge
// lines include the member's guests, whose charges belong to their account.
// One statement gives both sources and all periods a consistent read snapshot.
const personalSpendSQL = `WITH charges AS (
	SELECT COALESCE((s.starts_at AT TIME ZONE 'Australia/Sydney')::date, s.session_date) AS played_date,
	       l.amount_cents
	FROM charge_lines l
	JOIN settlements st ON st.id = l.settlement_id
	JOIN sessions s ON s.id = st.session_id
	JOIN transactions t ON t.id = st.transaction_id
	WHERE l.user_id = @user_id AND t.kind = 'session_settlement'
	  AND st.reversed_at IS NULL
	  AND NOT EXISTS (SELECT 1 FROM transactions rev WHERE rev.reverses_transaction_id = t.id)
	UNION ALL
	SELECT r.played_date, c.charge_cents AS amount_cents
	FROM splitwise_changes c
	JOIN splitwise_participants p ON p.id = c.participant_id
	JOIN splitwise_records r ON r.id = c.record_id
	JOIN transactions t ON t.id = r.transaction_id
	WHERE p.user_id = @user_id AND r.is_session AND c.charge_cents IS NOT NULL
	  AND t.kind = 'splitwise_import'
	  AND NOT EXISTS (SELECT 1 FROM transactions rev WHERE rev.reverses_transaction_id = t.id)
)
SELECT EXTRACT(YEAR FROM played_date)::int AS year,
       EXTRACT(MONTH FROM played_date)::int AS month,
       SUM(amount_cents)::bigint AS amount_cents,
       MIN(played_date)::text AS first_date
FROM charges
WHERE played_date <= CAST(@today AS date)
GROUP BY 1, 2
ORDER BY 1, 2`

// MySpend always scopes at the source, before aggregation. now is supplied by
// the caller so calendar-boundary tests do not depend on the machine's clock.
func (s *LedgerService) MySpend(userID uuid.UUID, now time.Time) (*PersonalSpend, error) {
	now = now.In(utils.SydneyLocation)
	result := &PersonalSpend{
		Year: now.Year(), AsOf: now.Format("2006-01-02"),
		Months: make([]SpendMonth, int(now.Month())),
	}
	for i := range result.Months {
		result.Months[i].Month = i + 1
	}
	var rows []struct {
		Year        int
		Month       int
		AmountCents int64
		FirstDate   string
	}
	if err := database.DB.Raw(personalSpendSQL, map[string]interface{}{
		"user_id": userID, "today": result.AsOf,
	}).Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		if result.RecordedFrom == nil {
			first := row.FirstDate
			result.RecordedFrom = &first
		}
		result.AllTimeCents += row.AmountCents
		if row.Year == result.Year {
			result.YTDCents += row.AmountCents
			result.Months[row.Month-1].AmountCents = row.AmountCents
		}
	}
	return result, nil
}
