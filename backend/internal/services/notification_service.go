package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"google.golang.org/api/option"
	"gorm.io/gorm"
)

type NotificationService struct {
	fcmClient      *messaging.Client
	sendGridClient *sendgrid.Client
	fromEmail      string
	fromName       string
	frontendURL    string
	fcmEnabled     bool
	emailEnabled   bool
}

type NotificationConfig struct {
	FirebaseCredentials string
	SendGridAPIKey      string
	SendGridFromEmail   string
	SendGridFromName    string
	FrontendURL         string
}

// NewNotificationService creates a new notification service
// It gracefully handles missing credentials (FCM or SendGrid can be disabled independently)
func NewNotificationService(cfg NotificationConfig) *NotificationService {
	service := &NotificationService{
		fromEmail:   cfg.SendGridFromEmail,
		fromName:    cfg.SendGridFromName,
		frontendURL: cfg.FrontendURL,
	}

	// Initialize Firebase FCM if credentials provided
	if cfg.FirebaseCredentials != "" {
		opt := option.WithCredentialsJSON([]byte(cfg.FirebaseCredentials))
		app, err := firebase.NewApp(context.Background(), nil, opt)
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

	// Initialize SendGrid if API key provided
	if cfg.SendGridAPIKey != "" {
		service.sendGridClient = sendgrid.NewSendClient(cfg.SendGridAPIKey)
		service.emailEnabled = true
		log.Println("SendGrid initialized successfully")
	} else {
		log.Println("SendGrid API key not configured, email notifications disabled")
	}

	return service
}

// IsEnabled returns true if at least one notification channel is enabled
func (s *NotificationService) IsEnabled() bool {
	return s.fcmEnabled || s.emailEnabled
}

// SendNotification sends a notification to a single user via configured channels
func (s *NotificationService) SendNotification(
	ctx context.Context,
	userID uuid.UUID,
	notifType models.NotificationType,
	title, body string,
	data map[string]string,
) error {
	// Get user
	var user models.User
	if err := database.DB.First(&user, "id = ?", userID).Error; err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}

	// Get or create notification preferences
	var prefs models.UserNotificationPreferences
	result := database.DB.Where("user_id = ?", userID).First(&prefs)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Create default preferences
		prefs = models.UserNotificationPreferences{UserID: userID}
		database.DB.Create(&prefs)
	} else if result.Error != nil {
		return fmt.Errorf("failed to get preferences: %w", result.Error)
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

	// Check if push is enabled for this notification type
	pushEnabled := prefs.IsPushEnabledForType(notifType) && s.fcmEnabled
	emailEnabled := prefs.IsEmailEnabledForType(notifType) && s.emailEnabled

	// Send push notification
	if pushEnabled {
		if err := s.sendPushNotification(ctx, userID, title, body, data); err != nil {
			log.Printf("Failed to send push to user %s: %v", userID, err)
		} else {
			now := time.Now()
			notification.PushSent = true
			notification.PushSentAt = &now
		}
	}

	// Send email notification
	if emailEnabled && user.Email != "" {
		if err := s.sendEmailNotification(user.Email, user.Name, title, body, notifType); err != nil {
			log.Printf("Failed to send email to user %s: %v", userID, err)
		} else {
			now := time.Now()
			notification.EmailSent = true
			notification.EmailSentAt = &now
		}
	}

	// Update notification record
	database.DB.Save(&notification)

	return nil
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
		return nil // No tokens, nothing to send
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
			Notification: &messaging.WebpushNotification{
				Icon: "/icons/icon-192x192.png",
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

	log.Printf("Push notification sent to %d/%d devices for user %s", response.SuccessCount, len(tokens), userID)
	return nil
}

// sendEmailNotification sends an email notification
func (s *NotificationService) sendEmailNotification(toEmail, toName, subject, body string, notifType models.NotificationType) error {
	if !s.emailEnabled {
		return errors.New("email not enabled")
	}

	from := mail.NewEmail(s.fromName, s.fromEmail)
	to := mail.NewEmail(toName, toEmail)

	// Build HTML email
	htmlContent := s.buildEmailHTML(subject, body, notifType)

	message := mail.NewSingleEmail(from, subject, to, body, htmlContent)

	response, err := s.sendGridClient.Send(message)
	if err != nil {
		return err
	}

	if response.StatusCode >= 400 {
		return fmt.Errorf("SendGrid returned status %d: %s", response.StatusCode, response.Body)
	}

	log.Printf("Email sent to %s: %s", toEmail, subject)
	return nil
}

// buildEmailHTML creates a styled HTML email
func (s *NotificationService) buildEmailHTML(subject, body string, notifType models.NotificationType) string {
	// Icon based on notification type
	iconEmoji := "🏸"
	switch notifType {
	case models.NotificationSessionReminder:
		iconEmoji = "⏰"
	case models.NotificationRSVPDeadline:
		iconEmoji = "📅"
	case models.NotificationWaitlistUpdate:
		iconEmoji = "🎉"
	case models.NotificationAdminAnnouncement:
		iconEmoji = "📢"
	}

	return fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif; max-width: 600px; margin: 0 auto; padding: 0; background-color: #f8fafc;">
    <div style="background-color: #0891b2; color: white; padding: 24px; text-align: center;">
        <h1 style="margin: 0; font-size: 24px;">🏸 Weekday Masters</h1>
    </div>
    <div style="padding: 24px; background-color: white;">
        <div style="font-size: 32px; text-align: center; margin-bottom: 16px;">%s</div>
        <h2 style="color: #1e293b; margin-top: 0;">%s</h2>
        <p style="color: #475569; font-size: 16px; line-height: 1.6;">%s</p>
        <div style="text-align: center; margin-top: 24px;">
            <a href="%s/dashboard" style="display: inline-block; background-color: #0891b2; color: white; padding: 12px 24px; text-decoration: none; border-radius: 8px; font-weight: 600;">View Dashboard</a>
        </div>
    </div>
    <div style="background-color: #f1f5f9; padding: 16px; text-align: center; font-size: 12px; color: #64748b;">
        <p style="margin: 0 0 8px 0;">You received this email because you have notifications enabled for Weekday Masters.</p>
        <p style="margin: 0;"><a href="%s/profile" style="color: #0891b2;">Manage your notification preferences</a></p>
    </div>
</body>
</html>
`, iconEmoji, subject, body, s.frontendURL, s.frontendURL)
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
		// Create default preferences
		prefs = models.UserNotificationPreferences{UserID: userID}
		if err := database.DB.Create(&prefs).Error; err != nil {
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
	// Check if token already exists
	var existing models.UserPushToken
	result := database.DB.Where("token = ?", token).First(&existing)

	if result.Error == nil {
		// Token exists, update user and last used
		existing.UserID = userID
		existing.DeviceName = deviceName
		existing.LastUsedAt = time.Now()
		return database.DB.Save(&existing).Error
	}

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		// Create new token
		newToken := models.UserPushToken{
			UserID:     userID,
			Token:      token,
			DeviceName: deviceName,
		}
		return database.DB.Create(&newToken).Error
	}

	return result.Error
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
