package services

import (
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

type reversalAccountSnapshot struct {
	ID         uuid.UUID
	ValueCents int64
	Units      int64
}

func reversalAccounts(t *testing.T) []reversalAccountSnapshot {
	t.Helper()
	var result []reversalAccountSnapshot
	if err := database.DB.Raw(`SELECT a.id, COALESCE(SUM(e.amount_cents), 0) AS value_cents,
		COALESCE(SUM(e.units), 0) AS units FROM accounts a
		LEFT JOIN ledger_entries e ON e.account_id = a.id GROUP BY a.id ORDER BY a.id`).Scan(&result).Error; err != nil {
		t.Fatal(err)
	}
	return result
}

func reverseFixtureSession(t *testing.T, f *settlementFixture) *models.Settlement {
	t.Helper()
	player := f.member(t, "player")
	comped := f.member(t, "comped")
	record, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}, {UserID: comped.ID, InBase: true, Comped: true}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return record
}

// Force a real database error after the inverse ledger entries have been posted.
// Both entry points must roll the entries back together with settlement status.
func TestSessionReversalRollsBackWhenStatusUpdateFails(t *testing.T) {
	for _, generic := range []bool{false, true} {
		name := "settlement endpoint"
		if generic {
			name = "transaction endpoint"
		}
		t.Run(name, func(t *testing.T) {
			f := newSettlementFixture(t, 24, 10000)
			record := reverseFixtureSession(t, f)
			before := reversalAccounts(t)
			entries, transactions := countEntries(t)
			if err := database.DB.Exec(`CREATE FUNCTION reject_reversal_status_update() RETURNS trigger
				LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'injected reversal status failure'; END $$`).Error; err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := database.DB.Exec("DROP FUNCTION reject_reversal_status_update() CASCADE").Error; err != nil {
					t.Error(err)
				}
			})
			if err := database.DB.Exec(`CREATE TRIGGER reject_reversal_status_update BEFORE UPDATE OF reversed_at
				ON settlements FOR EACH ROW EXECUTE FUNCTION reject_reversal_status_update()`).Error; err != nil {
				t.Fatal(err)
			}
			reverse := func() (*models.Transaction, error) {
				if generic {
					return f.ledger.ReverseTransaction(record.TransactionID, "correction", f.admin.ID)
				}
				return f.settlement.ReverseSettlement(record.ID, "correction", f.admin.ID)
			}
			result, err := reverse()
			if err == nil || !strings.Contains(err.Error(), "injected reversal status failure") || result != nil {
				t.Fatalf("expected rolled-back result, got %v / %v", result, err)
			}
			if after := reversalAccounts(t); !reflect.DeepEqual(after, before) {
				t.Fatalf("failed reversal changed accounts: before %+v after %+v", before, after)
			}
			if e, tr := countEntries(t); e != entries || tr != transactions {
				t.Fatalf("failed reversal left entries/transactions: %d/%d, want %d/%d", e, tr, entries, transactions)
			}
			live, err := f.settlement.LiveSettlementForSession(f.session.ID)
			if err != nil || live == nil || live.ID != record.ID || live.ReversedBy != nil {
				t.Fatalf("failed reversal changed live settlement: %+v / %v", live, err)
			}
			assertBalanced(t)

			if err := database.DB.Exec("DROP TRIGGER reject_reversal_status_update ON settlements").Error; err != nil {
				t.Fatal(err)
			}
			if _, err := reverse(); err != nil {
				t.Fatalf("retry after rollback: %v", err)
			}
			assertBalanced(t)
		})
	}
}

