package services

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

// Hold the importer's actual table-lock mode while each financial entry point
// starts. A waiting writer must not already own application row locks: otherwise
// an importer needing that row forms the inverse table/account wait cycle.
func TestFinancialWritesWaitForImportBeforeRowLocks(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(*testing.T) func() error
	}{
		{"ledger_post", func(t *testing.T) func() error {
			ledger, actor, player := newLedger(t)
			bank := clubAccount(t, ledger, models.AccountKindBank)
			return func() error {
				_, err := ledger.Post(PostInput{Kind: models.TxnPlayerTopup, CreatedBy: actor.ID, Movements: []Movement{{AccountID: player, AmountCents: 100}, {AccountID: bank, AmountCents: 100}}})
				return err
			}
		}},
		{"caller_transaction_post", func(t *testing.T) func() error {
			ledger, actor, player := newLedger(t)
			bank := clubAccount(t, ledger, models.AccountKindBank)
			return func() error {
				return database.DB.Transaction(func(tx *gorm.DB) error {
					_, err := ledger.PostWithin(tx, PostInput{Kind: models.TxnPlayerTopup, CreatedBy: actor.ID, Movements: []Movement{{AccountID: player, AmountCents: 100}, {AccountID: bank, AmountCents: 100}}})
					return err
				})
			}
		}},
		{"transaction_reversal", func(t *testing.T) func() error {
			ledger, actor, _ := newLedger(t)
			original, err := ledger.RecordTopup(CashInput{UserID: actor.ID, CreatedBy: actor.ID, AmountCents: 100})
			if err != nil {
				t.Fatal(err)
			}
			return func() error {
				_, err := ledger.ReverseTransaction(original.ID, "Correct original", actor.ID)
				return err
			}
		}},
		{"session_settlement", func(t *testing.T) func() error {
			f := newSettlementFixture(t, 24, 10000)
			player := f.member(t, "player")
			return func() error {
				_, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}})
				return err
			}
		}},
		{"settlement_reversal", func(t *testing.T) func() error {
			f := newSettlementFixture(t, 24, 10000)
			original := reverseFixtureSession(t, f)
			return func() error {
				_, err := f.settlement.ReverseSettlement(original.ID, "Correct session", f.admin.ID)
				return err
			}
		}},
		{"generic_session_reversal", func(t *testing.T) func() error {
			f := newSettlementFixture(t, 24, 10000)
			original := reverseFixtureSession(t, f)
			return func() error {
				_, err := f.ledger.ReverseTransaction(original.TransactionID, "Correct session", f.admin.ID)
				return err
			}
		}},
		{"asset_purchase_correction", func(t *testing.T) func() error {
			ledger, _, input := assetCorrectionFixture(t, true)
			return func() error { _, err := ledger.CorrectAssetPurchase(input); return err }
		}},
		{"asset_balance_audit", func(t *testing.T) func() error {
			ledger, actor, _ := newLedger(t)
			if err := database.DB.Model(&actor).Update("role", models.RoleAdmin).Error; err != nil {
				t.Fatal(err)
			}
			return func() error {
				_, err := ledger.CorrectAssetBalance(BalanceCorrectionInput{RequestID: uuid.New(), AccountKind: models.AccountKindBank, ExpectedCents: 0, TargetCents: 100, Reason: "Verified bank", CreatedBy: actor.ID})
				return err
			}
		}},
		{"reviewed_operation_runner", func(t *testing.T) func() error {
			request, ledger, player, bank := moneyProtocolFixture(t)
			return func() error {
				_, err := runMoneyOperation(request, protocolPosting(request, ledger, player, bank))
				return err
			}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			write := tc.prepare(t)
			gate := database.DB.Begin()
			if gate.Error != nil {
				t.Fatal(gate.Error)
			}
			done := make(chan error, 1)
			finished := false
			t.Cleanup(func() {
				gate.Rollback()
				if !finished {
					select {
					case <-done:
					case <-time.After(5 * time.Second):
						t.Error("financial writer did not stop after releasing import gate")
					}
				}
			})
			if err := gate.Exec("SET LOCAL idle_in_transaction_session_timeout = '20s'").Error; err != nil {
				t.Fatal(err)
			}
			if err := gate.Exec("LOCK TABLE transactions IN SHARE ROW EXCLUSIVE MODE").Error; err != nil {
				t.Fatal(err)
			}
			go func() { done <- write() }()

			// Observe the actual lock wait, rather than guessing timing with a sleep.
			// The target relation is scoped to this package's isolated schema.
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for {
				var waiting bool
				err := database.DB.Raw(`SELECT EXISTS (SELECT 1 FROM pg_locks
    WHERE locktype = 'relation' AND relation = 'transactions'::regclass
    AND database = (SELECT oid FROM pg_database WHERE datname = current_database())
    AND mode = 'RowExclusiveLock' AND NOT granted)`).Scan(&waiting).Error
				if err != nil {
					t.Fatal(err)
				}
				if waiting {
					break
				}
				select {
				case err := <-done:
					finished = true
					t.Fatalf("writer returned before waiting at import gate: %v", err)
				case <-deadline.C:
					t.Fatal("writer never reached the financial table-lock wait")
				case <-ticker.C:
				}
			}

			// Row-share table locks are compatible with the importer's table mode; NOWAIT
			// therefore detects prematurely held *rows* rather than blocking on the gate.
			probeErr := database.DB.Transaction(func(tx *gorm.DB) error {
				if err := tx.Exec("SET LOCAL lock_timeout = '1s'").Error; err != nil {
					return err
				}
				for _, table := range []string{"users", "sessions", "settlements", "transactions", "accounts"} {
					var rows []struct{ ID uuid.UUID }
					if err := tx.Raw("SELECT id FROM " + table + " FOR UPDATE NOWAIT").Scan(&rows).Error; err != nil {
						return fmt.Errorf("%s row locked before import gate: %w", table, err)
					}
				}
				return nil
			})
			if err := gate.Rollback().Error; err != nil {
				t.Fatal(err)
			}
			select {
			case err := <-done:
				finished = true
				if err != nil {
					t.Fatalf("financial write failed after gate released: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("writer did not finish after import gate released")
			}
			if probeErr != nil {
				t.Error(probeErr)
			}
			assertBalanced(t)
		})
	}
}
