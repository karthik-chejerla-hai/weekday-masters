package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
)

type InvitationHandler struct{ service *services.InvitationService }

func NewInvitationHandler(service *services.InvitationService) *InvitationHandler {
	return &InvitationHandler{service}
}

// protected already validates identity. Keep these access checks shared with tests.
func (h *InvitationHandler) RegisterRoutes(protected *gin.RouterGroup) {
	admin := protected.Group("/admin")
	admin.Use(middleware.RequireApproved(), middleware.RequireAdmin())
	admin.GET("/users/:id/invitation", h.Preview)
	admin.POST("/users/:id/invitation", h.Send)
	admin.POST("/users/:id/invitation/test", h.SendTest)
}

func invitationResponseError(c *gin.Context, err error) {
	var domain *services.InvitationError
	if errors.As(err, &domain) {
		c.JSON(domain.Status, gin.H{"error": domain.Message})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not complete the invitation request. Reopen the preview to check its status."})
}

func (h *InvitationHandler) Preview(c *gin.Context) {
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in first."})
		return
	}
	preview, err := h.service.Preview(actor.ID, userID)
	if err != nil {
		invitationResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

func (h *InvitationHandler) Send(c *gin.Context)     { h.send(c, false) }
func (h *InvitationHandler) SendTest(c *gin.Context) { h.send(c, true) }
func (h *InvitationHandler) send(c *gin.Context, test bool) {
	userID, ok := parseUserIDParam(c)
	if !ok {
		return
	}
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Sign in first."})
		return
	}
	var req struct {
		RequestID     string `json:"request_id" binding:"required"`
		ExpectedEmail string `json:"expected_email" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "A request ID and the reviewed member email are required."})
		return
	}
	requestID, err := uuid.Parse(req.RequestID)
	if err != nil || requestID == uuid.Nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request ID."})
		return
	}
	delivery, err := h.service.Send(c.Request.Context(), actor.ID, userID, requestID, test, req.ExpectedEmail)
	if err != nil {
		invitationResponseError(c, err)
		return
	}
	c.JSON(http.StatusOK, delivery)
}
