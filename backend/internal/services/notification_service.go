package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type pushClient interface {
	SendEachForMulticast(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error)
}

type NotificationService struct {
	disabled           bool
	disabledAllowEmail string
	fcmClient          pushClient
	frontendURL        string
	fcmEnabled         bool
}

const BalanceNudgeCooldown = 24 * time.Hour

var (
	ErrBalanceNudgeSelf       = errors.New("you cannot nudge yourself")
	ErrBalanceNudgeNotNeeded  = errors.New("this member's balance is not below the low-balance threshold")
	ErrBalanceNudgeNotAllowed = errors.New("only approved members can receive a balance nudge")
	ErrNotificationsPaused    = errors.New("notifications are currently paused")
	ErrNotificationsDisabled  = errors.New("notifications are disabled")
	ErrPushUnavailable        = errors.New("push delivery is unavailable")
	ErrPushDisabledForUser    = errors.New("account push alerts are disabled")
	ErrNoPushDevices          = errors.New("no registered push devices")
)

// BalanceNudgeCooldownError reports when the member may be nudged again.
// Locking the member row in SendBalanceNudge makes this check safe against two
// admins clicking at the same time.
type BalanceNudgeCooldownError struct {
	NextAllowedAt time.Time
}

func (e *BalanceNudgeCooldownError) Error() string {
	return fmt.Sprintf("this member was already nudged; try again after %s", e.NextAllowedAt.Format(time.RFC3339))
}

type BalanceNudgeResult struct {
	RequestID         uuid.UUID            `json:"request_id"`
	NotificationID    uuid.UUID            `json:"notification_id"`
	BalanceCents      int64                `json:"balance_cents"`
	PushSent          bool                 `json:"push_sent"`
	DeliveryState     models.DeliveryState `json:"delivery_state"`
	SuppressionReason string               `json:"suppression_reason,omitempty"`
	NextAllowedAt     time.Time            `json:"next_allowed_at"`
}

// PushTestResult reports provider acceptance for an explicit self-test. It
// deliberately does not create notification history: this is a delivery
// diagnostic, not a club announcement.
type PushTestResult struct {
	AcceptedDevices  int `json:"accepted_devices"`
	AttemptedDevices int `json:"attempted_devices"`
}

type NotificationConfig struct {
	Disabled            bool
	DisabledAllowEmail  string
	FirebaseCredentials string
	FirebaseProjectID   string
	SendGridAPIKey      string
	SendGridFromEmail   string
	SendGridFromName    string
	FrontendURL         string
}

// NewNotificationService creates a new notification service
// Cloud Run uses Application Default Credentials; local callers may supply JSON.
func NewNotificationService(cfg NotificationConfig) *NotificationService {
	service := &NotificationService{
		disabled:           cfg.Disabled,
		disabledAllowEmail: strings.TrimSpace(cfg.DisabledAllowEmail),
		frontendURL:        cfg.FrontendURL,
	}

	// An explicit project enables ADC without requiring a private key in Cloud Run.
	if cfg.FirebaseCredentials != "" || cfg.FirebaseProjectID != "" {
		var opts []option.ClientOption
		if cfg.FirebaseCredentials != "" {
			opts = append(opts, option.WithCredentialsJSON([]byte(cfg.FirebaseCredentials)))
		}
		app, err := firebase.NewApp(context.Background(), &firebase.Config{ProjectID: cfg.FirebaseProjectID}, opts...)
		if err != nil {
			log.Printf("Warning: Failed to initialize Firebase: %v", err)
		} else {
			fcmClient, err := app.Messaging(context.Background())
			if err != nil {
				log.Printf("Warning: Failed to initialize FCM: %v", err)
			} else {
				service.fcmClient = fcmClient
				service.fcmEnabled = true
				log.Println("Firebase Cloud Messaging initialized successfully")
			}
		}
	} else {
		log.Println("Firebase credentials not configured, push notifications disabled")
	}

	return service
}

// SendTestPush sends an explicit diagnostic push only to the caller's own
// registered devices. It intentionally bypasses the deployment-wide and club
// notification stops so an admin can verify preview setup without enabling
// announcements, cron, or delivery to any other member.
func (s *NotificationService) SendTestPush(ctx context.Context, userID uuid.UUID) (*PushTestResult, error) {
	if !s.fcmEnabled || s.fcmClient == nil {
		return nil, ErrPushUnavailable
	}

	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return nil, err
	}
	if user.MembershipStatus != models.MembershipApproved {
		return nil, ErrBalanceNudgeNotAllowed
	}

	return s.sendPushNotificationWithResult(
		ctx,
		userID,
		"Rally push test",
		"Push notifications are working on your registered device.",
		map[string]string{"type": "push_test"},
	)
}

