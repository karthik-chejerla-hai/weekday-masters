package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RSVPReminderBatch records a manual action and its immutable publication counts.
// RequestID is globally unique so retries cannot create another audience.
type RSVPReminderBatch struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey" json:"id"`
	RequestID       uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"request_id"`
	SessionID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"session_id"`
	CreatedBy       uuid.UUID  `gorm:"type:uuid;not null" json:"-"`
	AudienceCount   int        `json:"audience_count"`
	QueuedCount     int        `json:"queued_count"`
	SuppressedCount int        `json:"suppressed_count"`
	NextAllowedAt   *time.Time `json:"next_allowed_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

func (b *RSVPReminderBatch) BeforeCreate(_ *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}
