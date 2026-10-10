package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/url"
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
	NotificationID uuid.UUID `json:"notification_id"`
	BalanceCents   int64     `json:"balance_cents"`
	PushSent       bool      `json:"push_sent"`
	NextAllowedAt  time.Time `json:"next_allowed_at"`
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

	prefs, err := s.GetUserPreferences(userID)
	if err != nil {
		return nil, err
	}
	if !prefs.PushEnabled {
		return nil, ErrPushDisabledForUser
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
	allowed, err := s.deliveryAllowed(userID)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	// Fail closed if the pause cannot be read. The importer sets it in the
	// same commit as membership, so cron cannot see an unpaused imported user.
	var club models.Club
	if err := database.DB.Select("notifications_paused").First(&club).Error; err != nil {
		return err
	}
	if club.NotificationsPaused {
		return nil
	}
	// Create notification record
	dataJSON, _ := json.Marshal(data)
	notification := models.Notification{
		UserID:           userID,
		NotificationType: notifType,
		Title:            title,
		Body:             body,
		Data:             string(dataJSON),
	}

	if err := database.DB.Create(&notification).Error; err != nil {
		return fmt.Errorf("failed to create notification record: %w", err)
	}

	return s.deliverNotification(ctx, &notification)
}

// SendBalanceNudge records a targeted top-up reminder and attempts immediate
// push delivery. The history record is useful even when the member has opted
// out of push or has no registered device.
func (s *NotificationService) SendBalanceNudge(
	ctx context.Context,
	userID, sentBy uuid.UUID,
) (*BalanceNudgeResult, error) {
	if s.disabled {
		return nil, ErrNotificationsDisabled
	}
	if userID == sentBy {
		return nil, ErrBalanceNudgeSelf
	}

	var notification models.Notification
	var balance int64
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var user models.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&user, "id = ?", userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrMemberNotFound
			}
			return err
		}
		if user.MembershipStatus != models.MembershipApproved {
			return ErrBalanceNudgeNotAllowed
		}

		var club models.Club
		if err := tx.First(&club).Error; err != nil {
			return err
		}
		if club.NotificationsPaused {
			return ErrNotificationsPaused
		}

		if err := tx.Raw(`
			SELECT COALESCE(SUM(e.amount_cents), 0)
			FROM accounts a
			LEFT JOIN ledger_entries e ON e.account_id = a.id
			WHERE a.user_id = ?
		`, userID).Scan(&balance).Error; err != nil {
			return err
		}
		if balance >= club.LowBalanceThresholdCents {
			return ErrBalanceNudgeNotNeeded
		}

		var previous models.Notification
		cutoff := time.Now().Add(-BalanceNudgeCooldown)
		err := tx.Where(
			"user_id = ? AND created_at > ? AND data ->> 'source' = ?",
			userID, cutoff, "admin_balance_nudge",
		).Order("created_at DESC").First(&previous).Error
		if err == nil {
			return &BalanceNudgeCooldownError{NextAllowedAt: previous.CreatedAt.Add(BalanceNudgeCooldown)}
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		notifType := models.NotificationBalanceLow
		title := "Your Rally balance is running low"
		if balance < 0 {
			notifType = models.NotificationBalanceNegative
			title = "Please top up your Rally balance"
		}
		data, err := json.Marshal(map[string]string{
			"balance_cents": fmt.Sprintf("%d", balance),
			"nudged_by":     sentBy.String(),
			"source":        "admin_balance_nudge",
		})
		if err != nil {
			return err
		}
		notification = models.Notification{
			UserID:           userID,
			NotificationType: notifType,
			Title:            title,
			Body:             fmt.Sprintf("An admin sent you a reminder to top up. Your balance is %s.", formatCents(balance)),
			Data:             string(data),
		}
		return tx.Create(&notification).Error
	})
	if err != nil {
		return nil, err
	}

	if err := s.deliverNotification(ctx, &notification); err != nil {
		return nil, err
	}
	return &BalanceNudgeResult{
		NotificationID: notification.ID,
		BalanceCents:   balance,
		PushSent:       notification.PushSent,
		NextAllowedAt:  notification.CreatedAt.Add(BalanceNudgeCooldown),
	}, nil
}

// deliverNotification sends an already committed history record. Cancellation
// uses this to keep the session, announcement and audience atomic without
// holding database locks during provider requests.
func (s *NotificationService) deliverNotification(ctx context.Context, notification *models.Notification) error {
	allowed, err := s.deliveryAllowed(notification.UserID)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	var club models.Club
	if err := database.DB.Select("notifications_paused").First(&club).Error; err != nil {
		return err
	}
	if club.NotificationsPaused {
		return nil
	}
	userID := notification.UserID
	// Get user
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	prefs, err := s.GetUserPreferences(userID)
	if err != nil {
		return fmt.Errorf("failed to get preferences: %w", err)
	}

	if user.MembershipStatus != models.MembershipApproved {
		return nil
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(notification.Data), &data); err != nil {
		return err
	}
	// All delivery paths, including records created by cancellation, carry the
	// authoritative type for foreground and background click routing.
	if data == nil {
		data = make(map[string]string)
	}
	data["type"] = string(notification.NotificationType)
	if prefs.IsPushEnabledForType(notification.NotificationType) && s.fcmEnabled && !notification.PushSent {
		if err := s.sendPushNotification(ctx, userID, notification.Title, notification.Body, data); err != nil {
			log.Printf("Failed to send push to user %s: %v", userID, err)
		} else {
			now := time.Now()
			notification.PushSent = true
			notification.PushSentAt = &now
		}
	}
	// Preserve read_at if the member opens the notification while delivery runs.
	return database.DB.Model(notification).Select("push_sent", "push_sent_at").Updates(notification).Error
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
	if err := database.DB.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
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
				database.DB.Delete(&models.UserPushToken{}, "token = ?", tokenStrings[i])
				log.Printf("Removed invalid FCM token for user %s", userID)
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
	path := "/dashboard"
	if id, err := uuid.Parse(data["session_id"]); err == nil {
		path = "/sessions/" + id.String()
	} else if strings.HasPrefix(data["type"], "balance_") {
		path = "/money"
	}
	return base + path
}

// deliveryAllowed leaves the deployment stop in place for everyone except an
// exact, case-insensitive email allowlist entry. Preview supplies the verified
// admin email; production leaves this empty.
func (s *NotificationService) deliveryAllowed(userID uuid.UUID) (bool, error) {
	if !s.disabled {
		return true, nil
	}
	if s.disabledAllowEmail == "" {
		return false, nil
	}
	var user models.User
	if err := database.DB.Select("email").First(&user, "id = ?", userID).Error; err != nil {
		return false, err
	}
	return strings.EqualFold(strings.TrimSpace(user.Email), s.disabledAllowEmail), nil
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
		DoUpdates: clause.AssignmentColumns([]string{"user_id", "device_name", "last_used_at"}),
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
	var notifications []models.Notification
	query := database.DB.Where("user_id = ?", userID).Order("created_at DESC")

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

// MarkNotificationRead marks a notification as read
func (s *NotificationService) MarkNotificationRead(notificationID, userID uuid.UUID) error {
	now := time.Now()
	return database.DB.Model(&models.Notification{}).
		Where("id = ? AND user_id = ?", notificationID, userID).
		Update("read_at", &now).Error
}
