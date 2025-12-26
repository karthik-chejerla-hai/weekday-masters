package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/services"
)

type SessionHandler struct {
	sessionService *services.SessionService
	rsvpService    *services.RSVPService
}

func NewSessionHandler(sessionService *services.SessionService, rsvpService *services.RSVPService) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
		rsvpService:    rsvpService,
	}
}

// ListSessions returns all upcoming sessions
func (h *SessionHandler) ListSessions(c *gin.Context) {
	sessions, err := h.sessionService.ListUpcomingSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	// Add RSVP summary to each session
	type SessionWithSummary struct {
		*services.SessionService
		Summary *services.RSVPSummary `json:"rsvp_summary"`
	}

	c.JSON(http.StatusOK, sessions)
}

// GetSession returns a single session with full details
func (h *SessionHandler) GetSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	session, err := h.sessionService.GetSessionByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	// Get RSVP summary
	summary, _ := h.rsvpService.GetRSVPSummary(id)

	c.JSON(http.StatusOK, gin.H{
		"session":      session,
		"rsvp_summary": summary,
	})
}

// ListCancelledSessions returns upcoming cancelled sessions
func (h *SessionHandler) ListCancelledSessions(c *gin.Context) {
	sessions, err := h.sessionService.ListCancelledUpcomingSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list cancelled sessions"})
		return
	}

	c.JSON(http.StatusOK, sessions)
}
