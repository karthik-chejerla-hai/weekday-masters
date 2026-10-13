package services

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const notificationLeaseDuration = 2 * time.Minute

var ErrNotificationLeaseLost = errors.New("notification delivery lease lost")
var deliveryRetryDelays = []time.Duration{time.Minute, 5 * time.Minute, 15 * time.Minute}

// DeliveryProviderResult contains only a classification safe for storage/logging.
// Unknown submission outcomes are terminal uncertain, never automatic retries.
type DeliveryProviderResult struct {
	State               models.DeliveryState
	Code                string
	InvalidRegistration bool
}
type DeliveryProvider interface {
	Send(context.Context, models.Notification, string, time.Duration) DeliveryProviderResult
}
type NotificationDispatcher struct {
	DB       *gorm.DB
	Provider DeliveryProvider
	Now      func() time.Time
	// Policy returns a terminal suppression code or an operational database error.
	Policy func(*gorm.DB, models.Notification, time.Time) (string, error)
}
type DeliveryRunSummary struct {
	Claimed    int `json:"claimed"`
	Completed  int `json:"completed"`
	Partial    int `json:"partial"`
	Suppressed int `json:"suppressed"`
	Expired    int `json:"expired"`
	Failed     int `json:"failed"`
	Uncertain  int `json:"uncertain"`
	Pending    int `json:"pending"`
}

func (d *NotificationDispatcher) now() time.Time {
	if d.Now != nil {
		return d.Now().UTC()
	}
	return time.Now().UTC()
}