func TestSessionReversalMixedEntryPointsAreSerialized(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	// Create every participant before taking the baseline so zero accounts match.
	player := f.member(t, "player")
	comped := f.member(t, "comped")
	before := reversalAccounts(t)
	record, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}, {UserID: comped.ID, InBase: true, Comped: true}}})
	if err != nil {
		t.Fatal(err)
	}
	var original models.Transaction
	if err := database.DB.Preload("Entries").First(&original, "id = ?", record.TransactionID).Error; err != nil {
		t.Fatal(err)
	}

	const attempts = 8
	start := make(chan struct{})
	results := make([]error, attempts)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				_, results[i] = f.ledger.ReverseTransaction(record.TransactionID, "correction", f.admin.ID)
			} else {
				_, results[i] = f.settlement.ReverseSettlement(record.ID, "correction", f.admin.ID)
			}
		}(i)
	}
	close(start)
	wg.Wait()
	successes := 0
	for _, err := range results {
		if err == nil {
			successes++
		} else if e, ok := AsLedgerError(err); !ok || e.Code != CodeTransactionAlreadyReverse {
			t.Fatalf("unexpected concurrent failure: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("got %d successful reversals, want 1", successes)
	}
	if after := reversalAccounts(t); !reflect.DeepEqual(before, after) {
		t.Fatalf("reversal did not restore all assets, members and surplus: before %+v after %+v", before, after)
	}
	if err := database.DB.First(record, "id = ?", record.ID).Error; err != nil {
		t.Fatal(err)
	}
	if record.ReversedAt == nil || record.ReversedBy == nil || *record.ReversedBy != f.admin.ID {
		t.Fatalf("missing reversal metadata: %+v", record)
	}
	var inverses int64
	if err := database.DB.Model(&models.Transaction{}).Where("reverses_transaction_id = ?", original.ID).Count(&inverses).Error; err != nil || inverses != 1 {
		t.Fatalf("inverse count = %d: %v", inverses, err)
	}
	var unchanged models.Transaction
	if err := database.DB.Preload("Entries").First(&unchanged, "id = ?", original.ID).Error; err != nil {
		t.Fatal(err)
	}
	// Preload order is unspecified; compare each immutable entry by its ID.
	byID := map[uuid.UUID]models.LedgerEntry{}
	for _, entry := range original.Entries {
		byID[entry.ID] = entry
	}
	if len(unchanged.Entries) != len(byID) {
		t.Fatal("original entry count changed")
	}
	for _, entry := range unchanged.Entries {
		if !reflect.DeepEqual(entry, byID[entry.ID]) {
			t.Fatal("original entry was modified")
		}
	}
	assertBalanced(t)
	if _, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}}}); err != nil {
		t.Fatalf("cannot settle after reversal: %v", err)
	}
	assertBalanced(t)
}

func TestTransactionReversalCannotBeReversed(t *testing.T) {
	ls, user, _ := newLedger(t)
	original, err := ls.RecordTopup(CashInput{UserID: user.ID, AmountCents: 5000, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	inverse, err := ls.ReverseTransaction(original.ID, "wrong amount", user.ID)
	if err != nil {
		t.Fatal(err)
	}
	before := reversalAccounts(t)
	entries, transactions := countEntries(t)
	result, err := ls.ReverseTransaction(inverse.ID, "undo the undo", user.ID)
	if e, ok := AsLedgerError(err); !ok || e.Code != CodeReversalNotAllowed || e.Status != 422 || result != nil {
		t.Fatalf("got %v / %v, want reversal_not_allowed", result, err)
	}
	if after := reversalAccounts(t); !reflect.DeepEqual(before, after) {
		t.Fatal("reversal of reversal changed balances")
	}
	if e, tr := countEntries(t); e != entries || tr != transactions {
		t.Fatal("reversal of reversal wrote financial records")
	}
	assertBalanced(t)
}

func TestSessionReversalAndResettlementAreSerialized(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}}}
	before := reversalAccounts(t)
	record, _, err := f.settlement.Settle(input)
	if err != nil {
		t.Fatal(err)
	}
	settled := reversalAccounts(t)
	start := make(chan struct{})
	var reverseErr, settleErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, reverseErr = f.ledger.ReverseTransaction(record.TransactionID, "correction", f.admin.ID)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, _, settleErr = f.settlement.Settle(input)
	}()
	close(start)
	wg.Wait()
	if reverseErr != nil {
		t.Fatal(reverseErr)
	}
	want, wantLive := settled, int64(1)
	if settleErr != nil {
		if e, ok := AsLedgerError(settleErr); !ok || e.Code != CodeSessionAlreadySettled {
			t.Fatalf("unexpected settlement error: %v", settleErr)
		}
		want, wantLive = before, 0
	}
	var live int64
	if err := database.DB.Model(&models.Settlement{}).Where("session_id = ? AND reversed_at IS NULL", f.session.ID).Count(&live).Error; err != nil || live != wantLive {
		t.Fatalf("live settlements = %d, want %d: %v", live, wantLive, err)
	}
	if after := reversalAccounts(t); !reflect.DeepEqual(want, after) {
		t.Fatalf("race left wrong balances: want %+v got %+v", want, after)
	}
	assertBalanced(t)
}
