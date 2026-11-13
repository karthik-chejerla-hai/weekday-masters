package services

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func moneyProtocolFixture(t *testing.T) (moneyOperationRequest, *LedgerService, uuid.UUID, uuid.UUID) {
	t.Helper()
	ledger, admin, player := newLedger(t)
	if err := database.DB.Model(&admin).Update("role", models.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	original, err := ledger.RecordTopup(CashInput{UserID: admin.ID, AmountCents: 5000, CreatedBy: admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	bank := clubAccount(t, ledger, models.AccountKindBank)
	return moneyOperationRequest{RequestID: uuid.New(), ActorID: admin.ID, Action: models.MoneyOperationReverseTransaction, TargetTransactionID: &original.ID, Reason: "  Wrong amount  ", Proposed: json.RawMessage(`{"amount_cents":100,"note":"test"}`), ExpectedPreview: strings.Repeat("a", 64)}, ledger, player, bank
}

// The future type-specific orchestration uses this same tx to lock the original,
// revalidate its review and post via the sole ledger writer. These tests exercise
// actual money and original-row locks, not a fake repository or a second writer.
func protocolPosting(in moneyOperationRequest, ledger *LedgerService, player, bank uuid.UUID) func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
	return func(tx *gorm.DB, operationID uuid.UUID, normalized moneyOperationRequest) (moneyOperationResult, error) {
		posted, err := ledger.PostWithin(tx, PostInput{Kind: models.TxnPlayerTopup, CreatedBy: normalized.ActorID, MoneyOperationID: &operationID, OperationStep: moneyStep(0), Movements: []Movement{{AccountID: player, AmountCents: 100}, {AccountID: bank, AmountCents: 100}}})
		if err != nil {
			return moneyOperationResult{}, err
		}
		body, err := json.Marshal(map[string]any{"operation_id": operationID, "replacement_transaction_id": posted.ID})
		return moneyOperationResult{Status: 201, Body: body}, err
	}
}
func moneyStep(step int16) *int16 { return &step }
func protocolErrorCode(t *testing.T, err error, code string) {
	t.Helper()
	e, ok := AsLedgerError(err)
	if !ok || string(e.Code) != code {
		t.Fatalf("error=%v; want %s", err, code)
	}
}
func operationCount(t *testing.T, id uuid.UUID) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.MoneyOperation{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMoneyOperationReplayPrecedesChangedState(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	first, err := runMoneyOperation(in, protocolPosting(in, ledger, player, bank))
	if err != nil {
		t.Fatal(err)
	}
	// A later correction-style link and additional posting must not invalidate an
	// already accepted same-payload retry or cause the type-specific callback to run.
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		_, err := ledger.PostWithin(tx, PostInput{Kind: models.TxnPlayerTopup, CorrectsTransactionID: in.TargetTransactionID, CreatedBy: in.ActorID, Movements: []Movement{{AccountID: player, AmountCents: 50}, {AccountID: bank, AmountCents: 50}}})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	retry := in
	retry.Reason = "Wrong amount"
	retry.Proposed = json.RawMessage("{\n \"note\": \"test\", \"amount_cents\": 100 }")
	replay, err := runMoneyOperation(retry, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
		t.Fatal("successful replay entered stale-state branch")
		return moneyOperationResult{}, nil
	})
	if err != nil || replay == nil || replay.Status != first.Status || !bytes.Equal(first.Body, replay.Body) {
		t.Fatalf("replay mismatch: first=%+v replay=%+v err=%v", first, replay, err)
	}
	if got := operationCount(t, in.RequestID); got != 1 {
		t.Fatalf("operations=%d", got)
	}
	balance, err := ledger.BalanceOfUser(in.ActorID)
	if err != nil || balance != 5150 {
		t.Fatalf("balance=%d err=%v", balance, err)
	}
}

func TestMoneyOperationRejectsChangedContentsOrOwner(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	if _, err := runMoneyOperation(in, protocolPosting(in, ledger, player, bank)); err != nil {
		t.Fatal(err)
	}
	other := newUser(t, "other-admin")
	if err := database.DB.Model(&other).Update("role", models.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*moneyOperationRequest){
		func(r *moneyOperationRequest) { r.Proposed = json.RawMessage(`{"amount_cents":101,"note":"test"}`) },
		func(r *moneyOperationRequest) { r.Reason = "Different reason" },
		func(r *moneyOperationRequest) { r.ExpectedPreview = strings.Repeat("b", 64) },
		func(r *moneyOperationRequest) { r.ActorID = other.ID },
	} {
		r := in
		change(&r)
		result, err := runMoneyOperation(r, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
			t.Fatal("conflicting request executed")
			return moneyOperationResult{}, nil
		})
		protocolErrorCode(t, err, "request_conflict")
		if result != nil {
			t.Fatal("conflict disclosed result")
		}
	}
}

func TestMoneyOperationReplayReauthorizesActor(t *testing.T) {
	for _, state := range []string{"removed", "demoted"} {
		t.Run(state, func(t *testing.T) {
			in, ledger, player, bank := moneyProtocolFixture(t)
			if _, err := runMoneyOperation(in, protocolPosting(in, ledger, player, bank)); err != nil {
				t.Fatal(err)
			}
			field, value := "role", string(models.RolePlayer)
			if state == "removed" {
				field, value = "membership_status", string(models.MembershipRemoved)
			}
			if err := database.DB.Model(&models.User{}).Where("id = ?", in.ActorID).Update(field, value).Error; err != nil {
				t.Fatal(err)
			}
			result, err := runMoneyOperation(in, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
				t.Fatal("unauthorized replay executed")
				return moneyOperationResult{}, nil
			})
			protocolErrorCode(t, err, "forbidden")
			if result != nil {
				t.Fatal("unauthorized replay disclosed stored body")
			}
		})
	}
}

