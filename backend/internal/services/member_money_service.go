package services

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MoneyMaxCents int64 = 4503599627370495
const MoneyMaxBodyBytes = 128 * 1024

// CanonicalMoneyPayload validates a bounded JSON object, rejecting duplicate
// keys and non-integer/out-of-range numbers, and sorts object keys. Domain
// services must normalize their own dates, defaults and member/line arrays
// before submission: sorting arbitrary JSON arrays would change their meaning.
func CanonicalMoneyPayload(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) > MoneyMaxBodyBytes {
		return nil, moneyInputError("Enter a valid money request within the size limit.")
	}
	return canonicalMoneyObject(raw)
}

// Stored JSONB text can be larger than its accepted compact representation
// because PostgreSQL adds whitespace. Bound the canonical result, not database
// formatting, while wire input still has the strict raw-byte cap above.
func canonicalMoneyObject(raw json.RawMessage) (json.RawMessage, error) {
	if !utf8.Valid(raw) {
		return nil, moneyInputError("Enter valid UTF-8 JSON.")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	value, err := decodeMoneyValue(decoder, 0)
	if err != nil {
		return nil, moneyInputError("Enter valid JSON with unique keys and exact integer values.")
	}
	if _, ok := value.(map[string]any); !ok {
		return nil, moneyInputError("The money request must be a JSON object.")
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, moneyInputError("Send exactly one JSON object.")
	}
	result, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(result) > MoneyMaxBodyBytes {
		return nil, moneyInputError("The canonical money request is too large.")
	}
	return result, nil
}

func decodeMoneyValue(decoder *json.Decoder, depth int) (any, error) {
	if depth > 64 {
		return nil, errors.New("JSON is too deeply nested")
	}
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	switch v := token.(type) {
	case json.Delim:
		switch v {
		case '{':
			result := map[string]any{}
			for decoder.More() {
				key, err := decoder.Token()
				if err != nil {
					return nil, err
				}
				name, ok := key.(string)
				if !ok {
					return nil, errors.New("invalid object key")
				}
				if _, exists := result[name]; exists {
					return nil, errors.New("duplicate key")
				}
				value, err := decodeMoneyValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				result[name] = value
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim('}') {
				return nil, errors.New("invalid object")
			}
			return result, nil
		case '[':
			result := []any{}
			for decoder.More() {
				value, err := decodeMoneyValue(decoder, depth+1)
				if err != nil {
					return nil, err
				}
				result = append(result, value)
			}
			end, err := decoder.Token()
			if err != nil || end != json.Delim(']') {
				return nil, errors.New("invalid array")
			}
			return result, nil
		}
	case json.Number:
		number, err := strconv.ParseInt(string(v), 10, 64)
		if err != nil || number < -MoneyMaxCents || number > MoneyMaxCents {
			return nil, errors.New("invalid integer")
		}
		return number, nil
	case string, bool, nil:
		return v, nil
	}
	return nil, errors.New("invalid JSON value")
}

func moneyInputError(message string) *LedgerError {
	return newLedgerError("invalid_input", 400, message)
}

// MoneyReviewFingerprint is read-only. The type-specific preview supplies all
// canonical inputs and relevant state from one snapshot; this helper does not
// substitute for discovering/locking that state during confirmation.
func MoneyReviewFingerprint(review json.RawMessage) (string, error) {
	canonical, err := CanonicalMoneyPayload(review)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(append([]byte("member-money-review:v1\n"), canonical...))
	return hex.EncodeToString(digest[:]), nil
}

type moneyOperationRequest struct {
	RequestID           uuid.UUID
	ActorID             uuid.UUID
	Action              models.MoneyOperationAction
	TargetTransactionID *uuid.UUID
	Reason              string
	Proposed            json.RawMessage
	ExpectedPreview     string
	// Resolved from immutable original data and normalized proposal before locks.
	// All additional users must be known before taking any session/account lock.
	MemberIDs []uuid.UUID
}

type moneyOperationResult struct {
	Status int16
	Body   json.RawMessage
}

func canonicalMoneyRequest(in moneyOperationRequest) (moneyOperationRequest, string, string, error) {
	if in.RequestID == uuid.Nil {
		return in, "", "", moneyInputError("Provide a submission ID.")
	}
	if in.ActorID == uuid.Nil {
		return in, "", "", newLedgerError("unauthorized", 401, "Sign in to continue.")
	}
	switch in.Action {
	case models.MoneyOperationCreateSharedExpense:
		if in.TargetTransactionID != nil {
			return in, "", "", moneyInputError("A new expense cannot replace an existing transaction.")
		}
	case models.MoneyOperationCorrectTopup, models.MoneyOperationReverseTransaction, models.MoneyOperationCorrectSharedExpense, models.MoneyOperationCorrectSettlement:
		if in.TargetTransactionID == nil || *in.TargetTransactionID == uuid.Nil {
			return in, "", "", moneyInputError("Select an original transaction.")
		}
	default:
		return in, "", "", newLedgerError("unsupported_transaction", 422, "This operation is not supported.")
	}
	if len(in.ExpectedPreview) != 64 {
		return in, "", "", newLedgerError("preview_required", 409, "Review this operation before confirming.")
	}
	decoded, err := hex.DecodeString(in.ExpectedPreview)
	if err != nil || len(decoded) != 32 || strings.ToLower(in.ExpectedPreview) != in.ExpectedPreview {
		return in, "", "", newLedgerError("preview_required", 409, "Review this operation before confirming.")
	}
	in.Reason = strings.TrimSpace(in.Reason)
	if !utf8.ValidString(in.Reason) || utf8.RuneCountInString(in.Reason) > 1000 || (in.Action != models.MoneyOperationCreateSharedExpense && in.Reason == "") {
		return in, "", "", moneyInputError("A reason of 1–1000 characters is required.")
	}
	in.Proposed, err = CanonicalMoneyPayload(in.Proposed)
	if err != nil {
		return in, "", "", err
	}
	members := map[uuid.UUID]bool{in.ActorID: true}
	for _, id := range in.MemberIDs {
		if id == uuid.Nil {
			return in, "", "", moneyInputError("Select valid members.")
		}
		members[id] = true
	}
	if len(members) > 201 {
		return in, "", "", moneyInputError("Select at most 100 original and 100 replacement members.")
	}
	in.MemberIDs = make([]uuid.UUID, 0, len(members))
	for id := range members {
		in.MemberIDs = append(in.MemberIDs, id)
	}
	sort.Slice(in.MemberIDs, func(i, j int) bool { return in.MemberIDs[i].String() < in.MemberIDs[j].String() })
	envelope := struct {
		Version         string                      `json:"version"`
		RequestID       uuid.UUID                   `json:"request_id"`
		ActorID         uuid.UUID                   `json:"actor_id"`
		Action          models.MoneyOperationAction `json:"action"`
		Target          *uuid.UUID                  `json:"target_transaction_id"`
		Reason          string                      `json:"reason"`
		Proposed        json.RawMessage             `json:"proposed"`
		ExpectedPreview string                      `json:"expected_preview"`
	}{"member-money-submit:v1", in.RequestID, in.ActorID, in.Action, in.TargetTransactionID, in.Reason, in.Proposed, in.ExpectedPreview}
	payload, err := json.Marshal(envelope)
	if err != nil {
		return in, "", "", err
	}
	if len(payload) > MoneyMaxBodyBytes {
		return in, "", "", moneyInputError("The money request is too large.")
	}
	hash := sha256.Sum256(payload)
	return in, string(payload), hex.EncodeToString(hash[:]), nil
}

func requireMoneyActor(tx *gorm.DB, id uuid.UUID) error {
	var actor models.User
	if err := tx.First(&actor, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return newLedgerError("forbidden", 403, "Only approved admins can record money operations.")
		}
		return err
	}
	if !actor.IsApproved() || !actor.IsAdmin() {
		return newLedgerError("forbidden", 403, "Only approved admins can record money operations.")
	}
	return nil
}

func lockMoneyMembers(tx *gorm.DB, ids []uuid.UUID) error {
	var users []models.User
	if err := tx.Clauses(clause.Locking{Strength: "SHARE"}).Where("id IN ?", ids).Order("id").Find(&users).Error; err != nil {
		return err
	}
	if len(users) != len(ids) {
		return moneyInputError("A selected member no longer exists.")
	}
	return nil
}

// runMoneyOperation is an internal service boundary, not a handler API. The
// callback must lock the exact current original and complete account union,
// recompute/compare the review, and post only via LedgerService using this tx.
// No callback runs for an accepted identical retry. No notifier runs here.
func runMoneyOperation(in moneyOperationRequest, apply func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error)) (*moneyOperationResult, error) {
	in, payload, hash, err := canonicalMoneyRequest(in)
	if err != nil {
		return nil, err
	}
	if apply == nil {
		return nil, errors.New("money operation requires a transaction callback")
	}
	var result *moneyOperationResult
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := lockFinancialTransactions(tx); err != nil {
			return err
		}
		// Preliminary read avoids reserving against a nonexistent/unauthorized actor.
		// The authoritative check is repeated under SHARE lock on both branches.
		if err := requireMoneyActor(tx, in.ActorID); err != nil {
			return err
		}
		reserved := models.MoneyOperation{ID: in.RequestID, ActorID: in.ActorID, Action: in.Action, TargetTransactionID: in.TargetTransactionID, PayloadHash: hash, CanonicalPayload: payload}
		if err := tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "id"}}, DoNothing: true}).Create(&reserved).Error; err != nil {
			return err
		}
		var operation models.MoneyOperation
		if err := tx.First(&operation, "id = ?", in.RequestID).Error; err != nil {
			return err
		}
		if operation.ActorID != in.ActorID || operation.PayloadHash != hash {
			return newLedgerError("request_conflict", 409, "This submission ID was already used for different contents.")
		}
		members := in.MemberIDs
		if operation.ResponseStatus != nil {
			members = []uuid.UUID{in.ActorID}
		}
		if err := lockMoneyMembers(tx, members); err != nil {
			return err
		}
		if err := requireMoneyActor(tx, in.ActorID); err != nil {
			return err
		}
		if operation.ResponseStatus != nil {
			if operation.ResponseBody == nil {
				return errors.New("completed money operation has no response")
			}
			body, err := canonicalMoneyObject(json.RawMessage(*operation.ResponseBody))
			if err != nil {
				return err
			}
			result = &moneyOperationResult{Status: *operation.ResponseStatus, Body: body}
			return nil
		}
		response, err := apply(tx, operation.ID, in)
		if err != nil {
			return err
		}
		if response.Status < 200 || response.Status > 299 {
			return errors.New("money operation did not produce a successful response")
		}
		body, err := CanonicalMoneyPayload(response.Body)
		if err != nil {
			return err
		}
		// All type-specific locks have been acquired by this point. The recorded time
		// is wall-clock completion, not the transaction's possibly much earlier start.
		updated := tx.Model(&models.MoneyOperation{}).Where("id = ? AND response_status IS NULL", operation.ID).Updates(map[string]any{"created_at": time.Now().UTC(), "response_status": response.Status, "response_body": string(body)})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected != 1 {
			return errors.New("money operation completion was not stored")
		}
		result = &moneyOperationResult{Status: response.Status, Body: body}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// MoneyOperationPreview is the common read-only review DTO. Kind-specific
