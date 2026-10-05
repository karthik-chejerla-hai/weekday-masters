package handlers

import (
	"encoding/json"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"gorm.io/gorm"
	"io"
	"net/http"
)

type ExpenseHandler struct{ service *services.ExpenseService }

func NewExpenseHandler(service *services.ExpenseService) *ExpenseHandler {
	return &ExpenseHandler{service: service}
}
func (h *ExpenseHandler) RegisterRoutes(protected *gin.RouterGroup) {
	approved := protected.Group("", middleware.RequireApproved())
	approved.GET("/sessions/unsettled", h.ListUnsettled)
	admin := approved.Group("/admin", middleware.RequireAdmin())
	admin.POST("/sessions/:id/expense/preview", h.Preview)
	admin.POST("/sessions/:id/expense", h.Confirm)
}
func (h *ExpenseHandler) ListUnsettled(c *gin.Context) {
	items, err := h.service.ListUnsettledSessions()
	if err != nil {
		respondExpenseError(c, err)
		return
	}
	c.JSON(200, gin.H{"items": items, "total": len(items)})
}
func (h *ExpenseHandler) Preview(c *gin.Context) { h.handle(c, false) }
func (h *ExpenseHandler) Confirm(c *gin.Context) { h.handle(c, true) }
func (h *ExpenseHandler) handle(c *gin.Context, confirm bool) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(401, gin.H{"code": "unauthorized", "message": "Sign in to continue."})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Invalid session ID."})
		return
	}
	var in services.ExpenseInput
	if err = decodeBoundedJSON(c, &in, 32<<10); err != nil {
		c.JSON(400, gin.H{"code": "bad_request", "message": "Check the expense fields and try again."})
		return
	}
	if confirm {
		record, err := h.service.Confirm(id, in, actor)
		if err != nil {
			respondExpenseError(c, err)
			return
		}
		c.JSON(http.StatusCreated, record)
		return
	}
	preview, err := h.service.Preview(id, in, actor)
	if err != nil {
		respondExpenseError(c, err)
		return
	}
	c.JSON(200, preview)
}
func decodeBoundedJSON(c *gin.Context, target any, limit int64) error {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return errors.New("expected one JSON object")
	}
	return nil
}
func respondExpenseError(c *gin.Context, err error) {
	if _, ok := services.AsLedgerError(err); ok {
		respondLedgerError(c, err)
		return
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(404, gin.H{"code": "not_found", "message": "This session is not available."})
		return
	}
	c.JSON(500, gin.H{"code": "internal", "message": "Could not load the expense. Try again."})
}
