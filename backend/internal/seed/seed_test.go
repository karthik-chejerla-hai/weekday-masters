package seed

import (
	"os"
	"testing"
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/testsupport"
)

func TestMain(m *testing.M) {
	testsupport.Setup("seed_test")
	os.Exit(m.Run())
}

func requireDB(t *testing.T) {
	t.Helper()
	testsupport.RequireDB(t)
}

// fixedNow keeps session dates deterministic. A Wednesday, so the seeded
// sessions land on ordinary days either side of it.
var fixedNow = time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)

func run(t *testing.T) *Report {
	t.Helper()
	report, err := Run(Options{Now: fixedNow})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	return report
}

func TestSeedCreatesTheWholeRoster(t *testing.T) {
	requireDB(t)
	report := run(t)

	if report.UsersCreated != len(members) {
		t.Errorf("created %d members, want %d", report.UsersCreated, len(members))
	}

	var count int64
	if err := database.DB.Model(&models.User{}).
		Where("email LIKE ?", "%@"+EmailDomain).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != int64(len(members)) {
		t.Errorf("found %d tagged members in the database, want %d", count, len(members))
	}
}

// The point of the seed: each member lands in a different state.
func TestSeedProducesEachIntendedState(t *testing.T) {
	requireDB(t)
	run(t)

	ledger := services.NewLedgerService()
	club := clubSettings(t)

	balances := map[string]int64{}
	for _, spec := range members {
		user := userByKey(t, spec.key)
		if user.IsApproved() {
			balance, err := ledger.BalanceOfUser(user.ID)
			if err != nil {
				t.Fatalf("balance for %s: %v", spec.key, err)
			}
			balances[spec.key] = balance
		}
	}

	if balances["credit"] <= club.LowBalanceThresholdCents {
		t.Errorf("the in-credit member is at %d, want comfortably above the %d threshold",
			balances["credit"], club.LowBalanceThresholdCents)
	}

	low := balances["low"]
	if low <= 0 || low >= club.LowBalanceThresholdCents {
		t.Errorf("the low member is at %d, want between 0 and the %d threshold",
			low, club.LowBalanceThresholdCents)
	}

	if balances["negative"] >= 0 {
		t.Errorf("the indebted member is at %d, want negative", balances["negative"])
	}

	if got := userByKey(t, "pending").MembershipStatus; got != models.MembershipPending {
		t.Errorf("pending member status = %q, want pending", got)
	}

	invited := userByKey(t, "invited")
	if invited.HasSignedIn() {
		t.Errorf("the invited member should not look signed in; auth0_id = %q", invited.Auth0ID)
	}
}

// A second run must be a no-op. The preview workflow runs this on every push to
// a pull request, and the ledger is append-only — a double top-up is permanent.
func TestSeedIsIdempotent(t *testing.T) {
	requireDB(t)
	first := run(t)

	ledger := services.NewLedgerService()
	before := map[string]int64{}
	for _, spec := range members {
		user := userByKey(t, spec.key)
		if user.IsApproved() {
			balance, err := ledger.BalanceOfUser(user.ID)
			if err != nil {
				t.Fatal(err)
			}
			before[spec.key] = balance
		}
	}

	second := run(t)

	if second.UsersCreated != 0 {
		t.Errorf("second run created %d members, want 0", second.UsersCreated)
	}
	if second.UsersReused != first.UsersCreated {
		t.Errorf("second run reused %d members, want %d", second.UsersReused, first.UsersCreated)
	}
	if second.MoneyPosted != 0 {
		t.Errorf("second run posted %d transactions, want 0 — the ledger cannot be un-posted",
			second.MoneyPosted)
	}
	if second.SessionsMade != 0 || second.RSVPsMade != 0 {
		t.Errorf("second run made %d sessions and %d RSVPs, want 0 and 0",
			second.SessionsMade, second.RSVPsMade)
	}

	for key, want := range before {
		user := userByKey(t, key)
		got, err := ledger.BalanceOfUser(user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s balance moved on a second run: %d -> %d", key, want, got)
		}
	}
}

