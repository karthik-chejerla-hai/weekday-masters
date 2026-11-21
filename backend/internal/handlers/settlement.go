package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/middleware"
	"github.com/weekday-masters/backend/internal/services"
)

type SettlementHandler struct {
	settlementService *services.SettlementService
}

func NewSettlementHandler(settlementService *services.SettlementService) *SettlementHandler {
	return &SettlementHandler{settlementService: settlementService}
}

// settlementRequest is the settlement form's state.
//
// Rates are pointers so that "not supplied" is distinguishable from "zero" — the
// common case is submitting only the participant list and letting club settings
// fill in the rest, and an admin setting the extra hours to zero explicitly is a
// different statement from omitting them.
//
// ExpectedPreview is the fingerprint of the preview the admin reviewed. Preview
// ignores it; settle requires it, so the amount posted is the amount approved.
type settlementRequest struct {
	ExpectedPreview string   `json:"expected_preview"`
	BaseHours       *float64 `json:"base_hours"`
	BaseRateCents   *int64   `json:"base_rate_cents"`
	ExtraHours      *float64 `json:"extra_hours"`
	ExtraRateCents  *int64   `json:"extra_rate_cents"`
	ShuttlesPerHour *float64 `json:"shuttles_per_hour"`
	Lines           []struct {
		UserID    string `json:"user_id"`
		GuestName string `json:"guest_name"`
		InBase    bool   `json:"in_base"`
		InExtra   bool   `json:"in_extra"`
		Comped    bool   `json:"comped"`
	} `json:"lines"`
}

// errEmptyBody marks a request that carried no JSON at all, as distinct from one
// that carried JSON which could not be read.
var errEmptyBody = errors.New("request body is empty")

// decodeJSONBody reads exactly one JSON value from the request into dst.
//
// It exists because the money routes must not act on input they could not read.
// A decode error that is ignored leaves dst at its zero value, and a zero-valued
// settlement request is a valid one: it settles the session on club defaults.
// So a mistyped field, broken JSON, or a second value after the first is an
// error here, never a default.
//
// An absent body (nothing, or only whitespace) is reported as errEmptyBody so
// each route can decide explicitly whether "no body" is acceptable.
func decodeJSONBody(c *gin.Context, dst any) error {
	if c.Request.Body == nil {
		return errEmptyBody
	}
	dec := json.NewDecoder(c.Request.Body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return errEmptyBody
		}
		return err
	}
	// Anything after the first value means the client sent something other than
	// the one document this route understands.
	if err := dec.Decode(&json.RawMessage{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must hold a single JSON value")
	}
	return nil
}

// decodeOptionalJSONBody is decodeJSONBody for routes where every field is
// optional: no body leaves dst untouched, but a body that is present must parse.
func decodeOptionalJSONBody(c *gin.Context, dst any) error {
	if err := decodeJSONBody(c, dst); err != nil && !errors.Is(err, errEmptyBody) {
		return err
	}
	return nil
}

func respondInvalidBody(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid request body"})
}

func (r settlementRequest) toInput(sessionID, settledBy uuid.UUID) (services.SettleInput, error) {
	in := services.SettleInput{
		SessionID:       sessionID,
		BaseHours:       r.BaseHours,
		BaseRateCents:   r.BaseRateCents,
		ExtraHours:      r.ExtraHours,
		ExtraRateCents:  r.ExtraRateCents,
		ShuttlesPerHour: r.ShuttlesPerHour,
		ExpectedPreview: r.ExpectedPreview,
		SettledBy:       settledBy,
	}

	// A nil line list means "use the default": everyone who said they were
	// coming. An empty list is a different thing and stays empty.
	if r.Lines == nil {
		return in, nil
	}

	lines := make([]services.LineInput, 0, len(r.Lines))
	for _, line := range r.Lines {
		userID, err := uuid.Parse(line.UserID)
		if err != nil {
			return services.SettleInput{}, err
		}
		lines = append(lines, services.LineInput{
			UserID:    userID,
			GuestName: line.GuestName,
			InBase:    line.InBase,
			InExtra:   line.InExtra,
			Comped:    line.Comped,
		})
	}
	in.Lines = lines
	return in, nil
}

