package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/services"
)

// memberMoneyRequest is the shared reviewed-operation envelope. Proposed inputs
// are decoded into their kind-specific DTO separately, with this same strict
// helper. Actor identity always comes from middleware, never this JSON body.
type memberMoneyRequest struct {
	RequestID       uuid.UUID       `json:"request_id"`
	Reason          string          `json:"reason,omitempty"`
	Proposed        json.RawMessage `json:"proposed"`
	ExpectedPreview string          `json:"expected_preview,omitempty"`
}

// decodeMemberMoneyRequest rejects oversized bodies, duplicate keys, fractional
// money and unknown fields before a handler dispatches to business services.
// Route registration is a later task; this helper exposes no endpoint by itself.
func decodeMemberMoneyRequest(c *gin.Context, destination any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, services.MoneyMaxBodyBytes))
	if err != nil {
		return memberMoneyInputError("Send a valid money request within the size limit.")
	}
	return decodeMemberMoneyJSON(raw, destination)
}

func decodeMemberMoneyJSON(raw json.RawMessage, destination any) error {
	canonical, err := services.CanonicalMoneyPayload(raw)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(canonical))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return memberMoneyInputError("Check the request fields and exact integer amounts.")
	}
	return nil
}

func memberMoneyInputError(message string) *services.LedgerError {
	return &services.LedgerError{Code: "invalid_input", Status: http.StatusBadRequest, Message: message}
}
