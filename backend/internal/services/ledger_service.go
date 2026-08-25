package services

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services/money"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// LedgerService is the only thing in this codebase that writes ledger entries.
//
// That is the point. Constitution Principle VII requires the club-position
// identity to hold after every write, and an invariant is only as strong as the
// narrowest gate it can be enforced at. One writer means one place to audit, and
// one place where a movement that does not balance is refused.
type LedgerService struct{}

func NewLedgerService() *LedgerService { return &LedgerService{} }

// Movement is one account's part of a proposed transaction. Amounts carry the
// sign the account itself reads: a player gaining credit is positive, an asset
// being drawn down is negative.
type Movement struct {
	AccountID   uuid.UUID
	AmountCents int64
	Units       *int // shuttle count; set only for the shuttle stock account
}

// PostInput describes a transaction to write.
type PostInput struct {
	Kind                  models.TransactionKind
	SessionID             *uuid.UUID
	ReversesTransactionID *uuid.UUID
	Description           string
	OccurredAt            time.Time
	CreatedBy             uuid.UUID
	Movements             []Movement
}

// PlayerBalance pairs a member with what they currently hold.
type PlayerBalance struct {
	UserID         uuid.UUID `json:"user_id"`
	Name           string    `json:"name"`
	ProfilePicture string    `json:"profile_picture,omitempty"`
	BalanceCents   int64     `json:"balance_cents"`
}

// Post writes a transaction and its entries in one database transaction,
// refusing anything that would break the club-position identity.
func (s *LedgerService) Post(input PostInput) (*models.Transaction, error) {
	var txn *models.Transaction
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		txn, err = s.PostWithin(tx, input)
		return err
	})
	if err != nil {
		return nil, err
	}
	return txn, nil
}

// PostWithin writes into a caller-supplied transaction, so that settlement can
// lock a session, create its own rows, and move money as a single atomic act.
//
// Accounts are locked FOR UPDATE in id order. The order matters: two settlements
// touching overlapping accounts would otherwise be free to grab them in
// opposite orders and deadlock.
func (s *LedgerService) PostWithin(tx *gorm.DB, input PostInput) (*models.Transaction, error) {
	if len(input.Movements) == 0 {
		return nil, errors.New("ledger: refusing to post a transaction that moves nothing")
	}
	if input.OccurredAt.IsZero() {
		input.OccurredAt = utils.NowInSydney()
	}

	if err := lockAccounts(tx, input.Movements); err != nil {
		return nil, err
	}

	txn := models.Transaction{
		Kind:                  input.Kind,
		SessionID:             input.SessionID,
		ReversesTransactionID: input.ReversesTransactionID,
		Description:           input.Description,
		OccurredAt:            input.OccurredAt,
		CreatedBy:             input.CreatedBy,
	}
	if err := tx.Create(&txn).Error; err != nil {
		return nil, err
	}

	for _, m := range input.Movements {
		entry := models.LedgerEntry{
			TransactionID: txn.ID,
			AccountID:     m.AccountID,
			AmountCents:   m.AmountCents,
			Units:         m.Units,
		}
		if err := tx.Create(&entry).Error; err != nil {
			return nil, err
		}
	}

	// The gate. If the books do not balance we return an error, which rolls the
	// whole transaction back — so a violated invariant cannot be persisted, only
	// refused.
	residual, err := clubPositionResidual(tx)
	if err != nil {
		return nil, err
	}
	if residual != 0 {
		return nil, ErrInvariantViolated(residual)
	}

	return &txn, nil
}

// lockAccounts takes row locks on every account a transaction touches, in a
// deterministic order.
func lockAccounts(tx *gorm.DB, movements []Movement) error {
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(movements))
	for _, m := range movements {
		if !seen[m.AccountID] {
			seen[m.AccountID] = true
			ids = append(ids, m.AccountID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })

	var locked []models.Account
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("id IN ?", ids).
		Order("id").
		Find(&locked).Error; err != nil {
		return err
	}
	if len(locked) != len(ids) {
		return fmt.Errorf("ledger: %d of %d accounts in this transaction do not exist", len(ids)-len(locked), len(ids))
	}
	return nil
}

// clubPositionResidual evaluates the identity from Principle VII:
//
//	(bank + court credit + shuttle stock) − Σ player balances − surplus = 0
//
// Assets count positively and everything the club owes counts negatively, which
// is what lets balances be stored with the sign a human would expect while still
// giving a single number that must come to zero.
func clubPositionResidual(tx *gorm.DB) (int64, error) {
	var residual int64
	err := tx.Raw(`
		SELECT COALESCE(SUM(
			CASE WHEN a.kind IN ('bank','court_credit','shuttle_stock')
			     THEN e.amount_cents
			     ELSE -e.amount_cents
			END), 0)
		FROM ledger_entries e
		JOIN accounts a ON a.id = e.account_id
	`).Scan(&residual).Error
	return residual, err
}