// Whatever else the seed does, it must leave the books balanced — the identity
// the whole ledger design rests on.
func TestSeedLeavesTheClubBalanced(t *testing.T) {
	requireDB(t)
	run(t)

	report, err := services.NewLedgerService().Integrity()
	if err != nil {
		t.Fatalf("integrity check failed: %v", err)
	}
	if !report.Balanced {
		t.Errorf("the seeded club does not balance: %+v", report)
	}
}

func TestSeedLeavesOnePastSessionUnsettled(t *testing.T) {
	requireDB(t)
	run(t)

	settlement := services.NewSettlementService(services.NewLedgerService())

	settled := sessionByTitle(t, Tag+" Thursday night (settled)")
	live, err := settlement.LiveSettlementForSession(settled.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live == nil {
		t.Error("the settled session has no settlement")
	}

	// This is what the home screen's Settle button needs to act on.
	unsettled := sessionByTitle(t, Tag+" Thursday night (awaiting settlement)")
	live, err = settlement.LiveSettlementForSession(unsettled.ID)
	if err != nil {
		t.Fatal(err)
	}
	if live != nil {
		t.Error("the session left for the Settle button is already settled")
	}
}

func TestSeedSessionsStraddleToday(t *testing.T) {
	requireDB(t)
	run(t)

	upcoming := sessionByTitle(t, Tag+" Thursday night")
	if upcoming.EndsAt == nil {
		t.Fatal("the upcoming session has no resolved ends_at")
	}
	if !upcoming.EndsAt.After(fixedNow) {
		t.Errorf("the upcoming session ends at %s, which is not after %s",
			upcoming.EndsAt, fixedNow)
	}

	past := sessionByTitle(t, Tag+" Thursday night (settled)")
	if past.EndsAt == nil {
		t.Fatal("the settled session has no resolved ends_at")
	}
	if !past.EndsAt.Before(fixedNow) {
		t.Errorf("the settled session ends at %s, which is not before %s",
			past.EndsAt, fixedNow)
	}
}

// The guest line is what makes the settled session exercise the host-charged
// path rather than a plain even split.
func TestSeedSettlementChargesAGuestToTheHost(t *testing.T) {
	requireDB(t)
	run(t)

	session := sessionByTitle(t, Tag+" Thursday night (settled)")

	var guests int64
	err := database.DB.Model(&models.ChargeLine{}).
		Joins("JOIN settlements ON settlements.id = charge_lines.settlement_id").
		Where("settlements.session_id = ? AND charge_lines.guest_name <> ''", session.ID).
		Count(&guests).Error
	if err != nil {
		t.Fatal(err)
	}
	if guests != 1 {
		t.Errorf("found %d guest charge lines, want 1", guests)
	}
}

// Nothing the seed writes may be addressable in the real world: a preview runs
// against a branch of production with the notification services configured.
func TestSeededEmailsAreUndeliverable(t *testing.T) {
	requireDB(t)
	run(t)

	var users []models.User
	if err := database.DB.Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	for _, user := range users {
		if len(user.Email) < len(EmailDomain) ||
			user.Email[len(user.Email)-len(EmailDomain):] != EmailDomain {
			t.Errorf("seed run produced a user outside the reserved domain: %q", user.Email)
		}
	}
}

func TestSeedRefusesAnUnmigratedDatabase(t *testing.T) {
	requireDB(t)

	// requireDB reseeds the club; drop it to stand in for "never migrated".
	if err := database.DB.Exec("DELETE FROM clubs").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.SeedDefaultClub(); err != nil {
			t.Fatalf("failed to put the club back: %v", err)
		}
	})

	if _, err := Run(Options{Now: fixedNow}); err == nil {
		t.Fatal("seeding an unmigrated database should fail with a usable message")
	}
}

func userByKey(t *testing.T, key string) models.User {
	t.Helper()
	var user models.User
	if err := database.DB.Where("email = ?", email(key)).First(&user).Error; err != nil {
		t.Fatalf("member %s not found: %v", key, err)
	}
	return user
}

func sessionByTitle(t *testing.T, title string) models.Session {
	t.Helper()
	var session models.Session
	if err := database.DB.Where("title = ?", title).First(&session).Error; err != nil {
		t.Fatalf("session %q not found: %v", title, err)
	}
	return session
}

func clubSettings(t *testing.T) models.Club {
	t.Helper()
	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		t.Fatalf("no club row: %v", err)
	}
	return club
}
