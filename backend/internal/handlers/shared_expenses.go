package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
)

// SharedExpenseHandler records bills that are not sessions, such as a meal or a
// coffee, split between members.
type SharedExpenseHandler struct {
	service *services.SharedExpenseService
}

func NewSharedExpenseHandler(service *services.SharedExpenseService) *SharedExpenseHandler {
	return &SharedExpenseHandler{service: service}
}

// RegisterRoutes hangs both routes off approved-then-admin. Gin binds the chain
// at registration time, so the wrong parent group silently skips a check.
func (h *SharedExpenseHandler) RegisterRoutes(protected *gin.RouterGroup) {
	admin := protected.Group("/admin", middleware.RequireApproved(), middleware.RequireAdmin())
	admin.POST("/expenses/preview", h.Preview)
	admin.POST("/expenses", h.Confirm)
}

func (h *SharedExpenseHandler) Preview(c *gin.Context) { h.handle(c, false) }
func (h *SharedExpenseHandler) Confirm(c *gin.Context) { h.handle(c, true) }

func (h *SharedExpenseHandler) handle(c *gin.Context, confirm bool) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": "Sign in to continue."})
		return
	}
	var in services.SharedExpenseInput
	if err = decodeBoundedJSON(c, &in, 32<<10); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Check the expense fields and try again."})
		return
	}
	if confirm {
		txn, err := h.service.Confirm(in, actor)
		if err != nil {
			respondExpenseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, txn)
		return
	}
	preview, err := h.service.Preview(in, actor)
	if err != nil {
		respondExpenseError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}