// ProcessNotificationWork is a bounded worker entry point; trigger scanning is
// owned by the scheduler and caller. It performs no provider I/O under DB locks.
func (s *NotificationService) ProcessNotificationWork(ctx context.Context, batchSize int) (*DeliveryRunSummary, error) {
	d := NotificationDispatcher{DB: database.DB, Provider: notificationFCMProvider{s}, Policy: s.queueDeliveryPolicy}
	return d.Process(ctx, batchSize)
}
func (d *NotificationDispatcher) Process(ctx context.Context, batchSize int) (*DeliveryRunSummary, error) {
	if batchSize < 1 {
		batchSize = 25
	}
	if batchSize > 100 {
		batchSize = 100
	}
	summary := &DeliveryRunSummary{}
	for i := 0; i < batchSize; i++ {
		// Provider initialization is an operational worker concern. It must
		// never claim/suppress durable intent created by a separate API process.
		if d.Provider == nil {
			return summary, ErrPushUnavailable
		}
		if availability, ok := d.Provider.(interface{ Available() bool }); ok && !availability.Available() {
			return summary, ErrPushUnavailable
		}
		if err := ctx.Err(); err != nil {
			return summary, err
		}
		outbox, err := d.claim(ctx)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			break
		}
		if err != nil {
			return summary, err
		}
		summary.Claimed++
		state, err := d.processClaim(ctx, outbox)
		if err != nil {
			return summary, err
		}
		switch state {
		case models.DeliveryCompleted:
			summary.Completed++
		case models.DeliveryPartial:
			summary.Partial++
		case models.DeliverySuppressed:
			summary.Suppressed++
		case models.DeliveryExpired:
			summary.Expired++
		case models.DeliveryFailed:
			summary.Failed++
		case models.DeliveryUncertain:
			summary.Uncertain++
		default:
			summary.Pending++
		}
	}
	return summary, nil
}
func (d *NotificationDispatcher) claim(ctx context.Context) (*models.NotificationOutbox, error) {
	var outbox models.NotificationOutbox
	now := d.now()
	err := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).Where("(state = ? AND next_attempt_at <= ? AND due_at <= ?) OR (state = ? AND lease_until <= ?)", models.DeliveryPending, now, now, models.DeliveryClaimed, now).Order("next_attempt_at ASC, id ASC").First(&outbox).Error; err != nil {
			return err
		}
		// A previous worker may have submitted a message before crashing. Recover
		// only work that had not reached the persisted sending boundary.
		if outbox.State == models.DeliveryClaimed {
			if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state = ?", outbox.ID, models.DeliverySending).Updates(map[string]any{"state": models.DeliveryUncertain, "last_error_code": "lease_expired_inflight"}).Error; err != nil {
				return err
			}
		}
		outbox.State = models.DeliveryClaimed
		outbox.LeaseOwner = uuid.New()
		outbox.LeaseGeneration++
		until := now.Add(notificationLeaseDuration)
		outbox.LeaseUntil = &until
		return tx.Save(&outbox).Error
	})
	return &outbox, err
}
func (d *NotificationDispatcher) fenced(tx *gorm.DB, outbox *models.NotificationOutbox) *gorm.DB {
	return tx.Model(&models.NotificationOutbox{}).Where("id = ? AND state = ? AND lease_owner = ? AND lease_generation = ? AND lease_until > ?", outbox.ID, models.DeliveryClaimed, outbox.LeaseOwner, outbox.LeaseGeneration, d.now())
}
func (d *NotificationDispatcher) lockLease(tx *gorm.DB, outbox *models.NotificationOutbox) error {
	var current models.NotificationOutbox
	err := d.fenced(tx, outbox).Clauses(clause.Locking{Strength: "UPDATE"}).First(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotificationLeaseLost
	}
	return err
}
func (d *NotificationDispatcher) processClaim(ctx context.Context, outbox *models.NotificationOutbox) (models.DeliveryState, error) {
	var notification models.Notification
	if err := d.DB.WithContext(ctx).First(&notification, "id = ?", outbox.NotificationID).Error; err != nil {
		return "", err
	}
	err := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := d.lockLease(tx, outbox); err != nil {
			return err
		}
		if !d.now().Before(outbox.ExpiresAt) {
			return d.terminateWithin(tx, outbox, models.DeliveryExpired, "event_expired")
		}
		if d.Policy != nil {
			code, err := d.Policy(tx, notification, d.now())
			if err != nil {
				return err
			}
			if code != "" {
				return d.terminateWithin(tx, outbox, models.DeliverySuppressed, code)
			}
		}
		var count int64
		if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ?", outbox.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			var devices []models.UserPushToken
			if err := tx.Where("user_id = ? AND revoked_at IS NULL AND invalidated_at IS NULL AND token <> ''", notification.UserID).Find(&devices).Error; err != nil {
				return err
			}
			if len(devices) == 0 {
				return d.terminateWithin(tx, outbox, models.DeliverySuppressed, "no_push_devices")
			}
			for _, device := range devices {
				attempt := models.NotificationDeliveryAttempt{OutboxID: outbox.ID, EnrollmentID: device.ID, Channel: "push", State: models.DeliveryPending, NextAttemptAt: d.now()}
				if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "outbox_id"}, {Name: "enrollment_id"}, {Name: "channel"}}, DoNothing: true}).Create(&attempt).Error; err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	// terminateWithin updates the in-memory state only when its fenced write wins.
	if outbox.State != models.DeliveryClaimed {
		return outbox.State, nil
	}
	var attempts []models.NotificationDeliveryAttempt
	if err := d.DB.Where("outbox_id = ? AND state IN ? AND next_attempt_at <= ?", outbox.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}, d.now()).Order("id ASC").Find(&attempts).Error; err != nil {
		return "", err
	}
	for _, attempt := range attempts {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		device, started, err := d.startAttempt(ctx, outbox, &attempt, notification)
		if err != nil {
			return "", err
		}
		if !started {
			continue
		}
		// TTL must be recomputed on every retry from the remaining event lifetime.
		ttl := outbox.ExpiresAt.Sub(d.now())
		result := DeliveryProviderResult{State: models.DeliveryExpired, Code: "event_expired"}
		if ttl >= time.Second {
			sendTimeout := 20 * time.Second
			if ttl < sendTimeout {
				sendTimeout = ttl
			}
			sendCtx, cancel := context.WithTimeout(ctx, sendTimeout)
			result = d.Provider.Send(sendCtx, notification, device.Token, ttl)
			cancel()
		}
		if err := d.finishAttempt(ctx, outbox, &attempt, device, result); err != nil {
			return "", err
		}
	}
	return d.aggregate(ctx, outbox)
}

type deliveryDevice struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Token      string
	Generation int64
}