// IsEnabled returns true when push delivery is enabled
func (s *NotificationService) IsEnabled() bool {
	return !s.disabled && s.fcmEnabled
}

// SendNotification records a notification and attempts push delivery
func (s *NotificationService) SendNotification(
	ctx context.Context,
	userID uuid.UUID,
	notifType models.NotificationType,
	title, body string,
	data map[string]string,
) error {
	if database.DB == nil {
		return errors.New("notification database unavailable")
	}
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return err
	}
	notification := models.Notification{UserID: userID, NotificationType: notifType, Title: title, Body: body, Data: string(dataJSON)}
	return database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		key, expiry, err := notificationDeliveryIdentity(tx, &notification, now)
		if err != nil {
			return err
		}
		_, err = s.QueueNotificationWithin(tx, &notification, key, now, expiry)
		return err
	})
}

// SendBalanceNudge captures one manual reminder and its immutable balance
// snapshot. Stable actor/request identities survive retries; the manual-only
// cooldown remains serialized independently of automatic session receipts.
func (s *NotificationService) SendBalanceNudge(ctx context.Context, userID, sentBy uuid.UUID, requests ...uuid.UUID) (*BalanceNudgeResult, error) {
	requestID := uuid.New()
	if len(requests) > 0 {
		requestID = requests[0]
	}
	if requestID == uuid.Nil {
		return nil, ErrNudgeRequestRequired
	}
	key := fmt.Sprintf("nudge:%s:%s", sentBy, requestID)
	var notification models.Notification
	var delivery *models.NotificationOutbox
	var balance int64
	err := database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := lockNotificationKey(tx, key); err != nil {
			return err
		}
		var existing models.NotificationOutbox
		err := tx.Where("event_key = ?", key).First(&existing).Error
		if err == nil {
			if err := tx.First(&notification, "id = ?", existing.NotificationID).Error; err != nil {
				return err
			}
			if notification.UserID != userID {
				return ErrNudgeRequestConflict
			}
			var data map[string]string
			if err := json.Unmarshal([]byte(notification.Data), &data); err != nil {
				return err
			}
			var parseErr error
			balance, parseErr = strconv.ParseInt(data["balance_cents"], 10, 64)
			if parseErr != nil {
				return parseErr
			}
			delivery = &existing
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if userID == sentBy {
			return ErrBalanceNudgeSelf
		}
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "NO KEY UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemberNotFound
			}
			return err
		}
		if !user.IsApproved() {
			return ErrBalanceNudgeNotAllowed
		}
		// Serialize the snapshot with top-ups/settlements using ledger account order.
		var accounts []models.Account
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ?", userID).Order("id").Find(&accounts).Error; err != nil {
			return err
		}
		balance, err = balanceWithin(tx, userID)
		if err != nil {
			return err
		}
		if BalanceNotificationBand(balance) == "healthy" {
			return ErrBalanceNudgeNotNeeded
		}
		now := time.Now().UTC()
		var previous models.Notification
		err = tx.Where("user_id = ? AND created_at > ? AND data ->> 'source' = ?", userID, now.Add(-BalanceNudgeCooldown), "admin_balance_nudge").Order("created_at DESC,id DESC").First(&previous).Error
		if err == nil {
			return &BalanceNudgeCooldownError{NextAllowedAt: previous.CreatedAt.Add(BalanceNudgeCooldown)}
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		kind := models.NotificationBalanceLow
		if balance < 0 {
			kind = models.NotificationBalanceNegative
		}
		notification = models.Notification{UserID: userID, NotificationType: kind, Title: "Balance alert", Body: ManualBalanceMessage(balance, now), Data: mustNotificationJSON(map[string]string{"type": string(kind), "balance_cents": strconv.FormatInt(balance, 10), "balance_band": BalanceNotificationBand(balance), "checked_at": now.Format(time.RFC3339Nano), "nudged_by": sentBy.String(), "request_id": requestID.String(), "source": "admin_balance_nudge"})}
		delivery, err = s.QueueNotificationWithin(tx, &notification, key, now, now.Add(24*time.Hour))
		return err
	})
	if err != nil {
		return nil, err
	}
	return &BalanceNudgeResult{RequestID: requestID, NotificationID: notification.ID, BalanceCents: balance, PushSent: notification.PushSent, DeliveryState: delivery.State, SuppressionReason: delivery.LastErrorCode, NextAllowedAt: notification.CreatedAt.Add(BalanceNudgeCooldown)}, nil
}

