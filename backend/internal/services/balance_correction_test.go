package services

import (
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func correctionFixture(t *testing.T) (*LedgerService, BalanceCorrectionInput) {
	t.Helper()
	ls, actor, _ := newLedger(t)
	if err := database.DB.Model(&actor).Update("role", "admin").Error; err != nil {
		t.Fatal(err)
	}
	court := clubAccount(t, ls, models.AccountKindCourtCredit)
	surplus := clubAccount(t, ls, models.AccountKindSurplus)
	if _, err := ls.Post(PostInput{Kind: models.TxnOpeningBalance, CreatedBy: actor.ID, Movements: []Movement{{AccountID: court, AmountCents: 5300}, {AccountID: surplus, AmountCents: 5300}}}); err != nil {
		t.Fatal(err)
	}
	return ls, BalanceCorrectionInput{RequestID: uuid.New(), AccountKind: models.AccountKindCourtCredit, ExpectedCents: 5300, TargetCents: 11300, Reason: "Verified venue statement", CreatedBy: actor.ID}
}

func TestBalanceCorrectionAuditAndRetry(t *testing.T) {
	ls, in := correctionFixture(t)
	before, err := ls.Position()
	if err != nil {
		t.Fatal(err)
	}
	record, err := ls.CorrectAssetBalance(in)
	if err != nil {
		t.Fatal(err)
	}
	retry, err := ls.CorrectAssetBalance(in)
	if err != nil {
		t.Fatal(err)
	}
	if retry.TransactionID != record.TransactionID {
		t.Fatal("retry posted a second correction")
	}
	after, err := ls.Position()
	if err != nil {
		t.Fatal(err)
	}
	if after.Assets.CourtCreditCents != 11300 || after.SurplusCents-before.SurplusCents != 6000 || !after.Balanced {
		t.Fatalf("incorrect position: %+v", after)
	}
	if before.Assets.BankCents != after.Assets.BankCents || before.Assets.ShuttleStockCents != after.Assets.ShuttleStockCents || before.Liabilities.PlayerBalancesCents != after.Liabilities.PlayerBalancesCents {
		t.Fatal("unrelated balances moved")
	}
	items, total, err := ls.ListBalanceCorrections(20, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].BeforeCents != 5300 || items[0].AfterCents != 11300 || items[0].Reason != in.Reason || items[0].CreatedBy != in.CreatedBy || items[0].ActorName == "" || items[0].CreatedAt.IsZero() {
		t.Fatalf("audit incomplete: %+v", items)
	}
	if _, err := ls.ReverseTransaction(record.TransactionID, "undo", in.CreatedBy); err == nil {
		t.Fatal("snapshot reversal accepted")
	}
	in.RequestID = uuid.New()
	in.ExpectedCents = 11300
	in.TargetCents = 5300
	in.Reason = "Second verified correction"
	if _, err := ls.CorrectAssetBalance(in); err != nil {
		t.Fatal(err)
	}
	value, err := ls.BalanceOfKind(nil, models.AccountKindCourtCredit)
	if err != nil || value != 5300 {
		t.Fatalf("downward correction: %d %v", value, err)
	}
}

func TestBalanceCorrectionRejectsInvalidAndStale(t *testing.T) {
	ls, original := correctionFixture(t)
	for _, change := range []func(*BalanceCorrectionInput){
		func(in *BalanceCorrectionInput) { in.ExpectedCents = 0 },
		func(in *BalanceCorrectionInput) { in.TargetCents = -1 },
		func(in *BalanceCorrectionInput) { in.TargetCents = 5300 },
		func(in *BalanceCorrectionInput) { in.Reason = "   " },
		func(in *BalanceCorrectionInput) { in.AccountKind = models.AccountKindPlayer },
		func(in *BalanceCorrectionInput) { in.RequestID = uuid.Nil },
	} {
		in := original
		change(&in)
		if _, err := ls.CorrectAssetBalance(in); err == nil {
			t.Fatalf("invalid correction accepted: %+v", in)
		}
	}
	if _, err := ls.CorrectAssetBalance(original); err != nil {
		t.Fatal(err)
	}
	original.TargetCents = 11400
	if _, err := ls.CorrectAssetBalance(original); err == nil {
		t.Fatal("conflicting retry accepted")
	}
	var count int64
	database.DB.Model(&models.BalanceCorrection{}).Count(&count)
	if count != 1 {
		t.Fatalf("rejected inputs wrote audit rows: %d", count)
	}
}

func TestBalanceCorrectionConcurrentRetriesPostOnce(t *testing.T) {
	ls, in := correctionFixture(t)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := ls.CorrectAssetBalance(in); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var count int64
	database.DB.Model(&models.BalanceCorrection{}).Count(&count)
	if count != 1 {
		t.Fatalf("got %d corrections", count)
	}
}
