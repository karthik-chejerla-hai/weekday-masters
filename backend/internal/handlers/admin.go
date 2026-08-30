package handlers

import (
	"errors"
	"log"
	"net/http"
	"time"

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

// --- member management ----------------------------------------------------

// ListMembers returns every user row, whatever their membership status.
// GET /api/users is the approved-members list the club sees; this one is the
// admin's, and includes the pending, rejected and removed rows they act on.
func (h *AdminHandler) ListMembers(c *gin.Context) {
	users, err := h.userService.ListAllMembers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list members"})
		return
	}

	c.JSON(http.StatusOK, users)
}

type InviteMemberRequest struct {
	Email       string `json:"email" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Nickname    string `json:"nickname"`
	PhoneNumber string `json:"phone_number"`
	Role        string `json:"role" binding:"omitempty,oneof=player admin"`
}

// InviteMember adds a member who has not signed up yet.
func (h *AdminHandler) InviteMember(c *gin.Context) {
	var req InviteMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, err := h.userService.InviteMember(services.InviteMemberInput{
		Email:       req.Email,
		Name:        req.Name,
		Nickname:    req.Nickname,
		PhoneNumber: req.PhoneNumber,
		Role:        models.UserRole(req.Role),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, user)
}

// UpdateMemberRequest uses pointers so an omitted field is left alone rather
// than blanked — the edit form sends only what changed.
type UpdateMemberRequest struct {
	Name        *string `json:"name"`
	Nickname    *string `json:"nickname"`
	PhoneNumber *string `json:"phone_number"`
	Email       *string `json:"email"`
	Role        *string `json:"role" binding:"omitempty,oneof=pending player admin"`
	IsPlayer    *bool   `json:"is_player"`
}

// UpdateMember edits a member's details on an admin's behalf.
func (h *AdminHandler) UpdateMember(c *gin.Context) {
	id, ok := parseUserIDParam(c)
	if !ok {
		return
	}

	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	input := services.UpdateMemberInput{
		Name:        req.Name,
		Nickname:    req.Nickname,
		PhoneNumber: req.PhoneNumber,
		Email:       req.Email,
		IsPlayer:    req.IsPlayer,
	}
	if req.Role != nil {
		role := models.UserRole(*req.Role)
		input.Role = &role
	}

	user, err := h.userService.UpdateMemberDetails(id, input)
	if err != nil {
		respondMemberError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// RemoveMember revokes a member's access to the club. It is not a delete: the
// row and its ledger history stay, and ReinstateMember can undo it.
func (h *AdminHandler) RemoveMember(c *gin.Context) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	id, ok := parseUserIDParam(c)
	if !ok {
		return
	}

	user, err := h.userService.RemoveMember(id, actor.ID)
	if err != nil {
		respondMemberError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// ReinstateMember returns a removed member to the club.
func (h *AdminHandler) ReinstateMember(c *gin.Context) {
	id, ok := parseUserIDParam(c)
	if !ok {
		return
	}

	user, err := h.userService.ReinstateMember(id)
	if err != nil {
		respondMemberError(c, err)
		return
	}

	c.JSON(http.StatusOK, user)
}

// parseUserIDParam reads :id, answering the request itself when it is not a
// UUID. The bool says whether the caller should carry on.
func parseUserIDParam(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return uuid.Nil, false
	}
	return id, true
}

// respondMemberError separates "no such member" from "you may not do that".
// Everything else the member service rejects is a rule the admin broke, and the
// message is written to be read by them.
func respondMemberError(c *gin.Context, err error) {
	if errors.Is(err, services.ErrMemberNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

type CreateSessionRequest struct {
	Title              string  `json:"title" binding:"required"`
	Description        string  `json:"description"`
	SessionDate        string  `json:"session_date" binding:"required"` // YYYY-MM-DD
	StartTime          string  `json:"start_time" binding:"required"`   // HH:MM
	EndTime            string  `json:"end_time" binding:"required"`     // HH:MM
	Courts             int     `json:"courts" binding:"required,min=1,max=3"`
	IsRecurring        bool    `json:"is_recurring"`
	RecurringDayOfWeek *int    `json:"recurring_day_of_week"`
	Occurrences        *int    `json:"occurrences"`   // Number of recurring sessions to create
	RSVPDeadline       *string `json:"rsvp_deadline"` // Optional ISO datetime (RFC3339)
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

	input := services.CreateSessionInput{
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
	}

	if req.RSVPDeadline != nil {
		deadline, err := time.Parse(time.RFC3339, *req.RSVPDeadline)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid RSVP deadline format. Use RFC3339 (e.g. 2026-04-08T23:59:59+11:00)"})
			return
		}
		input.RSVPDeadline = &deadline
	}

	session, err := h.sessionService.CreateSession(input)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, session)
}

type UpdateSessionRequest struct {
	Title        *string `json:"title"`
	Description  *string `json:"description"`
	SessionDate  *string `json:"session_date"` // YYYY-MM-DD
	StartTime    *string `json:"start_time"`   // HH:MM
	EndTime      *string `json:"end_time"`     // HH:MM
	Courts       *int    `json:"courts"`
	Status       *string `json:"status"`
	RSVPDeadline *string `json:"rsvp_deadline"` // Optional ISO datetime (RFC3339)
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

	if req.RSVPDeadline != nil {
		deadline, err := time.Parse(time.RFC3339, *req.RSVPDeadline)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid RSVP deadline format. Use RFC3339 (e.g. 2026-04-08T23:59:59+11:00)"})
			return
		}
		input.RSVPDeadline = &deadline
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

	// Court count is the only input that moves max_players, so only a courts edit can
	// free spots. Renames and time changes skip the lock-and-promote pass entirely.
	if input.Courts != nil {
		if err := h.rsvpService.PromoteFromWaitlist(id); err != nil {
			log.Printf("failed to promote from waitlist for session %s: %v", id, err)
		}
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
