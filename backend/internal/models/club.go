package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Club holds the single row of club-wide configuration, including the defaults
// used when a session is settled.
//
// These are defaults, not history. Every settlement snapshots the values it
// actually used, so changing a rate here never rewrites what a past session
// cost — see Settlement.
type Club struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name         string    `gorm:"size:255;not null" json:"name"`
	VenueName    string    `gorm:"size:255" json:"venue_name"`
	VenueAddress string    `gorm:"type:text" json:"venue_address"`

	// Settlement defaults. The club plays a standing two-hour booking on one
	// court, sometimes extending by an hour at the cheaper off-peak rate.
	BaseHours       float64 `gorm:"type:numeric(4,2);not null;default:2" json:"base_hours"`
	BaseRateCents   int64   `gorm:"not null;default:3000" json:"base_rate_cents"`
	ExtraRateCents  int64   `gorm:"not null;default:2300" json:"extra_rate_cents"`
	ShuttlesPerHour float64 `gorm:"type:numeric(4,2);not null;default:5" json:"shuttles_per_hour"`

	// Below this, and still positive, a member is told they are running low.
	LowBalanceThresholdCents int64 `gorm:"not null;default:2000" json:"low_balance_threshold_cents"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c *Club) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}
