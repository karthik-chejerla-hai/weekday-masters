package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

const NotificationRSVPReminder NotificationType = "rsvp_reminder"

type RSVPReminderStageState string

const (
	RSVPReminderPending   RSVPReminderStageState = "pending"
	RSVPReminderPublished RSVPReminderStageState = "published"
	RSVPReminderSkipped   RSVPReminderStageState = "skipped"
)

// One logical stage per session freezes its eligible publication audience.
// Published means history was created; it never implies provider acceptance.
// Terminal skipped rollout/outage stages are not reset on future deployments.
type RSVPReminderStage struct {
	ID          uuid.UUID              `gorm:"type:uuid;primaryKey"`
	SessionID   uuid.UUID              `gorm:"type:uuid;not null;uniqueIndex:idx_rsvp_reminder_stage,priority:1"`
	Stage       int                    `gorm:"not null;check:stage IN (1,2);uniqueIndex:idx_rsvp_reminder_stage,priority:2"`
	State       RSVPReminderStageState `gorm:"type:text;not null;index"`
	StartsAt    time.Time              `gorm:"not null"`
	DueAt       time.Time              `gorm:"not null;index"`
	ExpiresAt   time.Time              `gorm:"not null"`
	PublishedAt *time.Time
	SkipReason  string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Session     *Session `gorm:"foreignKey:SessionID"`
}

func (s *RSVPReminderStage) BeforeCreate(_ *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}