func TestMoneyOperationFailureRollsBackPostingAndReservation(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	failure := errors.New("injected failure after posting")
	result, err := runMoneyOperation(in, func(tx *gorm.DB, id uuid.UUID, normalized moneyOperationRequest) (moneyOperationResult, error) {
		if _, err := protocolPosting(in, ledger, player, bank)(tx, id, normalized); err != nil {
			return moneyOperationResult{}, err
		}
		return moneyOperationResult{}, failure
	})
	if !errors.Is(err, failure) || result != nil {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if operationCount(t, in.RequestID) != 0 {
		t.Fatal("failed reservation committed")
	}
	balance, _ := ledger.BalanceOfUser(in.ActorID)
	if balance != 5000 {
		t.Fatalf("failed write changed balance=%d", balance)
	}
	if _, err := runMoneyOperation(in, protocolPosting(in, ledger, player, bank)); err != nil {
		t.Fatalf("retry after rollback: %v", err)
	}
}

func TestMoneyOperationResponseFailureRollsBack(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	for _, bad := range []moneyOperationResult{{Status: 500, Body: json.RawMessage(`{}`)}, {Status: 201, Body: json.RawMessage(`[]`)}} {
		called := false
		result, err := runMoneyOperation(in, func(tx *gorm.DB, id uuid.UUID, normalized moneyOperationRequest) (moneyOperationResult, error) {
			called = true
			if _, err := protocolPosting(in, ledger, player, bank)(tx, id, normalized); err != nil {
				return moneyOperationResult{}, err
			}
			return bad, nil
		})
		if !called {
			t.Fatal("response failure did not exercise the posting callback")
		}
		if err == nil || result != nil || operationCount(t, in.RequestID) != 0 {
			t.Fatalf("invalid completion committed: result=%+v err=%v", result, err)
		}
	}
	balance, _ := ledger.BalanceOfUser(in.ActorID)
	if balance != 5000 {
		t.Fatalf("invalid response posted balance=%d", balance)
	}
}

func TestMoneyOperationConcurrentIdenticalRetriesPostOnce(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	var calls atomic.Int32
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make(chan error, 8)
	bodies := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			result, err := runMoneyOperation(in, func(tx *gorm.DB, id uuid.UUID, normalized moneyOperationRequest) (moneyOperationResult, error) {
				calls.Add(1)
				return protocolPosting(in, ledger, player, bank)(tx, id, normalized)
			})
			if err != nil {
				errs <- err
				return
			}
			bodies <- string(result.Body)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	close(bodies)
	for err := range errs {
		t.Error(err)
	}
	var first string
	for body := range bodies {
		if first == "" {
			first = body
		} else if first != body {
			t.Fatal("retries returned different results")
		}
	}
	if calls.Load() != 1 || operationCount(t, in.RequestID) != 1 {
		t.Fatalf("executions=%d", calls.Load())
	}
	balance, _ := ledger.BalanceOfUser(in.ActorID)
	if balance != 5100 {
		t.Fatalf("duplicate money: balance=%d", balance)
	}
}

func TestMoneyOperationDistinctReservationsSerializeOriginal(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	ready := make(chan struct{}, 2)
	release := make(chan struct{})
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		request := in
		request.RequestID = uuid.New()
		go func() {
			_, err := runMoneyOperation(request, func(tx *gorm.DB, id uuid.UUID, normalized moneyOperationRequest) (moneyOperationResult, error) {
				ready <- struct{}{}
				select {
				case <-release:
				case <-time.After(5 * time.Second):
					return moneyOperationResult{}, fmt.Errorf("reservation barrier timed out")
				}
				if err := tx.Exec("SET LOCAL lock_timeout = '2s'").Error; err != nil {
					return moneyOperationResult{}, err
				}
				var original models.Transaction
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&original, "id = ?", *request.TargetTransactionID).Error; err != nil {
					return moneyOperationResult{}, err
				}
				var changed int64
				if err := tx.Model(&models.Transaction{}).Where("corrects_transaction_id = ?", original.ID).Count(&changed).Error; err != nil {
					return moneyOperationResult{}, err
				}
				if changed > 0 {
					return moneyOperationResult{}, newLedgerError("transaction_changed", 409, "Already changed.")
				}
				result, err := protocolPosting(request, ledger, player, bank)(tx, id, normalized)
				if err != nil {
					return moneyOperationResult{}, err
				}
				var posted models.Transaction
				if err := tx.Where("money_operation_id = ?", id).First(&posted).Error; err != nil {
					return moneyOperationResult{}, err
				}
				return result, tx.Model(&posted).Update("corrects_transaction_id", original.ID).Error
			})
			done <- err
		}()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-ready:
		case err := <-done:
			close(release)
			t.Fatalf("operation did not reach reservation barrier: %v", err)
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("both reservations must exist before original locks")
		}
	}
	close(release)
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		select {
		case err := <-done:
			if err == nil {
				success++
			} else {
				protocolErrorCode(t, err, "transaction_changed")
				conflict++
			}
		case <-time.After(5 * time.Second):
			t.Fatal("competing original lock did not finish")
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatalf("success=%d conflict=%d", success, conflict)
	}
	balance, _ := ledger.BalanceOfUser(in.ActorID)
	if balance != 5100 {
		t.Fatalf("competing operations posted balance=%d", balance)
	}
	var operations int64
	database.DB.Model(&models.MoneyOperation{}).Count(&operations)
	if operations != 1 {
		t.Fatalf("losing reservation survived: %d", operations)
	}
}

