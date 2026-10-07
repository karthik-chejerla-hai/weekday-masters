package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	disabled    bool
	fcmClient   pushClient
	frontendURL string
	fcmEnabled  bool
}

type NotificationConfig struct {
	Disabled            bool
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
	if cfg.Disabled {
		return &NotificationService{disabled: true}
	}
	service := &NotificationService{
		frontendURL: cfg.FrontendURL,
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
	if s.disabled {
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

// deliverNotification sends an already committed history record. Cancellation
// uses this to keep the session, announcement and audience atomic without
// holding database locks during provider requests.
func (s *NotificationService) deliverNotification(ctx context.Context, notification *models.Notification) error {
	if s.disabled {
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
	if !s.fcmEnabled {
		return errors.New("FCM not enabled")
	}

	// Get all push tokens for user
	var tokens []models.UserPushToken
	if err := database.DB.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return err
	}

	if len(tokens) == 0 {
		return errors.New("no registered push devices")
	}

	// Build token strings
	tokenStrings := make([]string, len(tokens))
	for i, t := range tokens {
		tokenStrings[i] = t.Token
	}

	// Build multicast message
	message := &messaging.MulticastMessage{
		Tokens: tokenStrings,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
		Webpush: &messaging.WebpushConfig{
			FCMOptions: &messaging.WebpushFCMOptions{Link: s.notificationURL(data)},
			Notification: &messaging.WebpushNotification{
				Icon: "/icons/icon-192x192.svg",
			},
		},
	}

	// Send
	response, err := s.fcmClient.SendEachForMulticast(ctx, message)
	if err != nil {
		return err
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
		return errors.New("FCM rejected all device deliveries")
	}
	log.Printf("Push notification sent to %d/%d devices for user %s", response.SuccessCount, len(tokens), userID)
	return nil
}

func (s *NotificationService) notificationURL(data map[string]string) string {
	path := "/dashboard"
	if id, err := uuid.Parse(data["session_id"]); err == nil {
		path = "/sessions/" + id.String()
	} else if strings.HasPrefix(data["type"], "balance_") {
		path = "/money"
	}
	return strings.TrimRight(s.frontendURL, "/") + path
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
