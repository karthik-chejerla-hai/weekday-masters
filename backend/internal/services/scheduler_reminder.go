package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// sendReminder commits the unique history row before contacting the provider.
// Only its creator attempts delivery. This prevents duplicate submissions across
// processes, but a process exit after commit can lose that attempt. Reliable
// retries require the durable outbox planned in issue #81.
func (s *SchedulerService) sendReminder(ctx context.Context, userID uuid.UUID, session models.Session, typ models.NotificationType, kind, title, body string, data map[string]string) (bool, error) {
	allowed, bypassPause, err := s.notificationService.deliveryPolicy(userID)
	if err != nil || !allowed {
		return false, err
	}
	instant := session.RSVPDeadline
	if typ == models.NotificationSessionReminder {
		instant, err = s.parseSessionDateTime(session)
		if err != nil {
			return false, err
		}
	}
	key := fmt.Sprintf("%s/%s/%s/%s/%s", userID, session.ID, typ, kind, instant.UTC().Format(time.RFC3339Nano))
	payload, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	notification := models.Notification{UserID: userID, NotificationType: typ, Title: title, Body: body, Data: string(payload), ReminderKey: &key}
	created := false
	err = database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// RSVP updates and cancellation also lock the session. Check current
		// eligibility rather than trusting the scheduler's earlier audience read.
		var current models.Session
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&current, "id = ?", session.ID).Error; err != nil {
			return err
		}
		if current.Status != models.SessionStatusOpen {
			return nil
		}
		currentInstant := current.RSVPDeadline
		if typ == models.NotificationSessionReminder {
			var err error
			currentInstant, err = s.parseSessionDateTime(current)
			if err != nil {
				return err
			}
		}
		if !currentInstant.Equal(instant) {
			return nil
		}
		var club models.Club
		if err := tx.Select("notifications_paused").First(&club).Error; err != nil {
			return err
		}
		if club.NotificationsPaused && !bypassPause {
			return nil
		}
		var user models.User
		if err := tx.First(&user, "id = ?", userID).Error; err != nil {
			return err
		}
		if user.MembershipStatus != models.MembershipApproved {
			return nil
		}
		var prefs models.UserNotificationPreferences
		prefsErr := tx.Where("user_id = ?", userID).First(&prefs).Error
		if prefsErr != nil && !errors.Is(prefsErr, gorm.ErrRecordNotFound) {
			return prefsErr
		}
		// A missing row uses the normal push-enabled defaults. Explicit opt-outs
		// must not reserve the reminder if the member opts in before it is due.
		if prefsErr == nil && !prefs.IsPushEnabledForType(typ) {
			return nil
		}
		var rsvp models.RSVP
		err := tx.Where("session_id = ? AND user_id = ?", session.ID, userID).First(&rsvp).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if typ == models.NotificationSessionReminder {
			if err != nil || rsvp.Status != models.RSVPStatusIn {
				return nil
			}
		} else if err == nil {
			return nil
		}
		result := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "reminder_key"}}, DoNothing: true}).Create(&notification)
		if result.Error != nil {
			return result.Error
		}
		created = result.RowsAffected == 1
		return nil
	})
	if err != nil || !created {
		return false, err
	}
	return true, s.notificationService.deliverNotification(ctx, &notification)
}
