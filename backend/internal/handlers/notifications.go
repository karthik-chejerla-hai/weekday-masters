package handlers

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

type NotificationHandler struct {
	notificationService *services.NotificationService
}

func NewNotificationHandler(notificationService *services.NotificationService) *NotificationHandler {
	return &NotificationHandler{notificationService: notificationService}
}

// GetPreferences returns the current user's notification preferences
func (h *NotificationHandler) GetPreferences(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	prefs, err := h.notificationService.GetUserPreferences(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notification preferences"})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// UpdatePreferencesRequest represents the request to update notification preferences
type UpdatePreferencesRequest struct {
	PushEnabled             *bool `json:"push_enabled,omitempty"`
	PushSessionReminders    *bool `json:"push_session_reminders,omitempty"`
	PushRSVPDeadlines       *bool `json:"push_rsvp_deadlines,omitempty"`
	PushWaitlistUpdates     *bool `json:"push_waitlist_updates,omitempty"`
	PushAdminAnnouncements  *bool `json:"push_admin_announcements,omitempty"`
	EmailEnabled            *bool `json:"email_enabled,omitempty"`
	EmailSessionReminders   *bool `json:"email_session_reminders,omitempty"`
	EmailRSVPDeadlines      *bool `json:"email_rsvp_deadlines,omitempty"`
	EmailWaitlistUpdates    *bool `json:"email_waitlist_updates,omitempty"`
	EmailAdminAnnouncements *bool `json:"email_admin_announcements,omitempty"`
}

// UpdatePreferences updates the current user's notification preferences
func (h *NotificationHandler) UpdatePreferences(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req UpdatePreferencesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.PushEnabled != nil {
		updates["push_enabled"] = *req.PushEnabled
	}
	if req.PushSessionReminders != nil {
		updates["push_session_reminders"] = *req.PushSessionReminders
	}
	if req.PushRSVPDeadlines != nil {
		updates["push_rsvp_deadlines"] = *req.PushRSVPDeadlines
	}
	if req.PushWaitlistUpdates != nil {
		updates["push_waitlist_updates"] = *req.PushWaitlistUpdates
	}
	if req.PushAdminAnnouncements != nil {
		updates["push_admin_announcements"] = *req.PushAdminAnnouncements
	}
	if req.EmailEnabled != nil {
		updates["email_enabled"] = *req.EmailEnabled
	}
	if req.EmailSessionReminders != nil {
		updates["email_session_reminders"] = *req.EmailSessionReminders
	}
	if req.EmailRSVPDeadlines != nil {
		updates["email_rsvp_deadlines"] = *req.EmailRSVPDeadlines
	}
	if req.EmailWaitlistUpdates != nil {
		updates["email_waitlist_updates"] = *req.EmailWaitlistUpdates
	}
	if req.EmailAdminAnnouncements != nil {
		updates["email_admin_announcements"] = *req.EmailAdminAnnouncements
	}

	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No preferences to update"})
		return
	}

	prefs, err := h.notificationService.UpdateUserPreferences(user.ID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update notification preferences"})
		return
	}

	c.JSON(http.StatusOK, prefs)
}

// RegisterTokenRequest represents the request to register a push token
type RegisterTokenRequest struct {
	Token      string `json:"token" binding:"required"`
	DeviceName string `json:"device_name"`
}

// RegisterPushToken registers a new FCM push token for the user
func (h *NotificationHandler) RegisterPushToken(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req RegisterTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.notificationService.RegisterPushToken(user.ID, req.Token, req.DeviceName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register push token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Push token registered successfully"})
}

// UnregisterTokenRequest represents the request to unregister a push token
type UnregisterTokenRequest struct {
	Token string `json:"token"`
}

// UnregisterPushToken removes a push token for the user
func (h *NotificationHandler) UnregisterPushToken(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req UnregisterTokenRequest
	c.ShouldBindJSON(&req) // Token is optional - if not provided, removes all tokens

	if err := h.notificationService.UnregisterPushToken(user.ID, req.Token); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unregister push token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Push token unregistered successfully"})
}

// GetNotificationHistory returns the user's notification history
func (h *NotificationHandler) GetNotificationHistory(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	// Parse query parameters
	limit := 20
	offset := 0
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := c.Query("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	notifications, err := h.notificationService.GetUserNotifications(user.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get notifications"})
		return
	}

	c.JSON(http.StatusOK, notifications)
}

// MarkNotificationRead marks a notification as read
func (h *NotificationHandler) MarkNotificationRead(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	notificationID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid notification ID"})
		return
	}

	if err := h.notificationService.MarkNotificationRead(notificationID, user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to mark notification as read"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Notification marked as read"})
}

// SendAnnouncementRequest represents the request to send an admin announcement
type SendAnnouncementRequest struct {
	Title string `json:"title" binding:"required"`
	Body  string `json:"body" binding:"required"`
}

// SendAnnouncement sends an announcement to all approved members (admin only)
func (h *NotificationHandler) SendAnnouncement(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req SendAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create announcement record
	announcement := models.Announcement{
		Title:     req.Title,
		Body:      req.Body,
		CreatedBy: user.ID,
	}
	if err := database.DB.Create(&announcement).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create announcement"})
		return
	}

	// Get all approved members
	var members []models.User
	if err := database.DB.Where("membership_status = ?", models.MembershipApproved).Find(&members).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get members"})
		return
	}

	// Send notifications to all members
	userIDs := make([]uuid.UUID, len(members))
	for i, m := range members {
		userIDs[i] = m.ID
	}

	ctx := context.Background()
	h.notificationService.SendBulkNotification(
		ctx,
		userIDs,
		models.NotificationAdminAnnouncement,
		req.Title,
		req.Body,
		map[string]string{"type": "admin_announcement", "announcement_id": announcement.ID.String()},
	)

	c.JSON(http.StatusCreated, announcement)
}
