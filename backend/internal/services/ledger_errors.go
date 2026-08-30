package services

import (
	"errors"
	"fmt"

	"github.com/weekday-masters/backend/internal/services/money"
)

// Ledger errors carry a stable code so the frontend can react to a specific
// condition rather than pattern-matching on message text. The codes are part of
// the API contract — see specs/001-club-ledger-settlement/contracts/README.md.

type LedgerErrorCode string

const (
	CodeShuttleStockShort         LedgerErrorCode = "shuttle_stock_short"
	CodeSessionAlreadySettled     LedgerErrorCode = "session_already_settled"
	CodeTransactionAlreadyReverse LedgerErrorCode = "transaction_already_reversed"
	CodeInvariantViolated         LedgerErrorCode = "invariant_violated"
	CodeNotSettleable             LedgerErrorCode = "not_settleable"
	CodeOpeningBalancesRecorded   LedgerErrorCode = "opening_balances_already_recorded"
)

// LedgerError is a domain failure with a wire code and optional structured
// details. Handlers translate it directly into the error body; nothing else
// needs to know how a particular failure is phrased.
type LedgerError struct {
	Code    LedgerErrorCode
	Message string
	Details map[string]any
	Status  int
	wrapped error
}

func (e *LedgerError) Error() string { return string(e.Code) + ": " + e.Message }
func (e *LedgerError) Unwrap() error { return e.wrapped }

func newLedgerError(code LedgerErrorCode, status int, message string) *LedgerError {
	return &LedgerError{Code: code, Status: status, Message: message}
}

// ErrShuttleStockShort reports that settlement needs more shuttles than the club
// holds. It carries both counts so the settlement form can offer to record the
// missing purchase inline instead of just refusing.
func ErrShuttleStockShort(required, available int) *LedgerError {
	return &LedgerError{
		Code:   CodeShuttleStockShort,
		Status: 422,
		Message: fmt.Sprintf(
			"This session uses %d shuttles but stock holds %d. Record the purchase you have not entered yet, then settle.",
			required, available),
		Details: map[string]any{
			"required_units":  required,
			"available_units": available,
		},
	}
}

func ErrSessionAlreadySettled() *LedgerError {
	return newLedgerError(CodeSessionAlreadySettled, 409,
		"This session is already settled. Reverse the settlement before settling it again.")
}

func ErrTransactionAlreadyReversed() *LedgerError {
	return newLedgerError(CodeTransactionAlreadyReverse, 409,
		"This transaction has already been reversed.")
}

func ErrNotSettleable(reason string) *LedgerError {
	return newLedgerError(CodeNotSettleable, 422, reason)
}

func ErrOpeningBalancesAlreadyRecorded() *LedgerError {
	return newLedgerError(CodeOpeningBalancesRecorded, 409,
		"Opening balances have already been recorded. Correct them with adjusting transactions instead.")
}

// ErrInvariantViolated means the club-position identity did not hold. The
// transaction that produced it has been rolled back, so nothing was written —
// but reaching this at all means a caller constructed movements that do not
// balance, which is a bug rather than a user error.
func ErrInvariantViolated(residualCents int64) *LedgerError {
	return &LedgerError{
		Code:   CodeInvariantViolated,
		Status: 500,
		Message: fmt.Sprintf(
			"Refusing to write: the ledger would be out by %d cents. Nothing was saved.", residualCents),
		Details: map[string]any{"residual_cents": residualCents},
	}
}

// AsLedgerError converts an error into a LedgerError where one is available,
// translating the money package's typed failures on the way through so callers
// do not have to know both vocabularies.
func AsLedgerError(err error) (*LedgerError, bool) {
	if err == nil {
		return nil, false
	}

	var ledgerErr *LedgerError
	if errors.As(err, &ledgerErr) {
		return ledgerErr, true
	}

	var short *money.InsufficientStockError
	if errors.As(err, &short) {
		return ErrShuttleStockShort(short.RequiredUnits, short.AvailableUnits), true
	}

	return nil, false
}
