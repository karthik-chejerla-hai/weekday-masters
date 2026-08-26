// Package testsupport holds the shared scratch-database harness for the
// database-backed test suites. It is imported only from _test.go files.
//
// Point TEST_DATABASE_URL at a scratch database to run them:
//
//	docker run -d --name rally-test-db -p 5433:5432 \
//	  -e POSTGRES_USER=badminton -e POSTGRES_PASSWORD=badminton123 \
//	  -e POSTGRES_DB=badminton_club_test postgres:16-alpine
//	TEST_DATABASE_URL="postgres://badminton:badminton123@localhost:5433/badminton_club_test?sslmode=disable" \
//	  go test ./...
//
// With the variable unset the database-backed tests skip, so `go test ./...`
// stays green on a machine with no database. NEVER point this at a database you
// care about: every test truncates the schema.
package testsupport

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"regexp"
	"testing"

	"github.com/weekday-masters/backend/internal/database"
	"gorm.io/gorm/logger"
)

// truncateAll empties every table between tests. Kept in one place so a new
// table cannot be added to the schema and forgotten in one suite's reset.
const truncateAll = `TRUNCATE TABLE charge_lines, settlements,
	ledger_entries, transactions, accounts,
	rsvps, notifications, user_push_tokens,
	user_notification_preferences, announcements, sessions, users, clubs
	RESTART IDENTITY CASCADE`

var dbAvailable bool

var safeSchemaName = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// Setup connects and migrates the scratch database if one is configured. Call it
// from TestMain before m.Run(). It reports whether a database is available.
//
// Each package gets its own PostgreSQL schema, named by the caller. `go test`
// runs packages in parallel, and every test here truncates before it runs — on a
// shared schema one package's reset deletes the rows another package is midway
// through using. Separate schemas keep that parallelism safe.
func Setup(schema string) bool {
	if !safeSchemaName.MatchString(schema) {
		log.Fatalf("invalid test schema name %q", schema)
	}

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		log.Println("TEST_DATABASE_URL not set — skipping database-backed tests")
		return false
	}

	// Connect once against the default schema purely to create ours.
	if err := database.Connect(dsn); err != nil {
		log.Fatalf("TEST_DATABASE_URL is set but unusable: %v", err)
	}
	database.DB.Logger = logger.Default.LogMode(logger.Silent)
	if err := database.DB.Exec("CREATE SCHEMA IF NOT EXISTS " + schema).Error; err != nil {
		log.Fatalf("failed to create test schema %s: %v", schema, err)
	}

	scoped, err := withSearchPath(dsn, schema)
	if err != nil {
		log.Fatalf("failed to scope TEST_DATABASE_URL to schema %s: %v", schema, err)
	}
	if err := database.Connect(scoped); err != nil {
		log.Fatalf("failed to connect to test schema %s: %v", schema, err)
	}
	database.DB.Logger = logger.Default.LogMode(logger.Silent)

	if err := database.Migrate(); err != nil {
		log.Fatalf("failed to migrate test database: %v", err)
	}

	dbAvailable = true
	return true
}

// withSearchPath points the connection at the package's own schema, keeping
// public on the path so built-in extensions still resolve.
func withSearchPath(dsn, schema string) (string, error) {
	parsed, err := url.Parse(dsn)
	if err != nil {
		return "", fmt.Errorf("TEST_DATABASE_URL is not a URL: %w", err)
	}

	query := parsed.Query()
	query.Set("search_path", schema+",public")
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

// Available reports whether Setup found a usable scratch database.
func Available() bool { return dbAvailable }

// RequireDB skips the test when no scratch database is configured, and otherwise
// hands it an empty schema.
func RequireDB(t *testing.T) {
	t.Helper()

	if !dbAvailable {
		t.Skip("set TEST_DATABASE_URL to run database-backed tests")
	}

	if err := database.DB.Exec(truncateAll).Error; err != nil {
		t.Fatalf("failed to reset test database: %v", err)
	}

	// The club row and its accounts are schema-shaped configuration, not test
	// data, so put them back. Truncating clubs matters as much as reseeding it:
	// settlement rates live there, and a test that changes one must not leak
	// that change into the next.
	if err := database.SeedDefaultClub(); err != nil {
		t.Fatalf("failed to reseed the club: %v", err)
	}
	if err := database.SeedClubAccounts(); err != nil {
		t.Fatalf("failed to reseed club accounts: %v", err)
	}
}
