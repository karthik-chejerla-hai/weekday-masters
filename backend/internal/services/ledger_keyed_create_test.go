package services

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func keyedLedger(t *testing.T) (*LedgerService, models.User, models.User) {
	t.Helper()
	ls, player, _ := newLedger(t)
	admin := newUser(t, "treasurer")
	if err := database.DB.Model(&admin).Update("role", models.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	return ls, admin, player
}

func transactionsOfKind(t *testing.T, kind models.TransactionKind) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.Transaction{}).Where("kind = ?", kind).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

// The retry that matters most is the one that overlaps the original: a client
// that timed out and resent while the first request was still in flight.
func TestKeyedTopupConcurrentIdenticalRetriesPostOnce(t *testing.T) {
	ls, admin, player := keyedLedger(t)
	in := CashInput{RequestID: uuid.New(), UserID: player.ID, AmountCents: 5000, Description: "Bank transfer", CreatedBy: admin.ID}

	const attempts = 8
	start := make(chan struct{})
	ids := make([]uuid.UUID, attempts)
	errs := make([]error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			txn, err := ls.RecordTopup(in)
			if errs[i] = err; err == nil {
				ids[i] = txn.ID
			}
		}(i)
	}
	close(start)
	wg.Wait()

	for i := range errs {
		if errs[i] != nil {
			t.Fatalf("attempt %d: %v", i, errs[i])
		}
		if ids[i] != ids[0] {
			t.Errorf("attempt %d returned %s, want the one transaction %s", i, ids[i], ids[0])
		}
	}
	if got := transactionsOfKind(t, models.TxnPlayerTopup); got != 1 {
		t.Errorf("%d top-ups posted, want 1", got)
	}
	if balance, err := ls.BalanceOfUser(player.ID); err != nil || balance != 5000 {
		t.Errorf("balance = %d / %v, want 5000", balance, err)
	}
	assertBalanced(t)
}

// "Now" is not part of what the admin submitted. A retry that omits the date
// must match the original although the clock has moved, and must return the
// time the original was recorded with.
func TestKeyedCreateBindsTheSubmittedDateNotTheClock(t *testing.T) {
	ls, admin, player := keyedLedger(t)
	in := CashInput{RequestID: uuid.New(), UserID: player.ID, AmountCents: 5000, CreatedBy: admin.ID}
	first, err := ls.RecordTopup(in)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	retry, err := ls.RecordTopup(in)
	if err != nil {
		t.Fatalf("retry without a date: %v", err)
	}
	if retry.ID != first.ID || !retry.OccurredAt.Equal(first.OccurredAt) {
		t.Errorf("retry = %s at %s, want %s at %s", retry.ID, retry.OccurredAt, first.ID, first.OccurredAt)
	}

	// The same instant written in another zone is the same submission; a
	// different instant is not.
	dated := CashInput{RequestID: uuid.New(), UserID: player.ID, AmountCents: 5000, CreatedBy: admin.ID,
		OccurredAt: time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)}
	if _, err := ls.RecordTopup(dated); err != nil {
		t.Fatal(err)
	}
	dated.OccurredAt = dated.OccurredAt.In(time.FixedZone("AEDT", 11*3600))
	if _, err := ls.RecordTopup(dated); err != nil {
		t.Errorf("same instant in another zone: %v", err)
	}
	dated.OccurredAt = dated.OccurredAt.Add(time.Hour)
	_, err = ls.RecordTopup(dated)
	protocolErrorCode(t, err, "request_conflict")

	if got := transactionsOfKind(t, models.TxnPlayerTopup); got != 2 {
		t.Errorf("%d top-ups posted, want 2", got)
	}
}

// One key cannot be replayed across kinds: a top-up key reused for a withdrawal
// of the same member and amount is a different submission.
func TestKeyedCreateBindsTheKind(t *testing.T) {
	ls, admin, player := keyedLedger(t)
	in := CashInput{RequestID: uuid.New(), UserID: player.ID, AmountCents: 5000, CreatedBy: admin.ID}
	if _, err := ls.RecordTopup(in); err != nil {
		t.Fatal(err)
	}
	_, err := ls.RecordWithdrawal(in)
	protocolErrorCode(t, err, "request_conflict")

	purchase := AssetPurchaseInput{RequestID: uuid.New(), AmountCents: 5000, Units: 12, CreatedBy: admin.ID}
	if _, err := ls.RecordShuttlePurchase(purchase); err != nil {
		t.Fatal(err)
	}
	_, err = ls.RecordCourtCreditPurchase(purchase)
	protocolErrorCode(t, err, "request_conflict")

	if balance, err := ls.BalanceOfUser(player.ID); err != nil || balance != 5000 {
		t.Errorf("balance = %d / %v, want 5000", balance, err)
	}
	stock, err := ls.StockPosition(nil)
	if err != nil || stock.Units != 12 || stock.ValueCents != 5000 {
		t.Errorf("stock = %+v / %v, want 12 shuttles worth 5000", stock, err)
	}
	assertBalanced(t)
}

// A keyed transaction is tied to its operation row, so the ledger itself
// records which submission produced it.
func TestKeyedCreateLinksTheTransactionToItsOperation(t *testing.T) {
	ls, admin, player := keyedLedger(t)
	in := CashInput{RequestID: uuid.New(), UserID: player.ID, AmountCents: 5000, CreatedBy: admin.ID}
	txn, err := ls.RecordWithdrawal(in)
	if err != nil {
		t.Fatal(err)
	}
	var stored models.Transaction
	if err := database.DB.First(&stored, "id = ?", txn.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.MoneyOperationID == nil || *stored.MoneyOperationID != in.RequestID {
		t.Errorf("money_operation_id = %v, want %s", stored.MoneyOperationID, in.RequestID)
	}
	var operation models.MoneyOperation
	if err := database.DB.First(&operation, "id = ?", in.RequestID).Error; err != nil {
		t.Fatal(err)
	}
	if operation.Action != models.MoneyOperationRecordWithdrawal || operation.ActorID != admin.ID || operation.TargetTransactionID != nil {
		t.Errorf("operation = %+v", operation)
	}
}

// Only an approved admin may hold a key, and a member who does not exist is
// refused before anything is reserved.
func TestKeyedCreateRefusesNonAdminsAndUnknownMembers(t *testing.T) {
	ls, admin, player := keyedLedger(t)
	id := uuid.New()
	_, err := ls.RecordTopup(CashInput{RequestID: id, UserID: player.ID, AmountCents: 5000, CreatedBy: player.ID})
	protocolErrorCode(t, err, "forbidden")
	if _, err := ls.RecordTopup(CashInput{RequestID: id, UserID: uuid.New(), AmountCents: 5000, CreatedBy: admin.ID}); err == nil {
		t.Error("a top-up for a member who does not exist was accepted")
	}
	if got := operationCount(t, id); got != 0 {
		t.Errorf("%d operations reserved by refused requests", got)
	}
	if got := transactionsOfKind(t, models.TxnPlayerTopup); got != 0 {
		t.Errorf("%d top-ups posted", got)
	}
}