func TestMoneyOperationCanonicalPayloadAndFingerprint(t *testing.T) {
	first, err := MoneyReviewFingerprint(json.RawMessage(`{"balance_cents":100,"members":["Alex","Bea"]}`))
	if err != nil {
		t.Fatal(err)
	}
	second, err := MoneyReviewFingerprint(json.RawMessage("{\n\"members\":[\"Alex\",\"Bea\"],\"balance_cents\":100}"))
	if err != nil || first != second {
		t.Fatalf("equivalent fingerprint differs: %v", err)
	}
	changed, err := MoneyReviewFingerprint(json.RawMessage(`{"balance_cents":101,"members":["Alex","Bea"]}`))
	if err != nil || changed == first {
		t.Fatal("changed reviewed balance did not change fingerprint")
	}
	for _, raw := range []string{`{"x":1,"x":2}`, `{"x":{"y":1,"y":2}}`, `{"cents":1.0}`, `{"cents":1e2}`, `{"cents":4503599627370496}`, `{"cents":-4503599627370496}`, `[]`, `{} {}`, `{"x":`} {
		if _, err := CanonicalMoneyPayload(json.RawMessage(raw)); err == nil {
			t.Errorf("invalid JSON/money accepted: %s", raw)
		}
	}
	exact, err := CanonicalMoneyPayload(json.RawMessage(`{"cents":4503599627370495,"small":-4503599627370495}`))
	if err != nil || string(exact) != `{"cents":4503599627370495,"small":-4503599627370495}` {
		t.Fatalf("large exact cents lost: %s err=%v", exact, err)
	}
}

func TestMoneyOperationRejectsInvalidInputBeforeCallback(t *testing.T) {
	in, _, _, _ := moneyProtocolFixture(t)
	for _, change := range []func(*moneyOperationRequest){
		func(r *moneyOperationRequest) { r.RequestID = uuid.Nil },
		func(r *moneyOperationRequest) { r.ActorID = uuid.Nil },
		func(r *moneyOperationRequest) { r.Reason = " \n " },
		func(r *moneyOperationRequest) { r.Reason = strings.Repeat("界", 1001) },
		func(r *moneyOperationRequest) { r.ExpectedPreview = "" },
		func(r *moneyOperationRequest) { r.ExpectedPreview = strings.Repeat("z", 64) },
		func(r *moneyOperationRequest) { r.TargetTransactionID = nil },
		func(r *moneyOperationRequest) { r.Proposed = json.RawMessage(`{"x":1,"x":2}`) },
	} {
		request := in
		change(&request)
		result, err := runMoneyOperation(request, func(*gorm.DB, uuid.UUID, moneyOperationRequest) (moneyOperationResult, error) {
			t.Fatal("invalid input executed")
			return moneyOperationResult{}, nil
		})
		if err == nil || result != nil {
			t.Fatal("invalid input accepted")
		}
	}
	if operationCount(t, in.RequestID) != 0 {
		t.Fatal("invalid input persisted reservation")
	}
}

func TestMoneyOperationCommitFailureReturnsNoSuccess(t *testing.T) {
	in, ledger, player, bank := moneyProtocolFixture(t)
	missing := uuid.New()
	in.TargetTransactionID = &missing
	called := false
	result, err := runMoneyOperation(in, func(tx *gorm.DB, id uuid.UUID, normalized moneyOperationRequest) (moneyOperationResult, error) {
		called = true
		if normalized.Reason != "Wrong amount" {
			t.Fatal("callback did not receive normalized reason")
		}
		return protocolPosting(in, ledger, player, bank)(tx, id, normalized)
	})
	if !called || err == nil || result != nil {
		t.Fatalf("deferred FK failure returned success: called=%v result=%+v err=%v", called, result, err)
	}
	if operationCount(t, in.RequestID) != 0 {
		t.Fatal("commit failure retained reservation")
	}
	balance, err := ledger.BalanceOfUser(in.ActorID)
	if err != nil || balance != 5000 {
		t.Fatalf("commit failure changed money: balance=%d err=%v", balance, err)
	}
}
