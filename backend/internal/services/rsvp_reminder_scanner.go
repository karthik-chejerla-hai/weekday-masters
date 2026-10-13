package services

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strconv"
	"time"
)

type RSVPReminderScanSummary struct {
	Sessions  int `json:"sessions"`
	Events    int `json:"events"`
	Published int `json:"published"`
	Skipped   int `json:"skipped"`
}

// RSVPReminderTimes uses resolved instants and the actual preserved deadline.
// Returning expires <= due means that this stage cannot be useful.
func RSVPReminderTimes(startsAt, deadline time.Time, stage int) (due, expires time.Time) {
	if stage == 1 {
		due = startsAt.Add(-96 * time.Hour)
		expires = startsAt.Add(-60 * time.Hour)
		if deadline.Before(expires) {
			expires = deadline
		}
		return
	}
	if stage == 2 {
		return startsAt.Add(-60 * time.Hour), deadline
	}
	return time.Time{}, time.Time{}
}
func RSVPReminderEventKey(sessionID, userID uuid.UUID, stage int) string {
	return fmt.Sprintf("rsvp:%s:%s:%d", sessionID, userID, stage)
}

// QueueDueRSVPReminders freezes each stage's no-response audience once, inside
// the session lock shared by RSVP writes. Provider delivery is left to the queue.
func (s *NotificationService) QueueDueRSVPReminders(ctx context.Context, now time.Time, batchSize int) (*RSVPReminderScanSummary, error) {
	if batchSize < 1 {
		batchSize = 25
	}
	if batchSize > 100 {
		batchSize = 100
	}
	now = now.UTC()
	summary := &RSVPReminderScanSummary{}
	for i := 0; i < batchSize; i++ {
		found := false
		step := &RSVPReminderScanSummary{}
		err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var session models.Session
			// Fully processed sessions must not consume every bounded scan, starving
			// later sessions. Rescheduled publications are revisited only for mismatches.
			query := `status = ? AND starts_at > ? AND rsvp_deadline > ? AND (
    (starts_at <= ? AND NOT EXISTS (SELECT 1 FROM rsvp_reminder_stages r WHERE r.session_id = sessions.id))
    OR EXISTS (SELECT 1 FROM rsvp_reminder_stages r WHERE r.session_id = sessions.id AND
     ((r.state = 'pending' AND r.due_at <= ?) OR r.starts_at <> sessions.starts_at OR
      (r.stage = 2 AND r.expires_at <> sessions.rsvp_deadline) OR
      (r.stage = 1 AND r.expires_at <> LEAST(sessions.starts_at - INTERVAL '60 hours',sessions.rsvp_deadline))))
    OR EXISTS (SELECT 1 FROM notification_outboxes o JOIN notifications n ON n.id = o.notification_id
     JOIN rsvp_reminder_stages r ON r.session_id = sessions.id AND r.stage::text = n.data->>'rsvp_stage'
     WHERE n.notification_type = 'rsvp_reminder' AND n.data->>'session_id' = sessions.id::text
      AND o.state = 'pending' AND (o.due_at <> r.due_at OR o.expires_at <> r.expires_at))
   )`
			result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where(query, models.SessionStatusOpen, now, now, now.Add(96*time.Hour), now).Order("starts_at ASC,id ASC").First(&session)
			if result.Error == gorm.ErrRecordNotFound {
				return nil
			}
			if result.Error != nil {
				return result.Error
			}
			found = true
			step.Sessions++
			return s.queueSessionRSVPStagesWithin(tx, session, now, step)
		})
		if err != nil {
			return summary, err
		}
		if !found {
			break
		}
		summary.Sessions += step.Sessions
		summary.Events += step.Events
		summary.Published += step.Published
		summary.Skipped += step.Skipped
	}
	return summary, nil
}
func (s *NotificationService) queueSessionRSVPStagesWithin(tx *gorm.DB, session models.Session, now time.Time, summary *RSVPReminderScanSummary) error {
	if session.StartsAt == nil {
		return nil
	}
	for _, index := range []int{1, 2} {
		due, expires := RSVPReminderTimes(*session.StartsAt, session.RSVPDeadline, index)
		stage := models.RSVPReminderStage{SessionID: session.ID, Stage: index, State: models.RSVPReminderPending, StartsAt: *session.StartsAt, DueAt: due, ExpiresAt: expires}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "session_id"}, {Name: "stage"}}, DoNothing: true}).Create(&stage).Error; err != nil {
			return err
		}
		// Use a fresh destination because a losing insert's generated ID differs.
		var stored models.RSVPReminderStage
		if err := tx.Where("session_id = ? AND stage = ?", session.ID, index).First(&stored).Error; err != nil {
			return err
		}
		if stored.State == models.RSVPReminderPublished {
			if err := rescheduleRSVPOutboxesWithin(tx, session, index, due, expires); err != nil {
				return err
			}
		}
		stored.StartsAt, stored.DueAt, stored.ExpiresAt = *session.StartsAt, due, expires
		if stored.State == models.RSVPReminderPending {
			switch {
			case !expires.After(due) || !now.Before(expires):
				stored.State = models.RSVPReminderSkipped
				stored.SkipReason = "stage_expired"
				summary.Skipped++
			case !now.Before(due):
				var users []models.User
				if err := tx.Where("membership_status = ? AND NOT EXISTS (SELECT 1 FROM rsvps WHERE rsvps.user_id = users.id AND rsvps.session_id = ?)", models.MembershipApproved, session.ID).Order("id ASC").Find(&users).Error; err != nil {
					return err
				}
				for _, user := range users {
					data, _ := json.Marshal(map[string]string{"type": string(models.NotificationRSVPReminder), "source": "rsvp_reminder", "session_id": session.ID.String(), "rsvp_stage": strconv.Itoa(index), "starts_at": session.StartsAt.UTC().Format(time.RFC3339Nano)})
					notification := models.Notification{UserID: user.ID, NotificationType: models.NotificationRSVPReminder, Title: "Please RSVP", Body: fmt.Sprintf("Please RSVP for %s on %s at %s.", session.Title, utils.FormatDateForDisplay(session.SessionDate), utils.FormatTimeForDisplay(*session.StartsAt)), Data: string(data)}
					if _, err := s.QueueNotificationWithin(tx, &notification, RSVPReminderEventKey(session.ID, user.ID, index), due, expires); err != nil {
						return err
					}
					summary.Events++
				}
				stored.State = models.RSVPReminderPublished
				published := now
				stored.PublishedAt = &published
				summary.Published++
			}
		}
		if err := tx.Save(&stored).Error; err != nil {
			return err
		}
	}
	return nil
}

