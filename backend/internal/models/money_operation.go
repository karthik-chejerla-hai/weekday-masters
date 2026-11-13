package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MoneyOperationAction identifies a reviewed member-money submission. It is
// separate from TransactionKind: a correction appends two transactions.
type MoneyOperationAction string

const (
	MoneyOperationCorrectTopup         MoneyOperationAction = "correct_topup"
	MoneyOperationReverseTransaction   MoneyOperationAction = "reverse_transaction"
	MoneyOperationCreateSharedExpense  MoneyOperationAction = "create_shared_expense"
	MoneyOperationCorrectSharedExpense MoneyOperationAction = "correct_shared_expense"
	MoneyOperationCorrectSettlement    MoneyOperationAction = "correct_settlement"
)

// MoneyOperation durably binds one submission ID to its accepted contents and
// response. A reservation and its completed response share the caller's DB
// transaction; an incomplete reservation cannot commit. Completed rows cannot
// be changed or deleted. Payloads are internal, not a public read API.
type MoneyOperation struct {
	ID                  uuid.UUID            `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ActorID             uuid.UUID            `gorm:"type:uuid;not null" json:"-"`
	Action              MoneyOperationAction `gorm:"type:text;not null" json:"-"`
	TargetTransactionID *uuid.UUID           `gorm:"type:uuid" json:"-"`
	PayloadHash         string               `gorm:"type:char(64);not null" json:"-"`
	CanonicalPayload    string               `gorm:"type:jsonb;not null" json:"-"`
	ResponseStatus      *int16               `gorm:"type:smallint" json:"-"`
	ResponseBody        *string              `gorm:"type:jsonb" json:"-"`
	CreatedAt           time.Time            `gorm:"not null;default:clock_timestamp()" json:"created_at"`
	Sequence            int64                `gorm:"->;not null;default:nextval('money_operations_sequence')" json:"-"`
}

func (o *MoneyOperation) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}
