package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AccountKind distinguishes the club's own accounts from its players'.
//
// The three asset kinds exist separately rather than as one lumped "club"
// account because most of the club's money is not cash: it is credit prepaid at
// the venue and shuttles bought in advance. A single balance cannot answer
// whether the club is square with its players; these three together can.
type AccountKind string

const (
	AccountKindPlayer       AccountKind = "player"
	AccountKindBank         AccountKind = "bank"
	AccountKindCourtCredit  AccountKind = "court_credit"
	AccountKindShuttleStock AccountKind = "shuttle_stock"
	AccountKindSurplus      AccountKind = "surplus"
)

// ClubAccountKinds are the four singleton accounts seeded at migration time.
var ClubAccountKinds = []AccountKind{
	AccountKindBank,
	AccountKindCourtCredit,
	AccountKindShuttleStock,
	AccountKindSurplus,
}

// IsAsset reports whether this account holds something the club owns, as
// opposed to something it owes. It is the discriminator in the club-position
// identity: assets, less what players have prepaid, less surplus, is zero.
func (k AccountKind) IsAsset() bool {
	switch k {
	case AccountKindBank, AccountKindCourtCredit, AccountKindShuttleStock:
		return true
	default:
		return false
	}
}

// Account is a place a balance accumulates.
//
// There is deliberately no balance column. A balance is derived by summing the
// account's ledger entries, which makes it impossible for a cached figure to
// drift away from the entries that produced it. At this club's scale — a few
// thousand entries over years — the aggregate costs nothing. Rows exist so that
// accounts can be named, referenced by entries, and locked FOR UPDATE while
// money moves.
type Account struct {
	ID     uuid.UUID   `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Kind   AccountKind `gorm:"size:50;not null;index" json:"kind"`
	UserID *uuid.UUID  `gorm:"type:uuid;uniqueIndex" json:"user_id,omitempty"`
	Name   string      `gorm:"size:255;not null" json:"name"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (a *Account) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}
