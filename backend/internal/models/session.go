package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SessionStatus string

const (
	SessionStatusOpen      SessionStatus = "open"
	SessionStatusClosed    SessionStatus = "closed"
	SessionStatusCancelled SessionStatus = "cancelled"
)

type Session struct {
	ID                 uuid.UUID     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title              string        `gorm:"size:255;not null" json:"title"`
	Description        string        `gorm:"type:text" json:"description"`
	SessionDate        time.Time     `gorm:"type:date;not null" json:"session_date"`
	StartTime          string        `gorm:"size:10;not null" json:"start_time"` // HH:MM format
	EndTime            string        `gorm:"size:10;not null" json:"end_time"`   // HH:MM format
	Courts             int           `gorm:"not null;check:courts >= 1 AND courts <= 3" json:"courts"`
	MaxPlayers         int           `gorm:"not null" json:"max_players"`
	RSVPDeadline       time.Time     `gorm:"not null" json:"rsvp_deadline"`
	IsRecurring        bool          `gorm:"default:false" json:"is_recurring"`
	RecurringDayOfWeek *int          `json:"recurring_day_of_week"` // 0=Sunday, 1=Monday, etc.
	RecurringParentID  *uuid.UUID    `gorm:"type:uuid" json:"recurring_parent_id"`
	Status             SessionStatus `gorm:"size:50;default:'open'" json:"status"`
	CancellationReason string        `gorm:"type:text" json:"cancellation_reason,omitempty"`
	CreatedBy          uuid.UUID     `gorm:"type:uuid" json:"created_by"`
	CreatedAt          time.Time     `json:"created_at"`
	UpdatedAt          time.Time     `json:"updated_at"`

	// Associations
	RSVPs   []RSVP `gorm:"foreignKey:SessionID" json:"rsvps,omitempty"`
	Creator *User  `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.MaxPlayers = MaxPlayersForCourts(s.Courts)
	return nil
}

// MaxPlayersForCourts returns the maximum number of players based on court count
func MaxPlayersForCourts(courts int) int {
	switch courts {
	case 1:
		return 6
	case 2:
		return 10
	case 3:
		return 16
	default:
		return 6
	}
}

// IsRSVPOpen returns true if the RSVP deadline has not passed
func (s *Session) IsRSVPOpen() bool {
	return time.Now().Before(s.RSVPDeadline)
}
