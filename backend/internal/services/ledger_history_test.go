package services

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/splitwise"
)

func TestSharedHistoryClassifiesSourceAndKeepsFullBalances(t *testing.T) {
	requireDB(t)
	const csv = `Date,Description,Category,Cost,Currency,Alice,Badminton Account,Bob,Former (removed)
2024-04-01,Top-Up,General,100.00,AUD,100.00,-100.00,0.00,0.00
2024-04-01,Top up Former,General,5.00,AUD,0.00,-5.00,0.00,5.00
2024-04-03,Game - 2 Apr,General,15.00,AUD,-5.00,15.00,-5.00,-5.00
2024-04-04,Game - 4 Apr,General,9.00,AUD,6.00,0.00,-6.00,0.00
2024-04-05,Alice paid Badminton A.,Payment,20.00,AUD,20.00,-20.00,0.00,0.00
2024-04-06,Bob paid Alice,Payment,1.00,AUD,-1.00,0.00,1.00,0.00
2024-04-07,Dinner,Dining out,6.00,AUD,3.00,0.00,-3.00,0.00
2024-04-08,Alice - Shuttles,General,10.00,AUD,10.00,-10.00,0.00,0.00
2024-04-09,Badminton A. paid Alice,Payment,2.00,AUD,-2.00,2.00,0.00,0.00
2024-04-09,Total balance, , ,AUD,131.00,-118.00,-13.00,0.00
`
	source, err := splitwise.Read(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	_, mapping := testImportSource(t)
	mapping.SessionPayers = map[string]string{"4": "Alice"}
	if _, err := ImportSplitwise(source, mapping, nil); err != nil {
		t.Fatal(err)
	}
	var alice models.User
	if err := database.DB.Where("email = ?", "alice@example.test").First(&alice).Error; err != nil {
		t.Fatal(err)
	}
	ls := NewLedgerService()
	all, total, err := ls.Entries(LedgerHistoryFilter{Limit: 200})
	if err != nil || total != 14 || len(all) != 14 {
		t.Fatalf("all %d/%d: %v", len(all), total, err)
	}
	expected := map[string]string{"Top-Up": "topup", "Top up Former": "topup", "Game - 2 Apr": "session", "Game - 4 Apr": "session", "Alice paid Badminton A.": "topup", "Bob paid Alice": "transfer", "Dinner": "food", "Alice - Shuttles": "shuttles", "Badminton A. paid Alice": "withdrawal"}
	byID := map[uuid.UUID]LedgerEntryView{}
	inactive := 0
	for _, e := range all {
		byID[e.ID] = e
		if e.Source != "splitwise" || e.Category != expected[e.Description] {
			t.Fatalf("wrong metadata %+v", e)
		}
		if e.Inactive {
			inactive++
			if e.UserID != nil || e.MemberName != "Former" {
				t.Fatalf("inactive %+v", e)
			}
		}
	}
	if inactive != 2 {
		t.Fatalf("inactive entries %d", inactive)
	}
	topups, total, err := ls.Entries(LedgerHistoryFilter{TopupsOnly: true, Limit: 200})
	if err != nil || total != 3 {
		t.Fatalf("topups %d: %v", total, err)
	}
	for _, e := range topups {
		if !reflect.DeepEqual(e, byID[e.ID]) {
			t.Fatalf("filter changed balance: %+v", e)
		}
	}
	mine, total, err := ls.Entries(LedgerHistoryFilter{UserID: &alice.ID, TopupsOnly: true, Limit: 1})
	if err != nil || total != 2 || len(mine) != 1 || mine[0].BalanceAfterCents != 12100 {
		t.Fatalf("mine %+v total %d err %v", mine, total, err)
	}
	second, _, err := ls.Entries(LedgerHistoryFilter{UserID: &alice.ID, TopupsOnly: true, Limit: 1, Offset: 1})
	if err != nil || len(second) != 1 || second[0].BalanceAfterCents != 10000 {
		t.Fatalf("second %+v: %v", second, err)
	}
}

func TestSharedHistoryBeyondFiftyAndEqualTimes(t *testing.T) {
	ls, alice, _ := newLedger(t)
	bob := newUser(t, "history-bob")
	when := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < 113; i++ {
		user := alice.ID
		if i%2 == 0 {
			user = bob.ID
		}
		if _, err := ls.RecordTopup(CashInput{UserID: user, AmountCents: 100, OccurredAt: when, CreatedBy: alice.ID}); err != nil {
			t.Fatal(err)
		}
	}
	all, total, err := ls.Entries(LedgerHistoryFilter{Limit: 200})
	if err != nil || total != 113 {
		t.Fatalf("all %d: %v", total, err)
	}
	var paged []LedgerEntryView
	for offset := 0; offset < 113; offset += 50 {
		page, total, err := ls.Entries(LedgerHistoryFilter{Limit: 50, Offset: offset})
		if err != nil || total != 113 {
			t.Fatalf("page %d: %v", total, err)
		}
		paged = append(paged, page...)
	}
	if !reflect.DeepEqual(all, paged) {
		t.Fatal("pages omit, reorder, or duplicate entries")
	}
	balances := map[uuid.UUID]int64{alice.ID: 5600, bob.ID: 5700}
	for _, e := range all {
		if e.BalanceAfterCents != balances[*e.UserID] || e.Category != "topup" || e.Source != "" {
			t.Fatalf("per-member running balance %+v", e)
		}
		balances[*e.UserID] -= e.AmountCents
	}
	if balances[alice.ID] != 0 || balances[bob.ID] != 0 {
		t.Fatal(balances)
	}
}

func TestAssetDatesDistinguishAuditFromMovements(t *testing.T) {
	ls, user, account := newLedger(t)
	position, err := ls.Position()
	if err != nil {
		t.Fatal(err)
	}
	if position.Assets.BankAsOf != nil || position.Assets.ShuttleAuditedOn != nil {
		t.Fatal("invented dates")
	}
	// 14:30 UTC is the next calendar date in Sydney during daylight saving.
	snapshot := time.Date(2026, 10, 4, 14, 30, 0, 0, time.UTC)
	if _, err := ls.RecordOpeningBalances(OpeningBalancesInput{BankCents: 20000, CourtCreditCents: 1000, ShuttleUnits: 12, ShuttleCents: 5000, OccurredAt: snapshot, CreatedBy: user.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := ls.RecordShuttlePurchase(AssetPurchaseInput{AmountCents: 5000, Units: 12, OccurredAt: snapshot.AddDate(0, 0, 1), CreatedBy: user.ID}); err != nil {
		t.Fatal(err)
	}
	units := -1
	if _, err := ls.Post(PostInput{Kind: models.TxnSessionSettlement, OccurredAt: snapshot.AddDate(0, 0, 2), CreatedBy: user.ID, Movements: []Movement{{AccountID: account, AmountCents: -400}, {AccountID: clubAccount(t, ls, models.AccountKindShuttleStock), AmountCents: -400, Units: &units}}}); err != nil {
		t.Fatal(err)
	}
	position, err = ls.Position()
	if err != nil {
		t.Fatal(err)
	}
	a := position.Assets
	if a.BankAsOf == nil || *a.BankAsOf != "2026-10-06" || a.CourtCreditAsOf == nil || *a.CourtCreditAsOf != "2026-10-05" || a.ShuttleAuditedOn == nil || *a.ShuttleAuditedOn != "2026-10-05" || a.ShuttleStockAsOf == nil || *a.ShuttleStockAsOf != "2026-10-07" {
		t.Fatalf("dates %+v", a)
	}
	if a.BankCents != 15000 || a.ShuttleStockCents != 9600 || a.ShuttleStockUnits != 23 || !position.Balanced {
		t.Fatalf("read changed position %+v", position)
	}
}

func TestImportAssetDateUsesCutoff(t *testing.T) {
	requireDB(t)
	source, mapping := testImportSource(t)
	if _, err := ImportSplitwise(source, mapping, &ImportAssets{BankCents: 4000, CourtCreditCents: 3000, ShuttleUnits: 12, ShuttleCents: 2000}); err != nil {
		t.Fatal(err)
	}
	ls := NewLedgerService()
	position, err := ls.Position()
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range []*string{position.Assets.BankAsOf, position.Assets.CourtCreditAsOf, position.Assets.ShuttleStockAsOf, position.Assets.ShuttleAuditedOn} {
		if d == nil || *d != "2024-04-04" {
			t.Fatalf("cutoff date %v", d)
		}
	}
}

func TestAssetDateOmitsReversedOpeningAudit(t *testing.T) {
	ls, user, _ := newLedger(t)
	txn, err := ls.RecordOpeningBalances(OpeningBalancesInput{BankCents: 1000, ShuttleUnits: 12, ShuttleCents: 5000, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ReverseTransaction(txn.ID, "test reversal", user.ID); err != nil {
		t.Fatal(err)
	}
	position, err := ls.Position()
	if err != nil {
		t.Fatal(err)
	}
	if position.Assets.ShuttleAuditedOn != nil {
		t.Fatal("reversed audit still used")
	}
}