func (d *NotificationDispatcher) startAttempt(ctx context.Context, outbox *models.NotificationOutbox, attempt *models.NotificationDeliveryAttempt, notification models.Notification) (deliveryDevice, bool, error) {
	var device deliveryDevice
	started := false
	err := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := d.lockLease(tx, outbox); err != nil {
			return err
		}
		// Renew between devices while the same fenced owner is still current.
		until := d.now().Add(notificationLeaseDuration)
		renewed := d.fenced(tx, outbox).Update("lease_until", until)
		if renewed.Error != nil {
			return renewed.Error
		}
		if renewed.RowsAffected != 1 {
			return ErrNotificationLeaseLost
		}
		outbox.LeaseUntil = &until
		state, code := models.DeliveryState(""), ""
		if !d.now().Before(outbox.ExpiresAt) {
			state, code = models.DeliveryExpired, "event_expired"
		} else if d.Policy != nil {
			reason, err := d.Policy(tx, notification, d.now())
			if err != nil {
				return err
			}
			if reason != "" {
				state, code = models.DeliverySuppressed, reason
			}
		}
		if state == "" {
			err := tx.Model(&models.UserPushToken{}).Select("id,user_id,token,generation").Where("id = ? AND user_id = ? AND revoked_at IS NULL AND invalidated_at IS NULL AND token <> ''", attempt.EnrollmentID, notification.UserID).First(&device).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				state, code = models.DeliverySuppressed, "device_inactive"
			} else if err != nil {
				return err
			}
		}
		changes := map[string]any{"state": state, "last_error_code": code}
		if state == "" {
			now := d.now()
			changes = map[string]any{"state": models.DeliverySending, "sending_owner": outbox.LeaseOwner, "send_started_at": now, "attempt_count": gorm.Expr("attempt_count + 1")}
			started = true
		}
		result := tx.Model(&models.NotificationDeliveryAttempt{}).Where("id = ? AND state IN ?", attempt.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}).Updates(changes)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			started = false
			return ErrNotificationLeaseLost
		}
		attempt.AttemptCount++
		return nil
	})
	return device, started, err
}
func (d *NotificationDispatcher) finishAttempt(ctx context.Context, outbox *models.NotificationOutbox, attempt *models.NotificationDeliveryAttempt, device deliveryDevice, result DeliveryProviderResult) error {
	return d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := d.lockLease(tx, outbox); err != nil {
			return err
		}
		// Reject unrecognized provider classifications as uncertainty; never infer acceptance.
		switch result.State {
		case models.DeliveryAccepted, models.DeliveryFailed, models.DeliveryUncertain, models.DeliveryTransient, models.DeliveryExpired:
		default:
			result.State = models.DeliveryUncertain
			result.Code = "unknown_provider_outcome"
		}
		now := d.now()
		if result.InvalidRegistration && result.State == models.DeliveryFailed {
			// A definite rejection is safe to retry at a newer healthy address.
			// Keep ownership and generation fencing: a rotated enrollment must
			// not be invalidated by the old address's result, nor an event sent
			// to a replacement owner. Acceptance and uncertainty stay terminal.
			invalidation := tx.Model(&models.UserPushToken{}).
				Where("id = ? AND user_id = ? AND generation = ? AND token = ? AND revoked_at IS NULL AND invalidated_at IS NULL", device.ID, device.UserID, device.Generation, device.Token).
				Updates(map[string]any{"invalidated_at": now, "generation": gorm.Expr("generation + 1")})
			if invalidation.Error != nil {
				return invalidation.Error
			}
			if invalidation.RowsAffected == 0 {
				var rotated int64
				if err := tx.Model(&models.UserPushToken{}).
					Where("id = ? AND user_id = ? AND generation > ? AND token <> ? AND token <> '' AND revoked_at IS NULL AND invalidated_at IS NULL", device.ID, device.UserID, device.Generation, device.Token).
					Count(&rotated).Error; err != nil {
					return err
				}
				if rotated == 1 {
					result.State = models.DeliveryTransient
					result.Code = "registration_rotated"
				}
			}
		}
		changes := map[string]any{"state": result.State, "last_error_code": result.Code}
		if result.State == models.DeliveryAccepted {
			changes["provider_accepted_at"] = now
			if err := tx.Model(&models.Notification{}).Where("id = ?", outbox.NotificationID).Updates(map[string]any{"push_sent": true, "push_sent_at": now}).Error; err != nil {
				return err
			}
		}
		if result.State == models.DeliveryTransient {
			if attempt.AttemptCount > len(deliveryRetryDelays) {
				changes["state"] = models.DeliveryFailed
				changes["last_error_code"] = "retry_exhausted"
			} else {
				next := now.Add(deliveryRetryDelays[attempt.AttemptCount-1])
				if !next.Before(outbox.ExpiresAt) {
					changes["state"] = models.DeliveryExpired
					changes["last_error_code"] = "retry_beyond_expiry"
				} else {
					changes["next_attempt_at"] = next
				}
			}
		}
		updated := tx.Model(&models.NotificationDeliveryAttempt{}).Where("id = ? AND state = ? AND sending_owner = ?", attempt.ID, models.DeliverySending, outbox.LeaseOwner).Updates(changes)
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return ErrNotificationLeaseLost
		}
		return nil
	})
}
func (d *NotificationDispatcher) terminateWithin(tx *gorm.DB, outbox *models.NotificationOutbox, state models.DeliveryState, code string) error {
	if err := tx.Model(&models.NotificationDeliveryAttempt{}).Where("outbox_id = ? AND state IN ?", outbox.ID, []models.DeliveryState{models.DeliveryPending, models.DeliveryTransient}).Updates(map[string]any{"state": state, "last_error_code": code}).Error; err != nil {
		return err
	}
	// Preserve earlier acceptance/uncertainty when retries expire or a stop
	// becomes active. A global stop cannot make accepted work appear unsent.
	var attempts []models.NotificationDeliveryAttempt
	if err := tx.Where("outbox_id = ?", outbox.ID).Find(&attempts).Error; err != nil {
		return err
	}
	accepted, uncertain, other := 0, false, false
	for _, a := range attempts {
		if a.State == models.DeliveryAccepted {
			accepted++
		} else {
			other = true
		}
		if a.State == models.DeliveryUncertain {
			uncertain = true
		}
	}
	if accepted > 0 {
		if other {
			state = models.DeliveryPartial
		} else {
			state = models.DeliveryCompleted
		}
	} else if uncertain {
		state = models.DeliveryUncertain
	}
	result := d.fenced(tx, outbox).Updates(map[string]any{"state": state, "last_error_code": code, "lease_until": nil})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrNotificationLeaseLost
	}
	outbox.State = state
	return nil
}
func (d *NotificationDispatcher) aggregate(ctx context.Context, outbox *models.NotificationOutbox) (models.DeliveryState, error) {
	state := models.DeliveryCompleted
	err := d.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := d.lockLease(tx, outbox); err != nil {
			return err
		}
		var attempts []models.NotificationDeliveryAttempt
		if err := tx.Where("outbox_id = ?", outbox.ID).Find(&attempts).Error; err != nil {
			return err
		}
		accepted, uncertain, failed, suppressed, expired, pending := 0, 0, 0, 0, 0, 0
		var next time.Time
		for _, a := range attempts {
			switch a.State {
			case models.DeliveryAccepted:
				accepted++
			case models.DeliveryUncertain:
				uncertain++
			case models.DeliveryFailed:
				failed++
			case models.DeliverySuppressed:
				suppressed++
			case models.DeliveryExpired:
				expired++
			case models.DeliveryPending, models.DeliveryTransient:
				pending++
				if next.IsZero() || a.NextAttemptAt.Before(next) {
					next = a.NextAttemptAt
				}
			default:
				uncertain++
			}
		}
		switch {
		case pending > 0:
			state = models.DeliveryPending
		case accepted > 0 && (uncertain+failed+suppressed+expired) > 0:
			state = models.DeliveryPartial
		case accepted > 0:
			state = models.DeliveryCompleted
		case uncertain > 0:
			state = models.DeliveryUncertain
		case failed > 0:
			state = models.DeliveryFailed
		case expired > 0:
			state = models.DeliveryExpired
		default:
			state = models.DeliverySuppressed
		}
		changes := map[string]any{"state": state, "lease_until": nil}
		if pending > 0 {
			changes["next_attempt_at"] = next
		}
		result := d.fenced(tx, outbox).Updates(changes)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrNotificationLeaseLost
		}
		return nil
	})
	return state, err
}
func (s *NotificationService) queueDeliveryPolicy(tx *gorm.DB, notification models.Notification, now time.Time) (string, error) {
	var user models.User
	if err := tx.First(&user, "id = ?", notification.UserID).Error; err != nil {
		return "", err
	}
	if !user.IsApproved() {
		return "membership_not_approved", nil
	}
	bypass := s.disabled && s.disabledAllowEmail != "" && strings.EqualFold(strings.TrimSpace(user.Email), s.disabledAllowEmail)
	if s.disabled && !bypass {
		return "deployment_paused", nil
	}
	var club models.Club
	if err := tx.First(&club).Error; err != nil {
		return "", err
	}
	if club.NotificationsPaused && !bypass {
		return "club_paused", nil
	}
	// Release 3 preserves existing trigger timing, but stale/cancelled sessions
	// and changed RSVP eligibility are rechecked at dispatch.
	var data map[string]string
	if err := json.Unmarshal([]byte(notification.Data), &data); err != nil {
		return "invalid_event_data", nil
	}
	if notification.NotificationType == models.NotificationSessionReminder || notification.NotificationType == models.NotificationRSVPDeadline {
		id, err := uuid.Parse(data["session_id"])
		if err != nil {
			return "invalid_session", nil
		}
		var session models.Session
		err = tx.First(&session, "id = ?", id).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "session_unavailable", nil
		}
		if err != nil {
			return "", err
		}
		if session.Status != models.SessionStatusOpen || !session.StartsAt.After(now) {
			return "session_unavailable", nil
		}
		var count int64
		query := tx.Model(&models.RSVP{}).Where("session_id = ? AND user_id = ?", id, user.ID)
		if notification.NotificationType == models.NotificationSessionReminder {
			query = query.Where("status = ?", models.RSVPStatusIn)
		}
		if err := query.Count(&count).Error; err != nil {
			return "", err
		}
		if notification.NotificationType == models.NotificationSessionReminder && count == 0 {
			return "no_longer_attending", nil
		}
		if notification.NotificationType == models.NotificationRSVPDeadline && (count > 0 || !now.Before(session.RSVPDeadline)) {
			return "no_longer_eligible", nil
		}
	}
	return "", nil
}