// PreviewSettlement costs a settlement without writing anything.
//
// The form calls this on every change, so what the admin is looking at is always
// what pressing settle will post. It also surfaces a shuttle shortfall here,
// before they commit, rather than after.
func (h *SettlementHandler) PreviewSettlement(c *gin.Context) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid session ID"})
		return
	}

	var req settlementRequest
	// A preview with no body is valid: it returns the default. A body that is
	// present but unreadable is not "no body", and must not be costed as one.
	if err := decodeOptionalJSONBody(c, &req); err != nil {
		respondInvalidBody(c)
		return
	}

	input, err := req.toInput(sessionID, admin.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid user ID on a line"})
		return
	}

	preview, err := h.settlementService.Preview(input)
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusOK, preview)
}

// SettleSession posts the settlement.
func (h *SettlementHandler) SettleSession(c *gin.Context) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid session ID"})
		return
	}

	// Nothing may post from a request that could not be read in full, and an
	// empty body is not a settlement: it carries no reviewed preview.
	var req settlementRequest
	if err := decodeJSONBody(c, &req); err != nil {
		respondInvalidBody(c)
		return
	}
	if len(req.ExpectedPreview) != services.PreviewFingerprintLength {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "bad_request",
			"message": "expected_preview is required: review a settlement preview before settling.",
		})
		return
	}

	input, err := req.toInput(sessionID, admin.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid user ID on a line"})
		return
	}

	settlement, preview, err := h.settlementService.Settle(input)
	if err != nil {
		respondLedgerError(c, err)
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"settlement":  settlement,
		"bands":       preview.Bands,
		"totals":      preview.Totals,
		"lines":       preview.Lines,
		"stock_after": preview.StockAfter,
	})
}

// ReverseSettlement unwinds a settled session so it can be settled again.
func (h *SettlementHandler) ReverseSettlement(c *gin.Context) {
	admin, err := middleware.GetUserFromContext(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": "unauthorized", "message": err.Error()})
		return
	}

	settlementID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid settlement ID"})
		return
	}

	// The description is optional, so no body is fine; a broken one is not.
	var req reverseRequest
	if err := decodeOptionalJSONBody(c, &req); err != nil {
		respondInvalidBody(c)
		return
	}

	txn, err := h.settlementService.ReverseSettlement(settlementID, req.Description, admin.ID)
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	c.JSON(http.StatusCreated, txn)
}

// ListSessionHistory returns past sessions, settled or not.
//
// An unsettled finished session still appears, marked as awaiting settlement,
// rather than being hidden — it is exactly the thing the admin needs to be
// reminded about.
func (h *SettlementHandler) ListSessionHistory(c *gin.Context) {
	limit, offset := parsePaging(c)

	items, total, err := h.settlementService.ListPastSessions(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": "internal", "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "total": total})
}

// GetSessionSettlement returns the full breakdown of a settled session, readable
// by any approved member so the split can be checked by the people in it.
func (h *SettlementHandler) GetSessionSettlement(c *gin.Context) {
	sessionID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid session ID"})
		return
	}

	settlementID := uuid.Nil
	if raw, provided := c.GetQuery("settlement_id"); provided {
		parsed, parseErr := uuid.Parse(raw)
		if parseErr != nil || parsed == uuid.Nil {
			c.JSON(http.StatusBadRequest, gin.H{"code": "bad_request", "message": "Invalid settlement ID"})
			return
		}
		settlementID = parsed
	}
	view, err := h.settlementService.SettlementForSessionVersion(sessionID, settlementID)
	if err != nil {
		respondLedgerError(c, err)
		return
	}
	if view == nil {
		c.JSON(http.StatusNotFound, gin.H{"code": "not_settled", "message": "This session has not been settled yet."})
		return
	}
	c.JSON(http.StatusOK, view)
}