// Rescheduling only touches unsent work. It fences an unsent old claim, follows
// pending token retries, and leaves accepted/sending/uncertain work untouched.
func rescheduleRSVPOutboxesWithin(tx *gorm.DB, session models.Session, index int, due, expires time.Time) error {
	var outboxes []models.NotificationOutbox
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("event_key LIKE ? AND state IN ?", fmt.Sprintf("rsvp:%s:%%:%d", session.ID, index), []models.DeliveryState{models.DeliveryPending, models.DeliveryClaimed}).Find(&outboxes).Error; err != nil {
		return err
	}
	for _, outbox := range outboxes {
		if outbox.DueAt.Equal(due) && outbox.ExpiresAt.Equal(expires) {
			continue
		}
		var inflight int64
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", outbox.ID, models.DeliverySending).Count(&inflight).Error; err != nil {
			return err
		}
		if inflight > 0 {
			continue
		}
		// A known transient rejection can retry only after its backoff and new due.
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", outbox.ID, models.DeliveryPending).Update("next_attempt_at", due).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", outbox.ID, models.DeliveryTransient).Update("next_attempt_at", gorm.Expr("GREATEST(next_attempt_at, ?)", due)).Error; err != nil {
			return err
		}
		next := due
		var attempts []models.NotificationDeliveryAttempt
		if err := tx.Where("outbox_id = ? AND state IN ?", outbox.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}).Find(&attempts).Error; err != nil {
			return err
		}
		if len(attempts) > 0 {
			next = attempts[0].NextAttemptAt
			for _, a := range attempts {
				if a.NextAttemptAt.Before(next) {
					next = a.NextAttemptAt
				}
			}
		}
		if err := tx.Model(&models.NotificationOutbox{}).Where("id = ?", outbox.ID).Updates(map[string]any{"state": models.DeliveryPending, "due_at": due, "expires_at": expires, "next_attempt_at": next, "lease_until": nil, "lease_owner": uuid.New(), "lease_generation": gorm.Expr("lease_generation + 1")}).Error; err != nil {
			return err
		}
		var accepted int64
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state IN ?", outbox.ID, []models.DeliveryState{models.DeliveryAccepted, models.DeliveryUncertain}).Count(&accepted).Error; err != nil {
			return err
		}
		if accepted == 0 {
			var n models.Notification
			if err := tx.First(&n, "id = ?", outbox.NotificationID).Error; err != nil {
				return err
			}
			var data map[string]string
			if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
				return err
			}
			data["starts_at"] = session.StartsAt.UTC().Format(time.RFC3339Nano)
			encoded, _ := json.Marshal(data)
			if err := tx.Model(&n).Updates(map[string]any{"data": string(encoded), "body": fmt.Sprintf("Please RSVP for %s on %s at %s.", session.Title, utils.FormatDateForDisplay(session.SessionDate), utils.FormatTimeForDisplay(*session.StartsAt))}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}
