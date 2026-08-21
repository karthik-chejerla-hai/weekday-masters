package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RSVPStatus string

const (
	RSVPStatusIn         RSVPStatus = "in"
	RSVPStatusOut        RSVPStatus = "out"
	RSVPStatusMaybe      RSVPStatus = "maybe"
	RSVPStatusWaitlisted RSVPStatus = "waitlisted"
)

type RSVP struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SessionID     uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_session_user" json:"session_id"`
	UserID        uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex:idx_session_user" json:"user_id"`
	Status        RSVPStatus `gorm:"size:50;not null" json:"status"`
	RSVPTimestamp time.Time  `gorm:"not null;default:now()" json:"rsvp_timestamp"`
	IsLateRSVP    bool       `gorm:"default:false" json:"is_late_rsvp"`
	AddedByAdmin  bool       `gorm:"default:false" json:"added_by_admin"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`

	// Associations
	Session *Session `gorm:"foreignKey:SessionID" json:"session,omitempty"`
	User    *User    `gorm:"foreignKey:UserID" json:"user,omitempty"`

	// WaitlistPosition is computed on read (1-based) and only set for waitlisted RSVPs.
	WaitlistPosition int `gorm:"-" json:"waitlist_position,omitempty"`
}

func (r *RSVP) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	if r.RSVPTimestamp.IsZero() {
		r.RSVPTimestamp = time.Now()
	}
	return nil
}
