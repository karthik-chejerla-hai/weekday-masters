package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InvitationDelivery records provider submission, not inbox delivery. A pending
// attempt after a process failure is intentionally not retried automatically.
type InvitationDelivery struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	RequestID      uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"-"`
	UserID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	ActorID        uuid.UUID  `gorm:"type:uuid;not null" json:"-"`
	RecipientEmail string     `gorm:"not null" json:"recipient_email"`
	IsTest         bool       `gorm:"not null" json:"is_test"`
	Status         string     `gorm:"not null" json:"status"`
	Message        string     `json:"message"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"-"`
}

func (d *InvitationDelivery) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
