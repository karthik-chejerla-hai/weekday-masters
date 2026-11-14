package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"strconv"
	"time"
)

// The dispatcher already holds this outbox's fenced lease lock. Recheck timing
// before expiry and before each device send; never acquire the session lock in
// reverse order to the scanner's session -> outbox locking order.
func (s *NotificationService) reconcileRSVPDeliveryWithin(tx *gorm.DB, n *models.Notification, o *models.NotificationOutbox, now time.Time) (bool, error) {
	if n.NotificationType != models.NotificationRSVPReminder {
		return false, nil
	}
	var data map[string]string
	if json.Unmarshal([]byte(n.Data), &data) != nil {
		return false, nil
	}
	sessionID, err := uuid.Parse(data["session_id"])
	if err != nil {
		return false, nil
	}
	stage, err := strconv.Atoi(data["rsvp_stage"])
	if err != nil || stage < 1 || stage > 2 {
		return false, nil
	}
	var session models.Session
	if err := tx.First(&session, "id = ?", sessionID).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	if session.StartsAt == nil || session.Status != models.SessionStatusOpen {
		return false, nil
	}
	due, expires := RSVPReminderTimes(*session.StartsAt, session.RSVPDeadline, stage)
	data["starts_at"] = session.StartsAt.UTC().Format(time.RFC3339Nano)
	encoded, err := json.Marshal(data)
	if err != nil {
		return false, err
	}
	n.Data = string(encoded)
	n.Body = fmt.Sprintf("Please RSVP for %s on %s at %s.", session.Title, utils.FormatDateForDisplay(session.SessionDate), utils.FormatTimeForDisplay(*session.StartsAt))
	var submitted int64
	if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state IN ?", o.ID, []models.DeliveryState{models.DeliverySending, models.DeliveryAccepted, models.DeliveryUncertain}).Count(&submitted).Error; err != nil {
		return false, err
	}
	if submitted == 0 {
		if err := tx.Model(n).Updates(map[string]any{"data": n.Data, "body": n.Body}).Error; err != nil {
			return false, err
		}
	}
	if o.DueAt.Equal(due) && o.ExpiresAt.Equal(expires) && !now.Before(due) {
		return false, nil
	}
	// Keep accepted/uncertain devices terminal, and retain retry backoff. An
	// unsent claim is released with a new generation so its old owner cannot send.
	if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", o.ID, models.DeliveryPending).Update("next_attempt_at", due).Error; err != nil {
		return false, err
	}
	if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", o.ID, models.DeliveryTransient).Update("next_attempt_at", gorm.Expr("GREATEST(next_attempt_at, ?)", due)).Error; err != nil {
		return false, err
	}
	next := due
	var attempts []models.NotificationDeliveryAttempt
	if err := tx.Where("outbox_id = ? AND state IN ?", o.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}).Find(&attempts).Error; err != nil {
		return false, err
	}
	for i, a := range attempts {
		if i == 0 || a.NextAttemptAt.Before(next) {
			next = a.NextAttemptAt
		}
	}
	result := tx.Model(&models.NotificationOutbox{}).Where("id = ? AND state = ? AND lease_owner = ? AND lease_generation = ?", o.ID, models.DeliveryClaimed, o.LeaseOwner, o.LeaseGeneration).Updates(map[string]any{"state": models.DeliveryPending, "due_at": due, "expires_at": expires, "next_attempt_at": next, "lease_owner": uuid.New(), "lease_generation": gorm.Expr("lease_generation + 1"), "lease_until": nil})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected != 1 {
		return false, ErrNotificationLeaseLost
	}
	o.State = models.DeliveryPending
	o.DueAt = due
	o.ExpiresAt = expires
	o.NextAttemptAt = next
	return true, nil
}
