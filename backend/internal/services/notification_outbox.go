package services

import (
	"errors"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"time"
)

// QueueNotificationWithin must run in the originating action's transaction.
// The stable action key serializes duplicates before creating member history.
func QueueNotificationWithin(tx *gorm.DB, notification *models.Notification, eventKey string, due, expires time.Time) (*models.NotificationOutbox, error) {
	if notification.ID == uuid.Nil {
		notification.ID = uuid.New()
	}
	if eventKey == "" {
		eventKey = "notification:" + notification.ID.String()
	}
	if !expires.After(due) {
		return nil, errors.New("notification expiry must follow due time")
	}
	if err := lockNotificationKey(tx, eventKey); err != nil {
		return nil, err
	}
	var existing models.NotificationOutbox
	err := tx.Where("event_key = ?", eventKey).First(&existing).Error
	if err == nil {
		var stored models.Notification
		if err := tx.First(&stored, "id = ?", existing.NotificationID).Error; err != nil {
			return nil, err
		}
		*notification = stored
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	if err := tx.Create(notification).Error; err != nil {
		return nil, err
	}
	return EnqueueNotificationWithin(tx, notification, eventKey, due, expires)
}

// EnqueueNotificationWithin queues an already persisted recipient event without
// duplicating it. It never scans/backfills historical notifications implicitly.
func EnqueueNotificationWithin(tx *gorm.DB, notification *models.Notification, eventKey string, due, expires time.Time) (*models.NotificationOutbox, error) {
	if eventKey == "" {
		eventKey = "notification:" + notification.ID.String()
	}
	if notification.ID == uuid.Nil || !expires.After(due) {
		return nil, errors.New("invalid notification delivery intent")
	}
	if err := lockNotificationKey(tx, eventKey); err != nil {
		return nil, err
	}
	var existing models.NotificationOutbox
	err := tx.Where("notification_id = ? OR event_key = ?", notification.ID, eventKey).First(&existing).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	outbox := models.NotificationOutbox{NotificationID: notification.ID, EventKey: eventKey, DueAt: due, NextAttemptAt: due, ExpiresAt: expires, State: models.DeliveryPending}
	if err := tx.Create(&outbox).Error; err != nil {
		return nil, err
	}
	return &outbox, nil
}
func lockNotificationKey(tx *gorm.DB, key string) error {
	return tx.Exec("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", "notification-event:"+key).Error
}

// QueueNotificationWithin records event intent and captures present delivery
// stops. Clearing a pause or enrolling later never replays suppressed events.
func (s *NotificationService) QueueNotificationWithin(tx *gorm.DB, notification *models.Notification, eventKey string, due, expires time.Time) (*models.NotificationOutbox, error) {
	outbox, err := QueueNotificationWithin(tx, notification, eventKey, due, expires)
	if err != nil {
		return nil, err
	}
	return s.suppressUnavailableWithin(tx, notification, outbox)
}
func (s *NotificationService) EnqueueNotificationWithin(tx *gorm.DB, notification *models.Notification, eventKey string, due, expires time.Time) (*models.NotificationOutbox, error) {
	outbox, err := EnqueueNotificationWithin(tx, notification, eventKey, due, expires)
	if err != nil {
		return nil, err
	}
	return s.suppressUnavailableWithin(tx, notification, outbox)
}
func (s *NotificationService) suppressUnavailableWithin(tx *gorm.DB, n *models.Notification, o *models.NotificationOutbox) (*models.NotificationOutbox, error) {
	if o.State != models.DeliveryPending {
		return o, nil
	}
	code, err := s.queueDeliveryPolicy(tx, *n, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	if code == "" {
		var count int64
		if err := tx.Model(&models.UserPushToken{}).Where("user_id = ? AND revoked_at IS NULL AND invalidated_at IS NULL AND token <> ''", n.UserID).Count(&count).Error; err != nil {
			return nil, err
		}
		if count == 0 {
			code = "no_push_devices"
		}
	}
	if code != "" {
		o.State = models.DeliverySuppressed
		o.LastErrorCode = code
		result := tx.Model(&models.NotificationOutbox{}).Where("id = ? AND state = ?", o.ID, models.DeliveryPending).Updates(map[string]any{"state": o.State, "last_error_code": code})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected == 0 {
			if err := tx.First(o, "id = ?", o.ID).Error; err != nil {
				return nil, err
			}
		}
	}
	return o, nil
}
