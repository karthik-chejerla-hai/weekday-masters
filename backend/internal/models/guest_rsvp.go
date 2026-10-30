package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// GuestRSVP records a session participant separately from the member paying for them.
type GuestRSVP struct {
	ID              uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SessionID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"session_id"`
	GuestName       string     `gorm:"size:255;not null" json:"guest_name"`
	ChargedToUserID uuid.UUID  `gorm:"type:uuid;not null;index" json:"charged_to_user_id"`
	Status          RSVPStatus `gorm:"size:50;not null;check:guest_status,status IN ('in','out')" json:"status"`
	RSVPTimestamp   time.Time  `gorm:"not null" json:"rsvp_timestamp"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Session         *Session   `gorm:"foreignKey:SessionID" json:"-"`
	ChargedToUser   *User      `gorm:"foreignKey:ChargedToUserID" json:"charged_to_user,omitempty"`
}

func (r *GuestRSVP) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.RSVPTimestamp.IsZero() {
		r.RSVPTimestamp = time.Now()
	}
	return nil
}
