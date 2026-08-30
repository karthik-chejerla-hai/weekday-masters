package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
)

type AuthHandler struct {
	userService  *services.UserService
	auth0Service *services.Auth0Service
}

func NewAuthHandler(userService *services.UserService, auth0Service *services.Auth0Service) *AuthHandler {
	return &AuthHandler{userService: userService, auth0Service: auth0Service}
}

// AuthCallbackRequest carries display-only fields. Identity (subject and email) is
// taken from the verified access token, never from this body.
type AuthCallbackRequest struct {
	Name           string `json:"name"`
	ProfilePicture string `json:"profile_picture"`
}

// Callback registers a first-time user or syncs an existing one after Auth0 login.
// It runs behind RequireValidToken, so the caller holds a valid token but may not
// have a user row yet.
func (h *AuthHandler) Callback(c *gin.Context) {
	auth0ID, err := middleware.GetAuth0IDFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthenticated"})
		return
	}

	var req AuthCallbackRequest
	// Body is optional; these fields are cosmetic.
	_ = c.ShouldBindJSON(&req)

	user, err := h.userService.FindByAuth0ID(auth0ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to look up user"})
		return
	}

	// Existing user: refresh display fields only.
	if user != nil {
		updated, err := h.userService.SyncDisplayFields(user, req.Name, req.ProfilePicture)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"user": updated, "is_new": false})
		return
	}

	// First login: pull the authoritative profile from Auth0 so the email — which
	// governs admin auto-promotion — cannot be chosen by the caller.
	accessToken, err := middleware.GetAccessTokenFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthenticated"})
		return
	}

	profile, err := h.auth0Service.FetchProfile(accessToken)
	if err != nil {
		log.Printf("auth callback: %v", err)
		if errors.Is(err, services.ErrProfileUnavailable) {
			c.JSON(http.StatusBadGateway, gin.H{"error": "Could not verify your profile with Auth0. Please try again."})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		return
	}

	// The profile must belong to the token holder.
	if profile.Sub != auth0ID {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Profile does not match token subject"})
		return
	}

	// Registration also claims an outstanding admin invite when the verified
	// email matches one, so an invited member signs in already approved.
	registered, isNew, err := h.userService.RegisterUser(profile)
	if err != nil {
		log.Printf("auth callback: failed to register %s: %v", profile.Sub, err)
		// These two say something the person can act on, so they are passed
		// through rather than flattened into "registration failed".
		if errors.Is(err, services.ErrEmailAlreadyLinked) || errors.Is(err, services.ErrInviteEmailNotVerified) {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": registered, "is_new": isNew})
}
