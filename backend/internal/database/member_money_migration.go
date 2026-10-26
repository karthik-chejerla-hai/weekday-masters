package database

import "gorm.io/gorm"

func prepareMemberMoneyMigration(db *gorm.DB) error {
	return db.Exec(`CREATE SEQUENCE IF NOT EXISTS money_operations_sequence AS bigint`).Error
}

// Explicit constraints avoid GORM creating the target FK as immediate. An
// immediate FK takes KEY SHARE on the original during reservation, before the
// orchestration's FOR UPDATE, and can deadlock two competing submissions.
func applyMemberMoneyConstraints(db *gorm.DB) error {
	constraints := []struct{ table, name, definition string }{
		{"money_operations", "fk_money_operations_actor", `FOREIGN KEY (actor_id) REFERENCES users(id) ON DELETE RESTRICT`},
		{"money_operations", "fk_money_operations_target_transaction", `FOREIGN KEY (target_transaction_id) REFERENCES transactions(id) ON DELETE NO ACTION DEFERRABLE INITIALLY DEFERRED`},
		{"transactions", "fk_transactions_money_operation", `FOREIGN KEY (money_operation_id) REFERENCES money_operations(id) ON DELETE RESTRICT`},
		{"money_operations", "money_operations_valid_id", `CHECK (id <> '00000000-0000-0000-0000-000000000000'::uuid)`},
		{"money_operations", "money_operations_action_target", `CHECK (
			(action = 'create_shared_expense' AND target_transaction_id IS NULL)
			OR (action IN ('correct_topup', 'reverse_transaction', 'correct_shared_expense', 'correct_settlement') AND target_transaction_id IS NOT NULL))`},
		{"money_operations", "money_operations_payload", `CHECK (payload_hash ~ '^[0-9a-f]{64}$' AND jsonb_typeof(canonical_payload) = 'object')`},
		{"money_operations", "money_operations_response", `CHECK (
			(response_status IS NULL AND response_body IS NULL)
			OR (response_status IS NOT NULL AND response_body IS NOT NULL
				AND response_status BETWEEN 200 AND 299 AND jsonb_typeof(response_body) = 'object'))`},
		{"transactions", "transactions_operation_step", `CHECK (
			(money_operation_id IS NULL AND operation_step IS NULL)
			OR (money_operation_id IS NOT NULL AND operation_step IS NOT NULL AND operation_step IN (0, 1)))`},
	}
	for _, c := range constraints {
		// Names/definitions are fixed migration constants, never request inputs.
		statement := `DO $migration$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conrelid = '` + c.table + `'::regclass AND conname = '` + c.name + `') THEN
				ALTER TABLE ` + c.table + ` ADD CONSTRAINT ` + c.name + ` ` + c.definition + `;
			END IF;
		END $migration$`
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	statements := []string{
		`ALTER SEQUENCE money_operations_sequence OWNED BY money_operations.sequence`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_money_operations_sequence ON money_operations (sequence)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_money_operation_step ON transactions (money_operation_id, operation_step)`,
		`CREATE INDEX IF NOT EXISTS idx_money_operations_target_transaction ON money_operations (target_transaction_id)`,
		// Query the final row, rather than the INSERT trigger's initial NEW value:
		// reservation and completion happen in separate statements in the same tx.
		`CREATE OR REPLACE FUNCTION check_money_operation_complete() RETURNS trigger
		 LANGUAGE plpgsql AS $function$
		 BEGIN
			IF EXISTS (SELECT 1 FROM money_operations WHERE id = NEW.id AND response_status IS NULL) THEN
				RAISE EXCEPTION 'money operation must complete before commit' USING ERRCODE = '23514';
			END IF;
			RETURN NULL;
		 END $function$`,
		`CREATE OR REPLACE FUNCTION protect_completed_money_operation() RETURNS trigger
		 LANGUAGE plpgsql AS $function$
		 BEGIN
			IF OLD.response_status IS NOT NULL THEN
				RAISE EXCEPTION 'completed money operation is immutable' USING ERRCODE = '23514';
			END IF;
			IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
			RETURN NEW;
		 END $function$`,
		`DO $migration$ BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid = 'money_operations'::regclass AND tgname = 'money_operation_complete') THEN
				CREATE CONSTRAINT TRIGGER money_operation_complete AFTER INSERT OR UPDATE ON money_operations
				DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_money_operation_complete();
			END IF;
			IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid = 'money_operations'::regclass AND tgname = 'money_operation_immutable') THEN
				CREATE TRIGGER money_operation_immutable BEFORE UPDATE OR DELETE ON money_operations
				FOR EACH ROW EXECUTE FUNCTION protect_completed_money_operation();
			END IF;
		END $migration$`,
	}
	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			return err
		}
	}
	return nil
}
