package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/utils"
)

type LedgerHandler struct {
	ledgerService *services.LedgerService
}

func NewLedgerHandler(ledgerService *services.LedgerService) *LedgerHandler {
	return &LedgerHandler{ledgerService: ledgerService}
}

// respondLedgerError translates a domain failure into the structured body the
// frontend keys off. A bare 500 would leave the settlement form unable to tell
// "you are short on shuttles" from "the server fell over".
func respondLedgerError(c *gin.Context, err error) {
	if ledgerErr, ok := services.AsLedgerError(err); ok {
		body := gin.H{"code": ledgerErr.Code, "message": ledgerErr.Message}
		if len(ledgerErr.Details) > 0 {
			body["details"] = ledgerErr.Details
		}
		c.JSON(ledgerErr.Status, body)
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
}

func parsePaging(c *gin.Context) (limit, offset int) {
	limit, _ = strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ = strconv.Atoi(c.DefaultQuery("offset", "0"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

// --- reads (approved members) ---------------------------------------------

// ListBalances returns every member's balance. Visible to all approved members:
// the club already worked this way in Splitwise and is comfortable with it.
func (h *LedgerHandler) ListBalances(c *gin.Context) {
	balances, err := h.ledgerService.AllPlayerBalances()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": balances})
}

// GetMyBalance backs the header chip, so it is deliberately tiny — it is
// requested on nearly every screen.
func (h *LedgerHandler) GetMyBalance(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	balance, err := h.ledgerService.BalanceOfUser(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal", "message": err.Error()})
		return
	}

	threshold, err := lowBalanceThreshold()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal", "message": err.Error()})
		return
	}

	// The threshold rule lives here rather than in the frontend so there is one
	// definition of "running low" rather than two that can drift.
	state := "ok"
	switch {
	case balance < 0:
		state = "negative"
	case balance < threshold:
		state = "low"
	}

	c.JSON(http.StatusOK, gin.H{"balance_cents": balance, "state": state})
}

// GetMyEntries returns the caller's own itemised history.
func (h *LedgerHandler) GetMyEntries(c *gin.Context) {
	user, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	limit, offset := parsePaging(c)
	entries, total, err := h.ledgerService.MyEntries(user.ID, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": entries, "total": total})
}

// --- writes (admin only) --------------------------------------------------

type cashRequest struct {
	UserID      string `json:"user_id" binding:"required"`
	AmountCents int64  `json:"amount_cents" binding:"required"`
	OccurredAt  string `json:"occurred_at"`
	Description string `json:"description"`
}

func (h *LedgerHandler) RecordTopup(c *gin.Context)      { h.recordCash(c, true) }
func (h *LedgerHandler) RecordWithdrawal(c *gin.Context) { h.recordCash(c, false) }

func (h *LedgerHandler) recordCash(c *gin.Context, isTopup bool) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	var req cashRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
		return
	}

	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid user ID"})
		return
	}

	occurredAt, err := parseOccurredAt(req.OccurredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
		return
	}

	input := services.CashInput{
		UserID:      userID,
		AmountCents: req.AmountCents,
		OccurredAt:  occurredAt,
		Description: req.Description,
		CreatedBy:   admin.ID,
	}

	var txn *models.Transaction
	if isTopup {
		txn, err = h.ledgerService.RecordTopup(input)
	} else {
		txn, err = h.ledgerService.RecordWithdrawal(input)
	}
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, txn)
}

type assetPurchaseRequest struct {
	AmountCents int64  `json:"amount_cents" binding:"required"`
	Units       int    `json:"units"`
	OccurredAt  string `json:"occurred_at"`
	Description string `json:"description"`
}

func (h *LedgerHandler) RecordCourtCredit(c *gin.Context) {
	h.recordAsset(c, func(in services.AssetPurchaseInput) (*models.Transaction, error) {
		return h.ledgerService.RecordCourtCreditPurchase(in)
	})
}

func (h *LedgerHandler) RecordShuttlePurchase(c *gin.Context) {
	h.recordAsset(c, func(in services.AssetPurchaseInput) (*models.Transaction, error) {
		return h.ledgerService.RecordShuttlePurchase(in)
	})
}

func (h *LedgerHandler) recordAsset(c *gin.Context, record func(services.AssetPurchaseInput) (*models.Transaction, error)) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	var req assetPurchaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
		return
	}

	occurredAt, err := parseOccurredAt(req.OccurredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
		return
	}

	txn, err := record(services.AssetPurchaseInput{
		AmountCents: req.AmountCents,
		Units:       req.Units,
		OccurredAt:  occurredAt,
		Description: req.Description,
		CreatedBy:   admin.ID,
	})
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, txn)
}

type openingBalancesRequest struct {
	OccurredAt string `json:"occurred_at"`
	Players    []struct {
		UserID       string `json:"user_id"`
		BalanceCents int64  `json:"balance_cents"`
	} `json:"players"`
	BankCents        int64 `json:"bank_cents"`
	CourtCreditCents int64 `json:"court_credit_cents"`
	ShuttleStock     struct {
		Units       int   `json:"units"`
		AmountCents int64 `json:"amount_cents"`
	} `json:"shuttle_stock"`
}

func (h *LedgerHandler) RecordOpeningBalances(c *gin.Context) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	var req openingBalancesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
		return
	}

	occurredAt, err := parseOccurredAt(req.OccurredAt)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": err.Error()})
		return
	}

	players := make([]services.OpeningPlayerBalance, 0, len(req.Players))
	for _, p := range req.Players {
		userID, err := uuid.Parse(p.UserID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid user ID: " + p.UserID})
			return
		}
		players = append(players, services.OpeningPlayerBalance{UserID: userID, BalanceCents: p.BalanceCents})
	}

	txn, err := h.ledgerService.RecordOpeningBalances(services.OpeningBalancesInput{
		Players:          players,
		BankCents:        req.BankCents,
		CourtCreditCents: req.CourtCreditCents,
		ShuttleUnits:     req.ShuttleStock.Units,
		ShuttleCents:     req.ShuttleStock.AmountCents,
		OccurredAt:       occurredAt,
		CreatedBy:        admin.ID,
	})
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, txn)
}

type reverseRequest struct {
	Description string `json:"description"`
}

// ReverseTransaction is the only way to undo anything. There is deliberately no
// edit or delete endpoint.
func (h *LedgerHandler) ReverseTransaction(c *gin.Context) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	transactionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid transaction ID"})
		return
	}

	var req reverseRequest
	_ = c.ShouldBindJSON(&req)

	txn, err := h.ledgerService.ReverseTransaction(transactionID, req.Description, admin.ID)
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, txn)
}

// --- helpers --------------------------------------------------------------

// parseOccurredAt accepts an RFC3339 timestamp, defaulting to now in Sydney.
// Money often gets recorded days after it moved, and the ledger should read in
// the order things actually happened.
func parseOccurredAt(value string) (time.Time, error) {
	if value == "" {
		return utils.NowInSydney(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, errors.New("occurred_at must be an RFC3339 timestamp")
	}
	return parsed.In(utils.SydneyLocation), nil
}

func lowBalanceThreshold() (int64, error) {
	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		return 0, err
	}
	return club.LowBalanceThresholdCents, nil
}
