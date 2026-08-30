package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
)

type UserHandler struct {
	userService *services.UserService
}

func NewUserHandler(userService *services.UserService) *UserHandler {
	return &UserHandler{userService: userService}
}

// GetMe returns the current user's profile
func (h *UserHandler) GetMe(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, user)
}

// UpdateProfileRequest carries the fields a member may change about themselves.
// Pointers so an omitted field is left alone rather than blanked.
type UpdateProfileRequest struct {
	PhoneNumber *string `json:"phone_number"`
	Nickname    *string `json:"nickname"`
}

// UpdateMe updates the current user's profile
func (h *UserHandler) UpdateMe(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedUser, err := h.userService.UpdateProfile(user.ID, services.UpdateProfileInput{
		PhoneNumber: req.PhoneNumber,
		Nickname:    req.Nickname,
	})
	if err != nil {
		// A rejected nickname is the caller's to fix, so it is not a 500.
		if errors.Is(err, services.ErrMemberNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedUser)
}

// ListMembers returns all approved club members
func (h *UserHandler) ListMembers(c *gin.Context) {
	users, err := h.userService.ListApprovedMembers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list members"})
		return
	}

	c.JSON(http.StatusOK, users)
}
