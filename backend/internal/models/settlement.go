package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Settlement is the record of a session being costed.
//
// The rates are snapshotted rather than referenced. Club settings are defaults
// for the next settlement, not a description of past ones — change the court
// rate next year and a session settled today must still show what it actually
// cost (FR-017).
//
// ReversedAt is what makes settlement a one-way door with a way back: a live
// settlement blocks the session from being settled again, and reversing it
// stamps this column, which frees the session and leaves both the original and
// its correction visible.
type Settlement struct {
	ID            uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SessionID     uuid.UUID `gorm:"type:uuid;not null;index" json:"session_id"`
	TransactionID uuid.UUID `gorm:"type:uuid;not null" json:"transaction_id"`

	// Snapshot of the rates in force when this was settled.
	BaseHours       float64 `gorm:"type:numeric(4,2);not null" json:"base_hours"`
	BaseRateCents   int64   `gorm:"not null" json:"base_rate_cents"`
	ExtraHours      float64 `gorm:"type:numeric(4,2);not null;default:0" json:"extra_hours"`
	ExtraRateCents  int64   `gorm:"not null;default:0" json:"extra_rate_cents"`
	ShuttlesPerHour float64 `gorm:"type:numeric(4,2);not null" json:"shuttles_per_hour"`

	// What the shuttles consumed by each band were actually worth, valued from
	// stock at the moment of settlement.
	BaseShuttleCents  int64 `gorm:"not null;default:0" json:"base_shuttle_cents"`
	ExtraShuttleCents int64 `gorm:"not null;default:0" json:"extra_shuttle_cents"`
	BaseShuttleUnits  int   `gorm:"not null;default:0" json:"base_shuttle_units"`
	ExtraShuttleUnits int   `gorm:"not null;default:0" json:"extra_shuttle_units"`

	SettledAt  time.Time  `gorm:"not null" json:"settled_at"`
	SettledBy  uuid.UUID  `gorm:"type:uuid" json:"settled_by"`
	ReversedAt *time.Time `gorm:"index" json:"reversed_at,omitempty"`
	ReversedBy *uuid.UUID `gorm:"type:uuid" json:"reversed_by,omitempty"`

	CreatedAt time.Time `json:"created_at"`

	Session *Session     `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	Lines   []ChargeLine `gorm:"foreignKey:SettlementID" json:"lines,omitempty"`
}

func (s *Settlement) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.SettledAt.IsZero() {
		s.SettledAt = time.Now()
	}
	return nil
}

// IsLive reports whether this settlement still stands.
func (s *Settlement) IsLive() bool { return s.ReversedAt == nil }

// CourtCents is what the venue was charged for, across both bands.
func (s *Settlement) CourtCents() int64 {
	base := int64(s.BaseHours * float64(s.BaseRateCents))
	extra := int64(s.ExtraHours * float64(s.ExtraRateCents))
	return base + extra
}

// ShuttleCents is the value of everything taken out of the bag.
func (s *Settlement) ShuttleCents() int64 {
	return s.BaseShuttleCents + s.ExtraShuttleCents
}

// ChargeLine is one participant's part of a settlement.
//
// Collectively these are the record of who played — including lines charged at
// zero. A comped player keeps a line so that history still shows they were
// there; dropping the line would make the record of who played and the record of
// who paid the same thing, and they are not.
//
// UserID is whose account bears the charge. For a guest line that is the host,
// who settles with their guest privately.
type ChargeLine struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SettlementID uuid.UUID `gorm:"type:uuid;not null;index" json:"settlement_id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	GuestName    string    `gorm:"size:255" json:"guest_name,omitempty"`

	InBase  bool `gorm:"not null;default:true" json:"in_base"`
	InExtra bool `gorm:"not null;default:false" json:"in_extra"`
	Comped  bool `gorm:"not null;default:false" json:"comped"`

	AmountCents int64     `gorm:"not null;default:0" json:"amount_cents"`
	CreatedAt   time.Time `json:"created_at"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (l *ChargeLine) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

// IsGuest reports whether this line is a guest rather than the member themselves.
func (l *ChargeLine) IsGuest() bool { return l.GuestName != "" }
