package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

func (h *LedgerHandler) CorrectAssetBalance(c *gin.Context) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Sign in to continue."})
		return
	}
	var req struct {
		RequestID     string             `json:"request_id" binding:"required,uuid"`
		AccountKind   models.AccountKind `json:"account_kind" binding:"required"`
		ExpectedCents *int64             `json:"expected_cents" binding:"required"`
		TargetCents   *int64             `json:"target_cents" binding:"required"`
		Reason        string             `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	id, err := uuid.Parse(req.RequestID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid request ID."})
		return
	}
	item, err := h.ledgerService.CorrectAssetBalance(services.BalanceCorrectionInput{
		RequestID: id, AccountKind: req.AccountKind, ExpectedCents: *req.ExpectedCents,
		TargetCents: *req.TargetCents, Reason: req.Reason, CreatedBy: actor.ID,
	})
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *LedgerHandler) ListBalanceCorrections(c *gin.Context) {
	limit, offset := parsePaging(c)
	items, total, err := h.ledgerService.ListBalanceCorrections(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not load correction history."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}
