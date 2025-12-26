package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

type RSVPHandler struct {
	rsvpService *services.RSVPService
}

func NewRSVPHandler(rsvpService *services.RSVPService) *RSVPHandler {
	return &RSVPHandler{rsvpService: rsvpService}
}

type RSVPRequest struct {
	Status string `json:"status" binding:"required,oneof=in out maybe"`
}

// CreateRSVP creates or updates an RSVP for the current user
func (h *RSVPHandler) CreateRSVP(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	var req RSVPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rsvp, err := h.rsvpService.CreateOrUpdateRSVP(services.RSVPInput{
		SessionID: sessionID,
		UserID:    user.ID,
		Status:    models.RSVPStatus(req.Status),
	}, false)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rsvp)
}

// UpdateRSVP updates an existing RSVP
func (h *RSVPHandler) UpdateRSVP(c *gin.Context) {
	// Same as CreateRSVP - the service handles both create and update
	h.CreateRSVP(c)
}

// DeleteRSVP removes the current user's RSVP
func (h *RSVPHandler) DeleteRSVP(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	if err := h.rsvpService.DeleteRSVP(sessionID, user.ID, false); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "RSVP removed"})
}

// GetMyRSVP returns the current user's RSVP for a session
func (h *RSVPHandler) GetMyRSVP(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	rsvp, err := h.rsvpService.GetUserRSVPForSession(sessionID, user.ID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No RSVP found"})
		return
	}

	c.JSON(http.StatusOK, rsvp)
}