type notificationFCMProvider struct{ s *NotificationService }

func (p notificationFCMProvider) Available() bool {
	return p.s != nil && p.s.fcmEnabled && p.s.fcmClient != nil
}

func (p notificationFCMProvider) Send(ctx context.Context, n models.Notification, token string, ttl time.Duration) DeliveryProviderResult {
	seconds := int64(math.Floor(ttl.Seconds()))
	if seconds < 1 {
		return DeliveryProviderResult{State: models.DeliveryExpired, Code: "event_expired"}
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
		return DeliveryProviderResult{State: models.DeliveryFailed, Code: "invalid_event_data"}
	}
	if data == nil {
		data = map[string]string{}
	}
	data["notification_id"] = n.ID.String()
	data["type"] = string(n.NotificationType)
	webpush := &messaging.WebpushConfig{Headers: map[string]string{"TTL": strconv.FormatInt(seconds, 10)}, Notification: &messaging.WebpushNotification{Icon: "/icons/icon-192x192.svg", Tag: n.ID.String()}}
	if link := p.s.notificationURL(data); link != "" {
		webpush.FCMOptions = &messaging.WebpushFCMOptions{Link: link}
	}
	response, err := p.s.fcmClient.SendEachForMulticast(ctx, &messaging.MulticastMessage{Tokens: []string{token}, Notification: &messaging.Notification{Title: n.Title, Body: n.Body}, Data: data, Webpush: webpush})
	if err == nil && response != nil && len(response.Responses) == 1 && response.Responses[0] != nil {
		if response.Responses[0].Success {
			return DeliveryProviderResult{State: models.DeliveryAccepted}
		}
		err = response.Responses[0].Error
	}
	if err == nil {
		return DeliveryProviderResult{State: models.DeliveryUncertain, Code: "malformed_provider_response"}
	}
	if messaging.IsUnregistered(err) {
		return DeliveryProviderResult{State: models.DeliveryFailed, Code: "unregistered", InvalidRegistration: true}
	}
	if messaging.IsQuotaExceeded(err) || messaging.IsUnavailable(err) || messaging.IsInternal(err) {
		return DeliveryProviderResult{State: models.DeliveryTransient, Code: "provider_transient"}
	}
	if messaging.IsInvalidArgument(err) || messaging.IsSenderIDMismatch(err) {
		return DeliveryProviderResult{State: models.DeliveryFailed, Code: "provider_rejected"}
	}
	return DeliveryProviderResult{State: models.DeliveryUncertain, Code: "submission_uncertain"}
}
