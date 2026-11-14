package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const RSVPManualReminderCooldown = time.Hour

var ErrRSVPReminderNotAllowed = errors.New("only approved administrators can send RSVP reminders")
var ErrRSVPReminderUnavailable = errors.New("reminders require an upcoming open session before its RSVP deadline")
var ErrRSVPReminderRequest = errors.New("a unique request_id is required and must belong to this session and admin")

type RSVPReminderPreview struct {
	AudienceCount int        `json:"audience_count"`
	NextAllowedAt *time.Time `json:"next_allowed_at"`
}
type RSVPReminderCooldownError struct{ NextAllowedAt time.Time }

func (e *RSVPReminderCooldownError) Error() string {
	return "This session was reminded in the last hour."
}

func manualReminderAudience(tx *gorm.DB, sessionID uuid.UUID) *gorm.DB {
	return tx.Model(&models.User{}).Where("membership_status = ? AND NOT EXISTS (SELECT 1 FROM rsvps WHERE rsvps.user_id = users.id AND rsvps.session_id = ?)", models.MembershipApproved, sessionID)
}
func requireRSVPReminderAdmin(tx *gorm.DB, actorID uuid.UUID) error {
	var actor models.User
	if err := tx.First(&actor, "id = ?", actorID).Error; err != nil {
		return err
	}
	if !actor.IsApproved() || actor.Role != models.RoleAdmin {
		return ErrRSVPReminderNotAllowed
	}
	return nil
}

func manualReminderSession(tx *gorm.DB, sessionID uuid.UUID) (*models.Session, error) {
	var session models.Session
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&session, "id = ?", sessionID).Error; err != nil {
		return nil, err
	}
	return &session, nil
}
func manualReminderEligible(session *models.Session, now time.Time) bool {
	return session.Status == models.SessionStatusOpen && session.StartsAt != nil && session.StartsAt.After(now) && session.RSVPDeadline.After(now)
}
func manualReminderNext(tx *gorm.DB, sessionID uuid.UUID, now time.Time) (*time.Time, error) {
	var previous models.RSVPReminderBatch
	err := tx.Where("session_id = ? AND audience_count > 0 AND created_at > ?", sessionID, now.Add(-RSVPManualReminderCooldown)).Order("created_at DESC").First(&previous).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	next := previous.CreatedAt.Add(RSVPManualReminderCooldown)
	return &next, nil
}
func (s *NotificationService) PreviewManualRSVPReminder(ctx context.Context, actorID, sessionID uuid.UUID) (*RSVPReminderPreview, error) {
	result := &RSVPReminderPreview{}
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := requireRSVPReminderAdmin(tx, actorID); err != nil {
			return err
		}
		session, err := manualReminderSession(tx, sessionID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if !manualReminderEligible(session, now) {
			return ErrRSVPReminderUnavailable
		}
		var count int64
		if err := manualReminderAudience(tx, sessionID).Count(&count).Error; err != nil {
			return err
		}
		result.AudienceCount = int(count)
		result.NextAllowedAt, err = manualReminderNext(tx, sessionID, now)
		return err
	})
	return result, err
}

// Session locking serializes audience snapshots with RSVP writes and all admins.
// Persist the batch and outbox in one transaction; retries return the saved result.
func (s *NotificationService) SendManualRSVPReminder(ctx context.Context, actorID, sessionID, requestID uuid.UUID) (*models.RSVPReminderBatch, error) {
	if requestID == uuid.Nil {
		return nil, ErrRSVPReminderRequest
	}
	var result models.RSVPReminderBatch
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotificationKey(tx, "manual-rsvp-request:"+requestID.String()); err != nil {
			return err
		}
		if err := requireRSVPReminderAdmin(tx, actorID); err != nil {
			return err
		}
		// Completed requests remain recoverable after the session is deleted.
		err := tx.Where("request_id = ?", requestID).First(&result).Error
		if err == nil {
			if result.CreatedBy != actorID || result.SessionID != sessionID {
				return ErrRSVPReminderRequest
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		session, err := manualReminderSession(tx, sessionID)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		if !manualReminderEligible(session, now) {
			return ErrRSVPReminderUnavailable
		}
		next, err := manualReminderNext(tx, sessionID, now)
		if err != nil {
			return err
		}
		if next != nil {
			return &RSVPReminderCooldownError{NextAllowedAt: *next}
		}
		var members []models.User
		if err := manualReminderAudience(tx, sessionID).Select("id").Order("id").Find(&members).Error; err != nil {
			return err
		}
		result = models.RSVPReminderBatch{RequestID: requestID, SessionID: sessionID, CreatedBy: actorID, AudienceCount: len(members), CreatedAt: now}
		if len(members) > 0 {
			next := now.Add(RSVPManualReminderCooldown)
			result.NextAllowedAt = &next
		}
		if err := tx.Create(&result).Error; err != nil {
			return err
		}
		expires := session.RSVPDeadline
		if session.StartsAt.Before(expires) {
			expires = *session.StartsAt
		}
		for _, member := range members {
			n := models.Notification{UserID: member.ID, NotificationType: models.NotificationRSVPReminder, Title: "Please RSVP", Body: fmt.Sprintf("Please RSVP for %s on %s at %s.", session.Title, utils.FormatDateForDisplay(session.SessionDate), utils.FormatTimeForDisplay(*session.StartsAt)), Data: mustNotificationJSON(map[string]string{"type": string(models.NotificationRSVPReminder), "source": "admin_rsvp_reminder", "session_id": sessionID.String(), "request_id": requestID.String(), "nudged_by": actorID.String(), "starts_at": session.StartsAt.UTC().Format(time.RFC3339Nano)})}
			outbox, err := s.QueueNotificationWithin(tx, &n, "manual-rsvp:"+result.ID.String()+":"+member.ID.String(), now, expires)
			if err != nil {
				return err
			}
			if outbox.State == models.DeliveryPending {
				result.QueuedCount++
			} else {
				result.SuppressedCount++
			}
		}
		return tx.Model(&result).Select("queued_count", "suppressed_count").Updates(&result).Error
	})
	return &result, err
}
