package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TransactionKind names the shape of a financial event. Each kind moves a fixed
// set of accounts; LedgerService owns those templates and is the only writer.
type TransactionKind string

const (
	TxnPlayerTopup         TransactionKind = "player_topup"
	TxnWithdrawal          TransactionKind = "withdrawal"
	TxnCourtCreditPurchase TransactionKind = "court_credit_purchase"
	TxnShuttlePurchase     TransactionKind = "shuttle_purchase"
	TxnSessionSettlement   TransactionKind = "session_settlement"
	TxnOpeningBalance      TransactionKind = "opening_balance"
	TxnReversal            TransactionKind = "reversal"
)

// Transaction is one recorded financial event.
//
// OccurredAt is when the money moved in the real world and may predate
// CreatedAt — a transfer received on Monday can be recorded on Wednesday, and
// the ledger should read in the order things actually happened.
type Transaction struct {
	ID                    uuid.UUID       `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Kind                  TransactionKind `gorm:"size:50;not null;index" json:"kind"`
	SessionID             *uuid.UUID      `gorm:"type:uuid;index" json:"session_id,omitempty"`
	ReversesTransactionID *uuid.UUID      `gorm:"type:uuid;uniqueIndex" json:"reverses_transaction_id,omitempty"`
	Description           string          `gorm:"type:text" json:"description"`
	OccurredAt            time.Time       `gorm:"not null;index" json:"occurred_at"`
	CreatedBy             uuid.UUID       `gorm:"type:uuid" json:"created_by"`
	CreatedAt             time.Time       `json:"created_at"`

	Entries []LedgerEntry `gorm:"foreignKey:TransactionID" json:"entries,omitempty"`
}

func (t *Transaction) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.OccurredAt.IsZero() {
		t.OccurredAt = time.Now()
	}
	return nil
}

// LedgerEntry is one account's part of a transaction.
//
// Append-only, per Constitution Principle VI. There is no update or delete path
// anywhere in the codebase, and there must not be one: the ledger's whole value
// is that it is a record. A mistake is corrected by posting a reversing
// transaction, which leaves both the error and the correction visible.
//
// AmountCents carries the sign that the account's own reading uses: a player in
// credit is positive, an asset held is positive. That is deliberately not the
// textbook convention, which would store liabilities negative so a naive
// SUM(amount) came to zero. The cost of that convention is a sign flip every
// reader has to remember; the cost of this one is a CASE expression in a single
// invariant check. See specs/001-club-ledger-settlement/research.md R1.
//
// Units is set only on shuttle stock entries, where the count of shuttles
// matters as much as their value. Deriving cost per shuttle from the pair is
// what lets a $50 tube of twelve stay exact.
type LedgerEntry struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TransactionID uuid.UUID `gorm:"type:uuid;not null;index" json:"transaction_id"`
	AccountID     uuid.UUID `gorm:"type:uuid;not null;index" json:"account_id"`
	AmountCents   int64     `gorm:"not null" json:"amount_cents"`
	Units         *int      `json:"units,omitempty"`
	CreatedAt     time.Time `json:"created_at"`

	Account     *Account     `gorm:"foreignKey:AccountID" json:"account,omitempty"`
	Transaction *Transaction `gorm:"foreignKey:TransactionID" json:"-"`
}

func (e *LedgerEntry) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}
