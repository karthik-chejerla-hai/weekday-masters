package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationType string

const (
	NotificationSessionReminder   NotificationType = "session_reminder"
	NotificationRSVPDeadline      NotificationType = "rsvp_deadline"
	NotificationWaitlistUpdate    NotificationType = "waitlist_update"
	NotificationAdminAnnouncement NotificationType = "admin_announcement"
)

// UserNotificationPreferences stores per-user notification settings
type UserNotificationPreferences struct {
	ID     uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID uuid.UUID `gorm:"type:uuid;uniqueIndex;not null" json:"user_id"`

	// Push notification preferences
	PushEnabled            bool `gorm:"default:true" json:"push_enabled"`
	PushSessionReminders   bool `gorm:"default:true" json:"push_session_reminders"`
	PushRSVPDeadlines      bool `gorm:"default:true" json:"push_rsvp_deadlines"`
	PushWaitlistUpdates    bool `gorm:"default:true" json:"push_waitlist_updates"`
	PushAdminAnnouncements bool `gorm:"default:true" json:"push_admin_announcements"`

	// Email notification preferences
	EmailEnabled            bool `gorm:"default:true" json:"email_enabled"`
	EmailSessionReminders   bool `gorm:"default:true" json:"email_session_reminders"`
	EmailRSVPDeadlines      bool `gorm:"default:true" json:"email_rsvp_deadlines"`
	EmailWaitlistUpdates    bool `gorm:"default:true" json:"email_waitlist_updates"`
	EmailAdminAnnouncements bool `gorm:"default:true" json:"email_admin_announcements"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Association
	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (p *UserNotificationPreferences) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

// UserPushToken stores FCM tokens for push notifications (one user can have multiple devices)
type UserPushToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Token      string    `gorm:"type:text;uniqueIndex;not null" json:"token"`
	DeviceName string    `gorm:"type:text" json:"device_name"`
	LastUsedAt time.Time `gorm:"default:now()" json:"last_used_at"`
	CreatedAt  time.Time `json:"created_at"`

	// Association
	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (t *UserPushToken) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.LastUsedAt.IsZero() {
		t.LastUsedAt = time.Now()
	}
	return nil
}

// Notification represents a notification sent to a user
type Notification struct {
	ID               uuid.UUID        `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID        `gorm:"type:uuid;not null;index" json:"user_id"`
	NotificationType NotificationType `gorm:"type:text;not null" json:"notification_type"`
	Title            string           `gorm:"type:text;not null" json:"title"`
	Body             string           `gorm:"type:text;not null" json:"body"`
	Data             string           `gorm:"type:jsonb" json:"data,omitempty"` // JSON string for additional payload

	PushSent    bool       `gorm:"default:false" json:"push_sent"`
	PushSentAt  *time.Time `json:"push_sent_at,omitempty"`
	EmailSent   bool       `gorm:"default:false" json:"email_sent"`
	EmailSentAt *time.Time `json:"email_sent_at,omitempty"`

	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `gorm:"index" json:"created_at"`

	// Association
	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

// Announcement represents an admin-sent announcement to all members
type Announcement struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Title     string    `gorm:"type:text;not null" json:"title"`
	Body      string    `gorm:"type:text;not null" json:"body"`
	CreatedBy uuid.UUID `gorm:"type:uuid;not null" json:"created_by"`
	SentAt    time.Time `gorm:"default:now()" json:"sent_at"`
	CreatedAt time.Time `json:"created_at"`

	// Association
	Creator *User `gorm:"foreignKey:CreatedBy" json:"creator,omitempty"`
}

func (a *Announcement) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.SentAt.IsZero() {
		a.SentAt = time.Now()
	}
	return nil
}

// IsPushEnabledForType checks if push notifications are enabled for a specific notification type
func (p *UserNotificationPreferences) IsPushEnabledForType(t NotificationType) bool {
	if !p.PushEnabled {
		return false
	}
	switch t {
	case NotificationSessionReminder:
		return p.PushSessionReminders
	case NotificationRSVPDeadline:
		return p.PushRSVPDeadlines
	case NotificationWaitlistUpdate:
		return p.PushWaitlistUpdates
	case NotificationAdminAnnouncement:
		return p.PushAdminAnnouncements
	default:
		return false
	}
}

// IsEmailEnabledForType checks if email notifications are enabled for a specific notification type
func (p *UserNotificationPreferences) IsEmailEnabledForType(t NotificationType) bool {
	if !p.EmailEnabled {
		return false
	}
	switch t {
	case NotificationSessionReminder:
		return p.EmailSessionReminders
	case NotificationRSVPDeadline:
		return p.EmailRSVPDeadlines
	case NotificationWaitlistUpdate:
		return p.EmailWaitlistUpdates
	case NotificationAdminAnnouncement:
		return p.EmailAdminAnnouncements
	default:
		return false
	}
}
