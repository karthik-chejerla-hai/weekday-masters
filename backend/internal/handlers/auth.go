package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/services"
)

type AuthHandler struct {
	userService *services.UserService
}

func NewAuthHandler(userService *services.UserService) *AuthHandler {
	return &AuthHandler{userService: userService}
}

type AuthCallbackRequest struct {
	Auth0ID        string `json:"auth0_id" binding:"required"`
	Email          string `json:"email" binding:"required,email"`
	Name           string `json:"name" binding:"required"`
	ProfilePicture string `json:"profile_picture"`
}

// Callback handles user registration/login after Auth0 authentication
func (h *AuthHandler) Callback(c *gin.Context) {
	var req AuthCallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, isNew, err := h.userService.CreateOrUpdateUser(services.CreateUserInput{
		Auth0ID:        req.Auth0ID,
		Email:          req.Email,
		Name:           req.Name,
		ProfilePicture: req.ProfilePicture,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create/update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":   user,
		"is_new": isNew,
	})
}
