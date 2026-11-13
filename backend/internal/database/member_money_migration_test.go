package database_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/testsupport"
	"gorm.io/gorm"
)

func TestMain(m *testing.M) {
	testsupport.Setup("database_test")
	os.Exit(m.Run())
}

func requireDB(t *testing.T) {
	t.Helper()
	testsupport.RequireDB(t)
}

func moneyFixture(t *testing.T) (models.User, *models.Transaction) {
	t.Helper()
	actor := models.User{Auth0ID: "migration|" + uuid.NewString(), Email: uuid.NewString() + "@seed.invalid", Name: "Migration admin", Role: models.RoleAdmin, MembershipStatus: models.MembershipApproved}
	if err := database.DB.Create(&actor).Error; err != nil {
		t.Fatal(err)
	}
	original, err := services.NewLedgerService().RecordTopup(services.CashInput{UserID: actor.ID, AmountCents: 5000, CreatedBy: actor.ID, Description: "Original top-up", OccurredAt: time.Date(2026, 1, 15, 8, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatal(err)
	}
	return actor, original
}

func insertOperation(tx *gorm.DB, id, actor, target uuid.UUID) error {
	return tx.Exec(`INSERT INTO money_operations
		(id, actor_id, action, target_transaction_id, payload_hash, canonical_payload, created_at)
		VALUES (?, ?, 'reverse_transaction', ?, ?, '{}'::jsonb, clock_timestamp())`,
		id, actor, target, strings.Repeat("a", 64)).Error
}

func completeOperation(tx *gorm.DB, id uuid.UUID) error {
	return tx.Exec(`UPDATE money_operations SET response_status = 201, response_body = '{"operation_id":"accepted"}'::jsonb WHERE id = ?`, id).Error
}

func TestMemberMoneyMigrationPreservesPopulatedLegacyLedger(t *testing.T) {
	requireDB(t)
	_, original := moneyFixture(t)
	var before, after string
	const snapshot = `SELECT jsonb_build_object(
		'transaction', (SELECT to_jsonb(t) - 'money_operation_id' - 'operation_step' FROM transactions t WHERE id = ?),
		'entries', (SELECT jsonb_agg(to_jsonb(e) ORDER BY e.id) FROM ledger_entries e WHERE transaction_id = ?))::text`
	if err := database.DB.Raw(snapshot, original.ID, original.ID).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	// Remove only the new schema from this disposable, package-isolated DB.
	// The retained user/top-up/entries are an actual pre-feature upgrade fixture.
	for _, sql := range []string{
		`DROP TABLE IF EXISTS money_operations CASCADE`,
		`ALTER TABLE transactions DROP COLUMN IF EXISTS money_operation_id, DROP COLUMN IF EXISTS operation_step`,
		`DROP SEQUENCE IF EXISTS money_operations_sequence`,
	} {
		if err := database.DB.Exec(sql).Error; err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() {
		if err := database.Migrate(); err != nil {
			t.Errorf("restore migration: %v", err)
		}
	})
	for i := 0; i < 2; i++ {
		if err := database.Migrate(); err != nil {
			t.Fatalf("migration run %d: %v", i+1, err)
		}
	}
	if !database.DB.Migrator().HasTable("money_operations") {
		t.Fatal("migration did not create money_operations")
	}
	if err := database.DB.Raw(snapshot, original.ID, original.ID).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Fatalf("upgrade changed legacy financial records:\nbefore %s\nafter  %s", before, after)
	}
}

func TestMemberMoneyReservationTargetIsDeferred(t *testing.T) {
	requireDB(t)
	actor, _ := moneyFixture(t)
	id, target := uuid.New(), uuid.New()
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// The reservation precedes the target lock/insertion. An immediate FK
		// would fail here (and would take a premature KEY SHARE on real targets).
		if err := insertOperation(tx, id, actor.ID, target); err != nil {
			return err
		}
		if err := tx.Create(&models.Transaction{ID: target, Kind: models.TxnPlayerTopup, Description: "FK fixture", CreatedBy: actor.ID}).Error; err != nil {
			return err
		}
		return completeOperation(tx, id)
	})
	if err != nil {
		t.Fatalf("target FK was not deferred until commit: %v", err)
	}
	var deferred bool
	if err := database.DB.Raw(`SELECT condeferrable AND condeferred FROM pg_constraint WHERE conrelid = 'money_operations'::regclass AND conname = 'fk_money_operations_target_transaction'`).Scan(&deferred).Error; err != nil || !deferred {
		t.Fatalf("target FK deferral metadata: deferred=%v err=%v", deferred, err)
	}
	missing := uuid.New()
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := insertOperation(tx, missing, actor.ID, uuid.New()); err != nil {
			return err
		}
		return completeOperation(tx, missing)
	}); err == nil {
		t.Fatal("missing target was allowed at commit")
	}
}

