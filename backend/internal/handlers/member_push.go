package handlers

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/middleware"
	"gorm.io/gorm"
)

func (h *AdminHandler) RegisterMemberPushRoutes(protected *gin.RouterGroup) {
	admin := protected.Group("/admin", middleware.RequireApproved(), middleware.RequireAdmin())
	admin.GET("/users/:id/push-notifications", h.MemberPushStatus)
}

func (h *AdminHandler) MemberPushStatus(c *gin.Context) {
	id, ok := parseUserIDParam(c)
	if !ok {
		return
	}
	status, err := h.userService.MemberPushStatus(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Member not found."})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not load push settings."})
		return
	}
	c.JSON(http.StatusOK, status)
}
