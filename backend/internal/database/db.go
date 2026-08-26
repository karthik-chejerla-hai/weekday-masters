package database

import (
	"log"

	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func Connect(databaseURL string) error {
	var err error
	DB, err = gorm.Open(postgres.Open(databaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return err
	}

	log.Println("Connected to database")
	return nil
}

func Migrate() error {
	log.Println("Running database migrations...")

	err := DB.AutoMigrate(
		&models.Club{},
		&models.User{},
		&models.Session{},
		&models.RSVP{},
		// Notification models
		&models.UserNotificationPreferences{},
		&models.UserPushToken{},
		&models.Notification{},
		&models.Announcement{},
		// Ledger models
		&models.Account{},
		&models.Transaction{},
		&models.LedgerEntry{},
	)
	if err != nil {
		return err
	}

	if err := applyLedgerConstraints(); err != nil {
		return err
	}

	if err := backfillSessionTimestamps(); err != nil {
		return err
	}

	// Seed default club if not exists
	var count int64
	DB.Model(&models.Club{}).Count(&count)
	if count == 0 {
		club := models.Club{
			Name:                     "Rally Badminton Club",
			BaseHours:                2,
			BaseRateCents:            3000,
			ExtraRateCents:           2300,
			ShuttlesPerHour:          5,
			LowBalanceThresholdCents: 2000,
		}
		DB.Create(&club)
		log.Println("Created default club")
	}

	if err := SeedClubAccounts(); err != nil {
		return err
	}

	log.Println("Database migrations completed")
	return nil
}

// applyLedgerConstraints adds the guarantees AutoMigrate cannot express.
//
// These are backstops rather than the primary defence — the services enforce the
// same rules under a row lock — but a constraint survives a future caller who
// forgets the lock, and that is worth the raw SQL.
func applyLedgerConstraints() error {
	statements := []string{
		// A transaction may be reversed at most once. Two reversals of the same
		// original would double-unwind the balances.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_one_reversal
		   ON transactions (reverses_transaction_id)
		 WHERE reverses_transaction_id IS NOT NULL`,

		// Ledger entries are read by account constantly and written once.
		`CREATE INDEX IF NOT EXISTS idx_ledger_entries_account_created
		   ON ledger_entries (account_id, created_at)`,
	}

	for _, stmt := range statements {
		if err := DB.Exec(stmt).Error; err != nil {
			return err
		}
	}
	return nil
}

// SeedClubAccounts creates the four singleton club accounts.
//
// They exist from the first migration, before any UI manages them, because the
// alternative — a single lumped club account, split apart later — would mean
// rewriting ledger history once real money was in it.
func SeedClubAccounts() error {
	names := map[models.AccountKind]string{
		models.AccountKindBank:         "Club bank",
		models.AccountKindCourtCredit:  "Court credit (venue account)",
		models.AccountKindShuttleStock: "Shuttle stock",
		models.AccountKindSurplus:      "Club surplus",
	}

	for _, kind := range models.ClubAccountKinds {
		var count int64
		if err := DB.Model(&models.Account{}).Where("kind = ?", kind).Count(&count).Error; err != nil {
			return err
		}
		if count > 0 {
			continue
		}

		account := models.Account{Kind: kind, Name: names[kind]}
		if err := DB.Create(&account).Error; err != nil {
			return err
		}
		log.Printf("Created club account: %s", names[kind])
	}
	return nil
}

// backfillSessionTimestamps fills in resolved start and end instants for
// sessions created before those columns existed.
//
// PostgreSQL resolves the Sydney wall-clock time itself, which gets DST right
// without the application looping over rows. Guarded by IS NULL so it is
// idempotent and safe to re-run on every migrate.
//
// An end time at or before the start means the session ran past midnight.
func backfillSessionTimestamps() error {
	return DB.Exec(`
		UPDATE sessions
		   SET starts_at = (session_date + start_time::time) AT TIME ZONE 'Australia/Sydney',
		       ends_at = CASE
		           WHEN end_time::time > start_time::time
		           THEN (session_date + end_time::time) AT TIME ZONE 'Australia/Sydney'
		           ELSE (session_date + INTERVAL '1 day' + end_time::time) AT TIME ZONE 'Australia/Sydney'
		       END
		 WHERE starts_at IS NULL
		   AND start_time <> ''
		   AND end_time <> ''
	`).Error
}
