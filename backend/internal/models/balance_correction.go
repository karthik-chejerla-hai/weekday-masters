package models

import (
	"github.com/google/uuid"
	"time"
)

// BalanceCorrection is an immutable audit snapshot attached to a ledger transaction.
// Its request ID also makes retries safe. There is no update or delete endpoint.
type BalanceCorrection struct {
	RequestID       uuid.UUID   `gorm:"type:uuid;primaryKey" json:"request_id"`
	TransactionID   uuid.UUID   `gorm:"type:uuid;not null;uniqueIndex" json:"transaction_id"`
	AccountID       uuid.UUID   `gorm:"type:uuid;not null;index" json:"account_id"`
	AccountKind     AccountKind `gorm:"size:50;not null" json:"account_kind"`
	BeforeCents     int64       `gorm:"not null" json:"before_cents"`
	AfterCents      int64       `gorm:"not null" json:"after_cents"`
	AdjustmentCents int64       `gorm:"not null" json:"adjustment_cents"`
	Reason          string      `gorm:"type:text;not null" json:"reason"`
	CreatedBy       uuid.UUID   `gorm:"type:uuid;not null" json:"created_by"`
	ActorName       string      `gorm:"size:255;not null" json:"actor_name"`
	CreatedAt       time.Time   `gorm:"not null;index" json:"created_at"`
}