// --- account lookup -------------------------------------------------------

// ClubAccountID returns the singleton account of a given kind.
func (s *LedgerService) ClubAccountID(tx *gorm.DB, kind models.AccountKind) (uuid.UUID, error) {
	db := tx
	if db == nil {
		db = database.DB
	}
	var account models.Account
	if err := db.Where("kind = ?", kind).First(&account).Error; err != nil {
		return uuid.Nil, fmt.Errorf("ledger: club account %q missing — has the migration run? %w", kind, err)
	}
	return account.ID, nil
}

// EnsurePlayerAccount creates a member's account if they do not have one yet.
// Idempotent, so it is safe to call on every approval path.
func (s *LedgerService) EnsurePlayerAccount(userID uuid.UUID, name string) (uuid.UUID, error) {
	var account models.Account
	err := database.DB.Where("user_id = ?", userID).First(&account).Error
	if err == nil {
		return account.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, err
	}

	account = models.Account{
		Kind:   models.AccountKindPlayer,
		UserID: &userID,
		Name:   name,
	}
	if err := database.DB.Create(&account).Error; err != nil {
		return uuid.Nil, err
	}
	return account.ID, nil
}

// PlayerAccountID looks up a member's account, creating it if approval predated
// this feature.
func (s *LedgerService) PlayerAccountID(userID uuid.UUID) (uuid.UUID, error) {
	var account models.Account
	err := database.DB.Where("user_id = ?", userID).First(&account).Error
	if err == nil {
		return account.ID, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return uuid.Nil, err
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return uuid.Nil, err
	}
	return s.EnsurePlayerAccount(userID, user.Name)
}

// --- balances -------------------------------------------------------------

// BalanceOf sums an account's entries. There is no cached balance to drift.
func (s *LedgerService) BalanceOf(accountID uuid.UUID) (int64, error) {
	var balance int64
	err := database.DB.Raw(
		`SELECT COALESCE(SUM(amount_cents), 0) FROM ledger_entries WHERE account_id = ?`,
		accountID,
	).Scan(&balance).Error
	return balance, err
}

// BalanceOfUser reports what a member holds. A member with no entries has a
// balance of zero, which is true whether or not their account row exists yet.
func (s *LedgerService) BalanceOfUser(userID uuid.UUID) (int64, error) {
	var balance int64
	err := database.DB.Raw(`
		SELECT COALESCE(SUM(e.amount_cents), 0)
		FROM accounts a
		LEFT JOIN ledger_entries e ON e.account_id = a.id
		WHERE a.user_id = ?
	`, userID).Scan(&balance).Error
	return balance, err
}

// AllPlayerBalances lists every approved member with their balance, including
// members who have never transacted — an empty row is information too.
func (s *LedgerService) AllPlayerBalances() ([]PlayerBalance, error) {
	var balances []PlayerBalance
	err := database.DB.Raw(`
		SELECT u.id AS user_id,
		       u.name,
		       u.profile_picture,
		       COALESCE(SUM(e.amount_cents), 0) AS balance_cents
		FROM users u
		LEFT JOIN accounts a ON a.user_id = u.id
		LEFT JOIN ledger_entries e ON e.account_id = a.id
		WHERE u.membership_status = ?
		GROUP BY u.id, u.name, u.profile_picture
		ORDER BY u.name
	`, models.MembershipApproved).Scan(&balances).Error
	return balances, err
}

// StockPosition reports the shuttle bag: what it is worth and how many are in
// it. Both halves come from the same entries, so they cannot disagree.
func (s *LedgerService) StockPosition(tx *gorm.DB) (money.Stock, error) {
	db := tx
	if db == nil {
		db = database.DB
	}

	var row struct {
		ValueCents int64
		Units      int
	}
	err := db.Raw(`
		SELECT COALESCE(SUM(e.amount_cents), 0) AS value_cents,
		       COALESCE(SUM(e.units), 0)        AS units
		FROM ledger_entries e
		JOIN accounts a ON a.id = e.account_id
		WHERE a.kind = ?
	`, models.AccountKindShuttleStock).Scan(&row).Error
	if err != nil {
		return money.Stock{}, err
	}
	return money.Stock{ValueCents: row.ValueCents, Units: row.Units}, nil
}

// BalanceOfKind sums a club account by kind.
func (s *LedgerService) BalanceOfKind(tx *gorm.DB, kind models.AccountKind) (int64, error) {
	db := tx
	if db == nil {
		db = database.DB
	}

	var balance int64
	err := db.Raw(`
		SELECT COALESCE(SUM(e.amount_cents), 0)
		FROM ledger_entries e
		JOIN accounts a ON a.id = e.account_id
		WHERE a.kind = ?
	`, kind).Scan(&balance).Error
	return balance, err
}