func TestMemberMoneyCompetingReservationsDoNotLockOriginal(t *testing.T) {
	requireDB(t)
	actor, original := moneyFixture(t)
	first, second := database.DB.Begin(), database.DB.Begin()
	if first.Error != nil || second.Error != nil {
		t.Fatalf("begin competing transactions: %v / %v", first.Error, second.Error)
	}
	t.Cleanup(func() { first.Rollback(); second.Rollback() })
	for _, tx := range []*gorm.DB{first, second} {
		if err := insertOperation(tx, uuid.New(), actor.ID, original.ID); err != nil {
			t.Fatal(err)
		}
	}
	// Both reservations exist before either original lock. An immediate FK
	// would leave each holding KEY SHARE and block this upgrade on the other.
	if err := first.Exec(`SET LOCAL lock_timeout = '1s'`).Error; err != nil {
		t.Fatal(err)
	}
	var locked struct{ ID uuid.UUID }
	if err := first.Raw(`SELECT id FROM transactions WHERE id = ? FOR UPDATE`, original.ID).Scan(&locked).Error; err != nil {
		t.Fatalf("reservation acquired a premature original lock: %v", err)
	}
	if locked.ID != original.ID {
		t.Fatal("original transaction disappeared")
	}
}

func TestMemberMoneyModelGeneratesIdentityAndSequence(t *testing.T) {
	requireDB(t)
	actor, original := moneyFixture(t)
	status, body := int16(201), `{"accepted":true}`
	var previous int64
	for i := 0; i < 2; i++ {
		op := models.MoneyOperation{
			ActorID: actor.ID, Action: models.MoneyOperationReverseTransaction,
			TargetTransactionID: &original.ID, PayloadHash: strings.Repeat("b", 64),
			CanonicalPayload: `{"reason":"test"}`, ResponseStatus: &status, ResponseBody: &body,
		}
		if err := database.DB.Create(&op).Error; err != nil {
			t.Fatal(err)
		}
		var stored models.MoneyOperation
		if err := database.DB.First(&stored, "id = ?", op.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.ID == uuid.Nil || stored.Sequence <= previous || stored.CreatedAt.IsZero() || stored.ResponseStatus == nil || *stored.ResponseStatus != 201 || stored.ResponseBody == nil {
			t.Fatalf("identity/sequence/replay snapshot not retained: id=%s sequence=%d", stored.ID, stored.Sequence)
		}
		previous = stored.Sequence
		if err := insertOperation(database.DB, stored.ID, actor.ID, original.ID); err == nil {
			t.Fatal("duplicate durable submission ID was accepted")
		}
	}
}

func TestMemberMoneyOperationMustCompleteAndRemainImmutable(t *testing.T) {
	requireDB(t)
	actor, original := moneyFixture(t)
	pending := uuid.New()
	if err := database.DB.Transaction(func(tx *gorm.DB) error { return insertOperation(tx, pending, actor.ID, original.ID) }); err == nil {
		t.Fatal("an incomplete reservation committed")
	}
	var count int64
	if err := database.DB.Table("money_operations").Where("id = ?", pending).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("failed reservation survived: count=%d err=%v", count, err)
	}
	id := uuid.New()
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := insertOperation(tx, id, actor.ID, original.ID); err != nil {
			return err
		}
		return completeOperation(tx, id)
	}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		`UPDATE money_operations SET canonical_payload = '{"changed":true}'::jsonb WHERE id = ?`,
		`UPDATE money_operations SET response_body = '{}'::jsonb WHERE id = ?`,
		`DELETE FROM money_operations WHERE id = ?`,
	} {
		if err := database.DB.Exec(sql, id).Error; err == nil {
			t.Fatalf("completed operation was mutable: %s", sql)
		}
	}
}

func TestMemberMoneyTransactionLinksAndOperationValidation(t *testing.T) {
	requireDB(t)
	actor, original := moneyFixture(t)
	operationID := uuid.New()
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := insertOperation(tx, operationID, actor.ID, original.ID); err != nil {
			return err
		}
		return completeOperation(tx, operationID)
	}); err != nil {
		t.Fatal(err)
	}
	insertLink := func(op any, step any) error {
		return database.DB.Exec(`INSERT INTO transactions (id, kind, occurred_at, created_at, money_operation_id, operation_step) VALUES (?, 'reversal', clock_timestamp(), clock_timestamp(), ?, ?)`, uuid.New(), op, step).Error
	}
	if err := insertLink(operationID, 0); err != nil {
		t.Fatal(err)
	}
	if err := insertLink(operationID, 1); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ op, step any }{
		{operationID, 1}, {operationID, nil}, {nil, 0}, {uuid.New(), 0}, {operationID, 2}, {operationID, -1},
	} {
		if err := insertLink(tc.op, tc.step); err == nil {
			t.Errorf("invalid/duplicate operation link accepted: %+v", tc)
		}
	}
	for _, change := range []struct {
		field string
		value any
	}{
		{"action", "unknown"}, {"payload_hash", "bad"}, {"canonical_payload", "[]"},
		{"actor_id", uuid.New()}, {"target_transaction_id", nil},
		{"response_status", 500}, {"response_body", "[]"}, {"response_body", nil},
	} {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			id := uuid.New()
			if err := insertOperation(tx, id, actor.ID, original.ID); err != nil {
				return err
			}
			values := map[string]any{"response_status": 201, "response_body": "{}"}
			values[change.field] = change.value
			return tx.Table("money_operations").Where("id = ?", id).Updates(values).Error
		})
		if err == nil {
			t.Errorf("invalid operation accepted: %s=%v", change.field, change.value)
		}
	}
}
