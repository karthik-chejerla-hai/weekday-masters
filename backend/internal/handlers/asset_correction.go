package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
	"net/http"
)

func (h *LedgerHandler) ListAssetPurchases(c *gin.Context) {
	limit, offset := parsePaging(c)
	items, total, err := h.ledgerService.AssetPurchases(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"message": "Could not load asset purchases."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

func (h *LedgerHandler) CorrectAssetPurchase(c *gin.Context) {
	actor, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"message": "Sign in to continue."})
		return
	}
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": "Invalid transaction ID."})
		return
	}
	var req struct {
		AmountCents int64  `json:"amount_cents" binding:"required"`
		Units       int    `json:"units"`
		Reason      string `json:"reason" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"message": err.Error()})
		return
	}
	result, err := h.ledgerService.CorrectAssetPurchase(services.AssetPurchaseCorrectionInput{TransactionID: id, AmountCents: req.AmountCents, Units: req.Units, Reason: req.Reason, CreatedBy: actor.ID})
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
