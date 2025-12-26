package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/utils"
)

type AdminHandler struct {
	userService    *services.UserService
	sessionService *services.SessionService
	rsvpService    *services.RSVPService
}

func NewAdminHandler(userService *services.UserService, sessionService *services.SessionService, rsvpService *services.RSVPService) *AdminHandler {
	return &AdminHandler{
		userService:    userService,
		sessionService: sessionService,
		rsvpService:    rsvpService,
	}
}

// ListJoinRequests returns all pending join requests
func (h *AdminHandler) ListJoinRequests(c *gin.Context) {
	users, err := h.userService.ListPendingJoinRequests()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list join requests"})
		return
	}

	c.JSON(http.StatusOK, users)
}

// ApproveJoinRequest approves a membership request
func (h *AdminHandler) ApproveJoinRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userService.ApproveJoinRequest(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// RejectJoinRequest rejects a membership request
func (h *AdminHandler) RejectJoinRequest(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	user, err := h.userService.RejectJoinRequest(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

type UpdateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=pending player admin"`
}

// UpdateUserRole updates a user's role
func (h *AdminHandler) UpdateUserRole(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userService.UpdateUserRole(id, models.UserRole(req.Role))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

type CreateSessionRequest struct {
	Title              string `json:"title" binding:"required"`
	Description        string `json:"description"`
	SessionDate        string `json:"session_date" binding:"required"` // YYYY-MM-DD
	StartTime          string `json:"start_time" binding:"required"`   // HH:MM
	EndTime            string `json:"end_time" binding:"required"`     // HH:MM
	Courts             int    `json:"courts" binding:"required,min=1,max=3"`
	IsRecurring        bool   `json:"is_recurring"`
	RecurringDayOfWeek *int   `json:"recurring_day_of_week"`
	Occurrences        *int   `json:"occurrences"` // Number of recurring sessions to create
}

// CreateSession creates a new session
func (h *AdminHandler) CreateSession(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sessionDate, err := utils.ParseDateInSydney(req.SessionDate)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}

	session, err := h.sessionService.CreateSession(services.CreateSessionInput{
		Title:              req.Title,
		Description:        req.Description,
		SessionDate:        sessionDate,
		StartTime:          req.StartTime,
		EndTime:            req.EndTime,
		Courts:             req.Courts,
		IsRecurring:        req.IsRecurring,
		RecurringDayOfWeek: req.RecurringDayOfWeek,
		Occurrences:        req.Occurrences,
		CreatedBy:          user.ID,
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

type UpdateSessionRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	SessionDate *string `json:"session_date"` // YYYY-MM-DD
	StartTime   *string `json:"start_time"`   // HH:MM
	EndTime     *string `json:"end_time"`     // HH:MM
	Courts      *int    `json:"courts"`
	Status      *string `json:"status"`
}

// UpdateSession updates a session
func (h *AdminHandler) UpdateSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	var req UpdateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.UpdateSessionInput{
		Title:       req.Title,
		Description: req.Description,
		StartTime:   req.StartTime,
		EndTime:     req.EndTime,
		Courts:      req.Courts,
	}

	if req.SessionDate != nil {
		sessionDate, err := utils.ParseDateInSydney(*req.SessionDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
			return
		}
		input.SessionDate = &sessionDate
	}

	if req.Status != nil {
		status := models.SessionStatus(*req.Status)
		input.Status = &status
	}

	session, err := h.sessionService.UpdateSession(id, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

// DeleteSession deletes or cancels a session
func (h *AdminHandler) DeleteSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	if err := h.sessionService.DeleteSession(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session deleted"})
}

type CancelSessionRequest struct {
	Reason string `json:"reason"`
}

// CancelSession cancels a session with an optional reason
func (h *AdminHandler) CancelSession(c *gin.Context) {
	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	var req CancelSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Reason is optional, so we don't error if body is empty
		req.Reason = ""
	}

	session, err := h.sessionService.CancelSession(id, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, session)
}

type AdminRSVPRequest struct {
	Status string `json:"status" binding:"required,oneof=in out maybe"`
}

// AddPlayerRSVP allows admin to add/update a player's RSVP
func (h *AdminHandler) AddPlayerRSVP(c *gin.Context) {
	sessionIDStr := c.Param("id")
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid session ID"})
		return
	}

	userIDStr := c.Param("userId")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	var req AdminRSVPRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rsvp, err := h.rsvpService.CreateOrUpdateRSVP(services.RSVPInput{
		SessionID: sessionID,
		UserID:    userID,
		Status:    models.RSVPStatus(req.Status),
	}, true) // byAdmin = true

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rsvp)
}

// GetClub returns club information
func (h *AdminHandler) GetClub(c *gin.Context) {
	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Club not found"})
		return
	}

	c.JSON(http.StatusOK, club)
}

type UpdateClubRequest struct {
	Name         *string `json:"name"`
	VenueName    *string `json:"venue_name"`
	VenueAddress *string `json:"venue_address"`
}

// UpdateClub updates club information
func (h *AdminHandler) UpdateClub(c *gin.Context) {
	var req UpdateClubRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Club not found"})
		return
	}

	if req.Name != nil {
		club.Name = *req.Name
	}
	if req.VenueName != nil {
		club.VenueName = *req.VenueName
	}
	if req.VenueAddress != nil {
		club.VenueAddress = *req.VenueAddress
	}

	if err := database.DB.Save(&club).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update club"})
		return
	}

	c.JSON(http.StatusOK, club)
}
