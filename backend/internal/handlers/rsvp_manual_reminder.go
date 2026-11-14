package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"gorm.io/gorm"
)

func (h *NotificationHandler) PreviewRSVPReminder(c *gin.Context) { h.manualRSVPReminder(c, false) }
func (h *NotificationHandler) SendRSVPReminder(c *gin.Context)    { h.manualRSVPReminder(c, true) }
func (h *NotificationHandler) manualRSVPReminder(c *gin.Context, send bool) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Authentication required"})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid session ID"})
		return
	}
	var result any
	if send {
		var req struct {
			RequestID uuid.UUID `json:"request_id" binding:"required"`
		}
		if c.ShouldBindJSON(&req) != nil || req.RequestID == uuid.Nil {
			c.JSON(http.StatusBadRequest, gin.H{"message": "A valid request_id is required"})
			return
		}
		result, err = h.notificationService.SendManualRSVPReminder(c.Request.Context(), actor.ID, id, req.RequestID)
	} else {
		result, err = h.notificationService.PreviewManualRSVPReminder(c.Request.Context(), actor.ID, id)
	}
	if err == nil {
		c.JSON(http.StatusOK, result)
		return
	}
	var cooldown *services.RSVPReminderCooldownError
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.JSON(http.StatusNotFound, gin.H{"message": "Session not found"})
	case errors.Is(err, services.ErrRSVPReminderNotAllowed):
		c.JSON(http.StatusForbidden, gin.H{"message": err.Error()})
	case errors.Is(err, services.ErrRSVPReminderUnavailable), errors.Is(err, services.ErrRSVPReminderRequest):
		c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
	case errors.As(err, &cooldown):
		c.JSON(http.StatusTooManyRequests, gin.H{"message": err.Error(), "next_allowed_at": cooldown.NextAllowedAt})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not confirm RSVP reminders. Retry to check the same request."})
	}
}
