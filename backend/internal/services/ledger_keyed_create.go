package services

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

// keyedCreate is what one submission key is bound to: the kind of create and
// the contents the admin submitted. The actor is bound from PostInput.CreatedBy.
type keyedCreate struct {
	RequestID uuid.UUID
	Action    models.MoneyOperationAction
	Contents  map[string]any
	// MemberIDs are SHARE-locked for the posting, so a member cannot be removed
	// between the request arriving and their balance moving.
	MemberIDs []uuid.UUID
}

// submittedInstant is the date as the admin gave it. An omitted date means
// "now" and is deliberately recorded as absent: the clock at the retry is not
// the clock at the original, and binding it would turn every retry into a
// conflict. A supplied date is normalised so that one instant written in two
// zones is one submission.
func submittedInstant(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339Nano)
}

// postCreate posts a top-up, withdrawal or asset purchase exactly once per
// submission key.
//
// The failure it exists for: the write commits, the HTTP reply is lost, and the
// admin (or the client on their behalf) sends the receipt again. Without a key
// the second request is indistinguishable from a second receipt. With one, the
// operation row is reserved, the transaction posted and the response stored in
// a single database transaction; an identical retry returns the stored response
// without posting, and the same key with different contents or a different
// admin is refused with request_conflict.
//
// A nil key posts directly. That path is for in-process callers that run once
// and have no client to retry them (cmd/seed, fixtures). The HTTP handlers
// refuse a missing key before reaching here.
func (s *LedgerService) postCreate(key keyedCreate, post PostInput) (*models.Transaction, error) {
	if key.RequestID == uuid.Nil {
		return s.Post(post)
	}
	// jsonb cannot hold a NUL and the contents are stored as jsonb. Refuse it by
	// name rather than surfacing a driver error.
	if strings.ContainsRune(post.Description, 0) {
		return nil, moneyInputError("Enter a description without control characters.")
	}
	proposed, err := json.Marshal(key.Contents)
	if err != nil {
		return nil, err
	}
	result, err := runMoneyOperation(moneyOperationRequest{
		RequestID: key.RequestID, ActorID: post.CreatedBy, Action: key.Action,
		Proposed: proposed, MemberIDs: key.MemberIDs,
	}, func(tx *gorm.DB, operationID uuid.UUID, _ moneyOperationRequest) (moneyOperationResult, error) {
		step := int16(0)
		post.MoneyOperationID = &operationID
		post.OperationStep = &step
		txn, err := s.PostWithin(tx, post)
		if err != nil {
			return moneyOperationResult{}, err
		}
		body, err := json.Marshal(txn)
		if err != nil {
			return moneyOperationResult{}, err
		}
		return moneyOperationResult{Status: 201, Body: body}, nil
	})
	if err != nil {
		return nil, err
	}
	// Decoded from the stored response on both branches, so the first reply and
	// every replay of it are the same bytes.
	var txn models.Transaction
	if err := json.Unmarshal(result.Body, &txn); err != nil {
		return nil, err
	}
	return &txn, nil
}
