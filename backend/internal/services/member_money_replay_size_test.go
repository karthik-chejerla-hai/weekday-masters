package services

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestMoneyOperationReplaysStoredJSONBWhitespaceExpansion(t *testing.T) {
	in, _, _, _ := moneyProtocolFixture(t)
	body := json.RawMessage(`{"rows":[` + strings.Repeat("0,", 49999) + `0]}`)
	if len(body) >= MoneyMaxBodyBytes {
		t.Fatal("fixture exceeds accepted wire size")
	}
	first, err := runMoneyOperation(in, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
		return moneyOperationResult{Status: 201, Body: body}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	replay, err := runMoneyOperation(in, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
		t.Fatal("accepted replay executed twice")
		return moneyOperationResult{}, nil
	})
	if err != nil {
		t.Fatalf("accepted compact response (%d bytes) failed replay: %v", len(body), err)
	}
	if first.Status != replay.Status || !bytes.Equal(first.Body, replay.Body) {
		t.Fatal("successful replay changed accepted response")
	}
}

func TestMoneyOperationRejectsCanonicalExpansionBeforeSuccess(t *testing.T) {
	in, _, _, _ := moneyProtocolFixture(t)
	// encoding/json escapes HTML characters, so normalization can itself expand
	// otherwise-small input. Never accept a result that exceeds the canonical cap.
	body := json.RawMessage(`{"text":"` + strings.Repeat("<", 30000) + `"}`)
	result, err := runMoneyOperation(in, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
		return moneyOperationResult{Status: 201, Body: body}, nil
	})
	if err == nil || result != nil || operationCount(t, in.RequestID) != 0 {
		t.Fatal("oversized canonical result was accepted")
	}
}