// services supply its snapshots, canonical input and fingerprint from one DB
// snapshot; these types do not themselves construct a financial review.
type MoneyOperationPreview struct {
	RequestID           uuid.UUID                   `json:"request_id"`
	Action              models.MoneyOperationAction `json:"action"`
	TargetTransactionID *uuid.UUID                  `json:"target_transaction_id"`
	TargetSettlementID  *uuid.UUID                  `json:"target_settlement_id,omitempty"`
	Input               json.RawMessage             `json:"input"`
	ExpectedPreview     string                      `json:"expected_preview"`
	Original            json.RawMessage             `json:"original,omitempty"`
	Replacement         json.RawMessage             `json:"replacement,omitempty"`
	Effects             MoneyOperationEffects       `json:"effects"`
	Explanation         string                      `json:"explanation"`
}

type MoneyOperationEffects struct {
	Members []MoneyMemberEffect `json:"members"`
	Assets  []MoneyAssetEffect  `json:"assets"`
}

type MoneyMemberEffect struct {
	UserID               uuid.UUID `json:"user_id"`
	DisplayName          string    `json:"display_name"`
	Inactive             bool      `json:"inactive"`
	OriginalPaidCents    *int64    `json:"original_paid_cents"`
	OriginalOwedCents    *int64    `json:"original_owed_cents"`
	ReplacementPaidCents *int64    `json:"replacement_paid_cents"`
	ReplacementOwedCents *int64    `json:"replacement_owed_cents"`
	DeltaCents           int64     `json:"delta_cents"`
	CurrentBalanceCents  int64     `json:"current_balance_cents"`
	NewBalanceCents      int64     `json:"new_balance_cents"`
}

type MoneyAssetEffect struct {
	Kind        models.AccountKind `json:"kind"`
	BeforeCents int64              `json:"before_cents"`
	DeltaCents  int64              `json:"delta_cents"`
	AfterCents  int64              `json:"after_cents"`
	BeforeUnits *int               `json:"before_units,omitempty"`
	DeltaUnits  *int               `json:"delta_units,omitempty"`
	AfterUnits  *int               `json:"after_units,omitempty"`
}