// deliverNotification sends an already committed history record. Cancellation
// uses this to keep the session, announcement and audience atomic without
// holding database locks during provider requests.
func (s *NotificationService) deliverNotification(ctx context.Context, notification *models.Notification) error {
	return database.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		due := notification.CreatedAt
		if due.IsZero() {
			due = time.Now().UTC()
		}
		key, expiry, err := notificationDeliveryIdentity(tx, notification, due)
		if err != nil {
			return err
		}
		_, err = s.EnqueueNotificationWithin(tx, notification, key, due, expiry)
		return err
	})
}

// Legacy trigger keys preserve current reminder timing without duplicate events
// from overlapping API revisions. Release 4 replaces these origins atomically.
func notificationDeliveryIdentity(tx *gorm.DB, n *models.Notification, due time.Time) (string, time.Time, error) {
	expiry := due.Add(24 * time.Hour)
	if n.NotificationType != models.NotificationSessionReminder && n.NotificationType != models.NotificationRSVPDeadline {
		return "", expiry, nil
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(n.Data), &data); err != nil {
		return "", expiry, err
	}
	id, err := uuid.Parse(data["session_id"])
	if err != nil {
		return "", expiry, err
	}
	var session models.Session
	if err := tx.First(&session, "id = ?", id).Error; err != nil {
		return "", expiry, err
	}
	if session.StartsAt == nil {
		return "", expiry, errors.New("session has no resolved start")
	}
	if n.NotificationType == models.NotificationRSVPDeadline {
		expiry = session.RSVPDeadline
	} else {
		expiry = *session.StartsAt
	}
	if !expiry.After(due) {
		expiry = due.Add(time.Nanosecond)
	}
	return fmt.Sprintf("legacy:%s:%s:%s:%s", n.NotificationType, id, n.UserID, data["stage"]), expiry, nil
}

// sendPushNotification sends a push notification to all user devices
func (s *NotificationService) sendPushNotification(
	ctx context.Context,
	userID uuid.UUID,
	title, body string,
	data map[string]string,
) error {
	_, err := s.sendPushNotificationWithResult(ctx, userID, title, body, data)
	return err
}

func (s *NotificationService) sendPushNotificationWithResult(
	ctx context.Context,
	userID uuid.UUID,
	title, body string,
	data map[string]string,
) (*PushTestResult, error) {
	if !s.fcmEnabled {
		return nil, ErrPushUnavailable
	}

	// Get all push tokens for user
	var tokens []models.UserPushToken
	if err := database.DB.Where("user_id = ? AND revoked_at IS NULL AND invalidated_at IS NULL", userID).Find(&tokens).Error; err != nil {
		return nil, err
	}

	if len(tokens) == 0 {
		return nil, ErrNoPushDevices
	}

	// Build token strings
	tokenStrings := make([]string, len(tokens))
	for i, t := range tokens {
		tokenStrings[i] = t.Token
	}

	// Build multicast message
	webpush := &messaging.WebpushConfig{
		Notification: &messaging.WebpushNotification{
			Icon: "/icons/icon-192x192.svg",
		},
	}
	if link := s.notificationURL(data); link != "" {
		webpush.FCMOptions = &messaging.WebpushFCMOptions{Link: link}
	}
	message := &messaging.MulticastMessage{
		Tokens: tokenStrings,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data:    data,
		Webpush: webpush,
	}

	// Send
	response, err := s.fcmClient.SendEachForMulticast(ctx, message)
	if err != nil {
		return nil, err
	}

	// Remove invalid tokens
	for i, result := range response.Responses {
		if !result.Success {
			if messaging.IsRegistrationTokenNotRegistered(result.Error) {
				database.DB.Model(&models.UserPushToken{}).Where("id = ? AND token = ? AND generation = ?", tokens[i].ID, tokens[i].Token, tokens[i].Generation).Update("invalidated_at", time.Now())
				log.Printf("Invalidated FCM registration for user %s", userID)
			}
		}
	}

	if response.SuccessCount == 0 {
		return nil, errors.New("FCM rejected all device deliveries")
	}
	log.Printf("Push notification sent to %d/%d devices for user %s", response.SuccessCount, len(tokens), userID)
	return &PushTestResult{AcceptedDevices: response.SuccessCount, AttemptedDevices: len(tokens)}, nil
}

func (s *NotificationService) notificationURL(data map[string]string) string {
	base := strings.TrimRight(s.frontendURL, "/")
	parsed, err := url.Parse(base)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	destination, err := url.Parse(NotificationDestination(data))
	if err != nil {
		return ""
	}
	parsed.Path = destination.Path
	parsed.RawPath = ""
	parsed.RawQuery = destination.RawQuery
	parsed.Fragment = ""
	return parsed.String()
}

