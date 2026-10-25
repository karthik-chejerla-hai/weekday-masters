package services

import (
	"reflect"
	"sync"
	"testing"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func assetCorrectionFixture(t *testing.T, shuttle bool) (*LedgerService, *models.Transaction, AssetPurchaseCorrectionInput) {
	t.Helper()
	ls, actor, _ := newLedger(t)
	if err := database.DB.Model(&actor).Update("role", models.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	input := AssetPurchaseInput{AmountCents: 5000, Units: 12, Description: "Original purchase", CreatedBy: actor.ID}
	var original *models.Transaction
	var err error
	if shuttle {
		original, err = ls.RecordShuttlePurchase(input)
	} else {
		original, err = ls.RecordCourtCreditPurchase(input)
	}
	if err != nil {
		t.Fatal(err)
	}
	in := AssetPurchaseCorrectionInput{TransactionID: original.ID, AmountCents: 6000, Reason: "Receipt checked", CreatedBy: actor.ID}
	if shuttle {
		in.Units = 24
	}
	return ls, original, in
}

func TestAssetPurchaseCorrectionAuditAndRetry(t *testing.T) {
	for _, shuttle := range []bool{false, true} {
		t.Run(map[bool]string{true: "shuttles", false: "court"}[shuttle], func(t *testing.T) {
			ls, original, in := assetCorrectionFixture(t, shuttle)
			var before models.Transaction
			database.DB.Preload("Entries").First(&before, "id = ?", original.ID)
			corrected, err := ls.CorrectAssetPurchase(in)
			if err != nil {
				t.Fatal(err)
			}
			retry, err := ls.CorrectAssetPurchase(in)
			if err != nil || retry.ID != corrected.ID {
				t.Fatalf("unsafe retry: %v %v", retry, err)
			}
			if corrected.CorrectsTransactionID == nil || *corrected.CorrectsTransactionID != original.ID || corrected.CorrectionReason != in.Reason || corrected.CreatedBy != in.CreatedBy || !corrected.OccurredAt.Equal(original.OccurredAt) {
				t.Fatalf("missing audit: %+v", corrected)
			}
			var after models.Transaction
			database.DB.Preload("Entries").First(&after, "id = ?", original.ID)
			if !reflect.DeepEqual(before, after) {
				t.Fatal("original changed")
			}
			var reversal models.Transaction
			if err := database.DB.Preload("Entries").First(&reversal, "reverses_transaction_id = ?", original.ID).Error; err != nil {
				t.Fatal(err)
			}
			for _, e := range before.Entries {
				matched := false
				for _, r := range reversal.Entries {
					if r.AccountID == e.AccountID && r.AmountCents == -e.AmountCents && ((r.Units == nil && e.Units == nil) || (r.Units != nil && e.Units != nil && *r.Units == -*e.Units)) {
						matched = true
					}
				}
				if !matched {
					t.Fatal("reversal is not the exact negation")
				}
			}
			position, err := ls.Position()
			if err != nil {
				t.Fatal(err)
			}
			if !position.Balanced || position.Assets.BankCents != -6000 || position.Liabilities.PlayerBalancesCents != 0 || position.SurplusCents != 0 {
				t.Fatalf("bad position: %+v", position)
			}
			if shuttle && (position.Assets.ShuttleStockCents != 6000 || position.Assets.ShuttleStockUnits != 24) {
				t.Fatal("bad stock")
			}
			if !shuttle && position.Assets.CourtCreditCents != 6000 {
				t.Fatal("bad court balance")
			}
			items, total, err := ls.AssetPurchases(1, 0)
			if err != nil || total != 2 || len(items) != 1 || items[0].ID != corrected.ID {
				t.Fatalf("bad page: %v %d %v", items, total, err)
			}
			items, _, err = ls.AssetPurchases(1, 1)
			if err != nil || !items[0].Reversed {
				t.Fatal("original not marked reversed")
			}
			conflict := in
			conflict.AmountCents++
			if _, err := ls.CorrectAssetPurchase(conflict); err == nil {
				t.Fatal("second correction accepted")
			}
			in.TransactionID = corrected.ID
			in.AmountCents = 7000
			if _, err := ls.CorrectAssetPurchase(in); err != nil {
				t.Fatalf("replacement cannot be corrected: %v", err)
			}
		})
	}
}

func TestAssetPurchaseCorrectionRejectsConsumedStockAtomically(t *testing.T) {
	ls, _, in := assetCorrectionFixture(t, true)
	stock := clubAccount(t, ls, models.AccountKindShuttleStock)
	surplus := clubAccount(t, ls, models.AccountKindSurplus)
	used := -10
	if _, err := ls.Post(PostInput{Kind: models.TxnSessionSettlement, Movements: []Movement{{AccountID: stock, AmountCents: -4000, Units: &used}, {AccountID: surplus, AmountCents: -4000}}}); err != nil {
		t.Fatal(err)
	}
	entries, txns := countEntries(t)
	for _, pair := range []struct {
		cents int64
		units int
	}{{1000, 24}, {6000, 1}, {6000, 10}} {
		in.AmountCents, in.Units = pair.cents, pair.units
		if _, err := ls.CorrectAssetPurchase(in); err == nil {
			t.Fatalf("invalid stock accepted: %+v", pair)
		}
		e, n := countEntries(t)
		if entries != e || txns != n {
			t.Fatal("failed correction left ledger entries")
		}
	}
	in.AmountCents, in.Units = 4500, 11
	if _, err := ls.CorrectAssetPurchase(in); err != nil {
		t.Fatal(err)
	}
	stockPosition, err := ls.StockPosition(nil)
	if err != nil || stockPosition.ValueCents != 500 || stockPosition.Units != 1 {
		t.Fatalf("unexpected stock: %+v %v", stockPosition, err)
	}
}

func TestAssetPurchaseCorrectionValidation(t *testing.T) {
	ls, original, in := assetCorrectionFixture(t, true)
	for _, change := range []func(*AssetPurchaseCorrectionInput){
		func(i *AssetPurchaseCorrectionInput) { i.AmountCents = 0 },
		func(i *AssetPurchaseCorrectionInput) { i.AmountCents = -1 },
		func(i *AssetPurchaseCorrectionInput) { i.Units = 0 },
		func(i *AssetPurchaseCorrectionInput) { i.Units = -1 },
		func(i *AssetPurchaseCorrectionInput) { i.Reason = " " },
		func(i *AssetPurchaseCorrectionInput) { i.AmountCents = 5000; i.Units = 12 },
	} {
		bad := in
		change(&bad)
		if _, err := ls.CorrectAssetPurchase(bad); err == nil {
			t.Fatalf("invalid input accepted: %+v", bad)
		}
	}
	player := newUser(t, "nonadmin")
	bad := in
	bad.CreatedBy = player.ID
	if _, err := ls.CorrectAssetPurchase(bad); err == nil {
		t.Fatal("nonadmin accepted")
	}
	if _, err := ls.ReverseTransaction(original.ID, "undo", in.CreatedBy); err != nil {
		t.Fatal(err)
	}
	if _, err := ls.CorrectAssetPurchase(in); err == nil {
		t.Fatal("reversed purchase corrected")
	}
}

func TestAssetPurchaseCorrectionConcurrency(t *testing.T) {
	for _, same := range []bool{false, true} {
		t.Run(map[bool]string{true: "retries", false: "competing"}[same], func(t *testing.T) {
			ls, _, in := assetCorrectionFixture(t, true)
			var wg sync.WaitGroup
			errs := make(chan error, 8)
			start := make(chan struct{})
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func(n int) {
					defer wg.Done()
					<-start
					input := in
					if !same {
						input.AmountCents += int64(n)
					}
					_, err := ls.CorrectAssetPurchase(input)
					errs <- err
				}(i)
			}
			close(start)
			wg.Wait()
			close(errs)
			success := 0
			for err := range errs {
				if err == nil {
					success++
				} else if e, ok := AsLedgerError(err); !ok || e.Status != 409 {
					t.Fatalf("unexpected error: %v", err)
				}
			}
			expected := 1
			if same {
				expected = 8
			}
			if success != expected {
				t.Fatalf("%d successes; want %d", success, expected)
			}
			e, n := countEntries(t)
			if e != 6 || n != 3 {
				t.Fatalf("duplicate posts: %d entries, %d transactions", e, n)
			}
			position, err := ls.Position()
			if err != nil || !position.Balanced {
				t.Fatalf("invariant: %v %v", position, err)
			}
		})
	}
}

func TestAssetPurchaseCorrectionVersusReversal(t *testing.T) {
	ls, original, in := assetCorrectionFixture(t, true)
	start := make(chan struct{})
	errs := make(chan error, 2)
	go func() { <-start; _, err := ls.CorrectAssetPurchase(in); errs <- err }()
	go func() { <-start; _, err := ls.ReverseTransaction(original.ID, "undo", in.CreatedBy); errs <- err }()
	close(start)
	first, second := <-errs, <-errs
	if (first == nil) == (second == nil) {
		t.Fatalf("expected one success: %v %v", first, second)
	}
}
