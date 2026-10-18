package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type DeliveryState string

const (
	DeliveryPending    DeliveryState = "pending"
	DeliveryClaimed    DeliveryState = "claimed"
	DeliverySending    DeliveryState = "sending"
	DeliveryAccepted   DeliveryState = "accepted"
	DeliveryTransient  DeliveryState = "transient_failure"
	DeliveryFailed     DeliveryState = "failed"
	DeliveryUncertain  DeliveryState = "uncertain"
	DeliverySuppressed DeliveryState = "suppressed"
	DeliveryExpired    DeliveryState = "expired"
	DeliveryCompleted  DeliveryState = "completed"
	DeliveryPartial    DeliveryState = "partial"
)

// NotificationOutbox separates delivery outcome from member-readable history.
// No provider addresses are exposed through this model.
type NotificationOutbox struct {
	ID              uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	NotificationID  uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex" json:"notification_id"`
	EventKey        string        `gorm:"type:text;not null;uniqueIndex" json:"-"`
	DueAt           time.Time     `gorm:"not null" json:"-"`
	ExpiresAt       time.Time     `gorm:"not null" json:"-"`
	State           DeliveryState `gorm:"type:text;not null;index:idx_delivery_due,priority:1" json:"state"`
	LeaseGeneration int64         `gorm:"not null;default:0" json:"-"`
	LeaseOwner      uuid.UUID     `gorm:"type:uuid" json:"-"`
	LeaseUntil      *time.Time    `json:"-"`
	NextAttemptAt   time.Time     `gorm:"not null;index:idx_delivery_due,priority:2" json:"-"`
	LastErrorCode   string        `gorm:"type:text" json:"-"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	Notification    *Notification `gorm:"foreignKey:NotificationID" json:"-"`
}

func (o *NotificationOutbox) BeforeCreate(_ *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

// One row tracks the entire bounded sequence for one original enrollment. A
// token rotation follows that enrollment while accepted/uncertain rows stay terminal.
type NotificationDeliveryAttempt struct {
	ID                 uuid.UUID           `gorm:"type:uuid;primaryKey" json:"-"`
	OutboxID           uuid.UUID           `gorm:"type:uuid;not null;uniqueIndex:idx_delivery_device_channel,priority:1" json:"-"`
	EnrollmentID       uuid.UUID           `gorm:"type:uuid;not null;uniqueIndex:idx_delivery_device_channel,priority:2" json:"-"`
	Channel            string              `gorm:"type:text;not null;default:push;uniqueIndex:idx_delivery_device_channel,priority:3" json:"-"`
	State              DeliveryState       `gorm:"type:text;not null" json:"state"`
	AttemptCount       int                 `gorm:"not null;default:0" json:"-"`
	NextAttemptAt      time.Time           `gorm:"not null" json:"-"`
	SendingOwner       uuid.UUID           `gorm:"type:uuid" json:"-"`
	SendStartedAt      *time.Time          `json:"-"`
	ProviderAcceptedAt *time.Time          `json:"-"`
	LastErrorCode      string              `gorm:"type:text" json:"-"`
	CreatedAt          time.Time           `json:"-"`
	UpdatedAt          time.Time           `json:"-"`
	Outbox             *NotificationOutbox `gorm:"foreignKey:OutboxID" json:"-"`
}

func (a *NotificationDeliveryAttempt) BeforeCreate(_ *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

// Business backfills record completion in the same transaction as their work.
type NotificationMigrationRecord struct {
	Version     string    `gorm:"type:text;primaryKey"`
	Cutoff      time.Time `gorm:"not null"`
	CompletedAt time.Time `gorm:"not null"`
}