// deliveryPolicy leaves the deployment stop in place for everyone except an
// exact, case-insensitive email allowlist entry. That preview-only exception
// also bypasses a club pause inherited from production data. Production leaves
// the allowlist empty.
func (s *NotificationService) deliveryPolicy(userID uuid.UUID) (allowed, bypassClubPause bool, err error) {
	if !s.disabled {
		return true, false, nil
	}
	if s.disabledAllowEmail == "" {
		return false, false, nil
	}
	var user models.User
	if err := database.DB.Select("email").First(&user, "id = ?", userID).Error; err != nil {
		return false, false, err
	}
	allowed = strings.EqualFold(strings.TrimSpace(user.Email), s.disabledAllowEmail)
	return allowed, allowed, nil
}

// SendBulkNotification sends notifications to multiple users
func (s *NotificationService) SendBulkNotification(
	ctx context.Context,
	userIDs []uuid.UUID,
	notifType models.NotificationType,
	title, body string,
	data map[string]string,
) {
	for _, userID := range userIDs {
		// Send in goroutine for parallelism
		go func(uid uuid.UUID) {
			if err := s.SendNotification(ctx, uid, notifType, title, body, data); err != nil {
				log.Printf("Failed to send notification to user %s: %v", uid, err)
			}
		}(userID)
	}
}

// GetUserPreferences retrieves notification preferences for a user
func (s *NotificationService) GetUserPreferences(userID uuid.UUID) (*models.UserNotificationPreferences, error) {
	var prefs models.UserNotificationPreferences
	result := database.DB.Where("user_id = ?", userID).First(&prefs)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Concurrent first notifications can both observe no preferences. Keep
		// whichever row was saved first, including any member opt-outs.
		defaults := models.UserNotificationPreferences{UserID: userID}
		if err := database.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}}, DoNothing: true,
		}).Create(&defaults).Error; err != nil {
			return nil, err
		}
		// Reload by user ID, not the generated ID of an insert that may have
		// lost the race. Delivery must use the persisted preferences.
		if err := database.DB.Where("user_id = ?", userID).First(&prefs).Error; err != nil {
			return nil, err
		}
	} else if result.Error != nil {
		return nil, result.Error
	}
	return &prefs, nil
}

// UpdateUserPreferences updates notification preferences for a user
func (s *NotificationService) UpdateUserPreferences(userID uuid.UUID, updates map[string]interface{}) (*models.UserNotificationPreferences, error) {
	prefs, err := s.GetUserPreferences(userID)
	if err != nil {
		return nil, err
	}

	if err := database.DB.Model(prefs).Updates(updates).Error; err != nil {
		return nil, err
	}

	// Reload to get updated values
	database.DB.First(prefs, "id = ?", prefs.ID)
	return prefs, nil
}

// RegisterPushToken registers a new FCM push token for a user
func (s *NotificationService) RegisterPushToken(userID uuid.UUID, token, deviceName string) error {
	tokenRow := models.UserPushToken{UserID: userID, Token: token, DeviceName: deviceName, LastUsedAt: time.Now()}
	// Settings and app startup can register the same device concurrently.
	return database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "token"}},
		DoNothing: true,
	}).Create(&tokenRow).Error
}

// UnregisterPushToken removes a push token
func (s *NotificationService) UnregisterPushToken(userID uuid.UUID, token string) error {
	if token != "" {
		return database.DB.Where("user_id = ? AND token = ?", userID, token).Delete(&models.UserPushToken{}).Error
	}
	// Remove all tokens for user
	return database.DB.Where("user_id = ?", userID).Delete(&models.UserPushToken{}).Error
}

// GetUserNotifications retrieves notification history for a user
func (s *NotificationService) GetUserNotifications(userID uuid.UUID, limit, offset int) ([]models.Notification, error) {
	notifications := make([]models.Notification, 0)
	query := database.DB.Where("user_id = ?", userID).Order("created_at DESC, id DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	if err := query.Find(&notifications).Error; err != nil {
		return nil, err
	}
	return notifications, nil
}

// GetUserNotification intentionally returns not-found for foreign IDs as well.
func (s *NotificationService) GetUserNotification(notificationID, userID uuid.UUID) (*models.Notification, error) {
	var notification models.Notification
	if err := database.DB.Where("id = ? AND user_id = ?", notificationID, userID).First(&notification).Error; err != nil {
		return nil, err
	}
	return &notification, nil
}

// MarkNotificationRead marks a notification as read
func (s *NotificationService) MarkNotificationRead(notificationID, userID uuid.UUID) error {
	now := time.Now()
	result := database.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("read_at", &now)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
