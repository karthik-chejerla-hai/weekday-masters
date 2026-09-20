package seed

import (
	"fmt"
	"os"
	"strings"
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

// --- seeding from an export ------------------------------------------------

func runFrom(t *testing.T, body string, assets *AssetSplit) *Report {
	t.Helper()
	report, err := Run(Options{
		Now: fixedNow, FromCSV: writeExport(t, body),
		Assets: assets, AdminName: "Alex Stone",
	})
	if err != nil {
		t.Fatalf("seed from export failed: %v", err)
	}
	return report
}

// The whole point of this fixture: what lands in the ledger is what the
// spreadsheet says, to the cent.
func TestSeedFromExportMatchesTheSpreadsheet(t *testing.T) {
	requireDB(t)
	report := runFrom(t, goodExport, nil)

	if report.UsersCreated != 3 {
		t.Errorf("created %d members, want 3", report.UsersCreated)
	}
	if report.TransactionsRead != 4 {
		t.Errorf("reconciled %d transactions, want 4", report.TransactionsRead)
	}

	want := map[string]int64{"Alex Stone": 9000, "Jo Patel": 2000, "Sam Reilly": 0}
	got := map[string]int64{}
	for _, line := range report.Balances {
		got[line.Name] = line.BalanceCred
	}
	for name, cents := range want {
		if got[name] != cents {
			t.Errorf("%s balance = %d, want %d", name, got[name], cents)
		}
	}
}

func TestSeedFromExportMarksRemovedMembers(t *testing.T) {
	requireDB(t)
	runFrom(t, goodExport, nil)

	var user models.User
	if err := database.DB.Where("email = ?", "sam-reilly@"+EmailDomain).
		First(&user).Error; err != nil {
		t.Fatal(err)
	}
	if user.MembershipStatus != models.MembershipRemoved {
		t.Errorf("status = %q, want removed — removal is a status change, not a delete",
			user.MembershipStatus)
	}
}

// Real names, but never a real address: these rows must not be mailable and
// must not collide with the actual person's account.
func TestSeedFromExportUsesUndeliverableAddresses(t *testing.T) {
	requireDB(t)
	runFrom(t, goodExport, nil)

	var users []models.User
	if err := database.DB.Find(&users).Error; err != nil {
		t.Fatal(err)
	}
	if len(users) == 0 {
		t.Fatal("no users were seeded")
	}
	for _, user := range users {
		if !strings.HasSuffix(user.Email, "@"+EmailDomain) {
			t.Errorf("%q is outside the reserved domain", user.Email)
		}
	}
}

func TestSeedFromExportLeavesTheClubBalanced(t *testing.T) {
	requireDB(t)
	runFrom(t, goodExport, nil)

	report, err := services.NewLedgerService().Integrity()
	if err != nil {
		t.Fatalf("integrity check failed: %v", err)
	}
	if !report.Balanced {
		t.Errorf("the seeded club does not balance: %+v", report)
	}
}

// The asset split decides where the club's money is recorded as sitting, and
// surplus is the balancing figure. An honest split leaves no surplus.
func TestSeedFromExportHonoursTheAssetSplit(t *testing.T) {
	requireDB(t)
	runFrom(t, goodExport, &AssetSplit{
		BankCents:        6000,
		CourtCreditCents: 3000,
		ShuttleCents:     2000,
		ShuttleUnits:     12,
	})

	ledger := services.NewLedgerService()
	position, err := ledger.Position()
	if err != nil {
		t.Fatalf("position: %v", err)
	}
	if !position.Balanced {
		t.Errorf("position does not balance: %+v", position)
	}

	stock, err := ledger.StockPosition(nil)
	if err != nil {
		t.Fatalf("stock: %v", err)
	}
	if stock.Units != 12 || stock.ValueCents != 2000 {
		t.Errorf("shuttle stock = %+v, want 12 units at 2000c", stock)
	}
}

// Opening balances can only be posted once, so a second run must not try.
func TestSeedFromExportIsIdempotent(t *testing.T) {
	requireDB(t)
	path := writeExport(t, goodExport)

	first, err := Run(Options{Now: fixedNow, FromCSV: path, AdminName: "Alex Stone"})
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	second, err := Run(Options{Now: fixedNow, FromCSV: path, AdminName: "Alex Stone"})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}

	if second.UsersCreated != 0 {
		t.Errorf("second run created %d members, want 0", second.UsersCreated)
	}
	if second.MoneyPosted != 0 {
		t.Errorf("second run posted %d transactions, want 0", second.MoneyPosted)
	}
	if second.SessionsMade != 0 || second.RSVPsMade != 0 {
		t.Errorf("second run made %d sessions and %d RSVPs, want 0 and 0",
			second.SessionsMade, second.RSVPsMade)
	}
	for i, line := range second.Balances {
		if line.BalanceCred != first.Balances[i].BalanceCred {
			t.Errorf("%s balance moved on a second run: %d -> %d",
				line.Name, first.Balances[i].BalanceCred, line.BalanceCred)
		}
	}
}

func TestSeedFromExportRefusesADifferentOpeningPosition(t *testing.T) {
	requireDB(t)
	runFrom(t, goodExport, nil)

	changed := strings.NewReplacer(
		"120.00,AUD,-120.00,120.00", "130.00,AUD,-130.00,130.00",
		"AUD,-110.00,90.00,20.00", "AUD,-120.00,100.00,20.00",
	).Replace(goodExport)
	_, err := Run(Options{
		Now: fixedNow, FromCSV: writeExport(t, changed), AdminName: "Alex Stone",
	})
	if err == nil {
		t.Fatal("a different export was accepted over an existing opening balance")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("unexpected mismatch error: %v", err)
	}
}

func TestSeedFromExportCanChangeTheSeededAdmin(t *testing.T) {
	requireDB(t)
	path := writeExport(t, goodExport)
	if _, err := Run(Options{Now: fixedNow, FromCSV: path, AdminName: "Alex Stone"}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if _, err := Run(Options{Now: fixedNow, FromCSV: path, AdminName: "Jo Patel"}); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var alex, jo models.User
	if err := database.DB.Where("email = ?", "alex-stone@"+EmailDomain).First(&alex).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Where("email = ?", "jo-patel@"+EmailDomain).First(&jo).Error; err != nil {
		t.Fatal(err)
	}
	if alex.Role != models.RolePlayer || jo.Role != models.RoleAdmin {
		t.Errorf("roles after changing admin: Alex=%q Jo=%q", alex.Role, jo.Role)
	}
}

// Settling would charge the players and move them off the exported figures.
func TestSeedFromExportSettlesNothing(t *testing.T) {
	requireDB(t)
	report := runFrom(t, goodExport, nil)

	if report.SettlementMade {
		t.Error("a settlement was posted; the balances would no longer match the export")
	}

	var settlements int64
	if err := database.DB.Model(&models.Settlement{}).Count(&settlements).Error; err != nil {
		t.Fatal(err)
	}
	if settlements != 0 {
		t.Errorf("found %d settlements, want 0", settlements)
	}
}

func TestSeedFromExportRefusesAnExportWithoutTheAdmin(t *testing.T) {
	requireDB(t)

	noAdmin := `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone

2026-01-12,Top-Up,General,50.00,AUD,-50.00,50.00

2026-01-12,Total balance, , ,AUD,-50.00,50.00
`
	_, err := Run(Options{
		Now: fixedNow, FromCSV: writeExport(t, noAdmin),
		AdminName: "Someone Not In The Export",
	})
	if err == nil {
		t.Fatal("an export naming nobody to make admin was accepted")
	}
	if !strings.Contains(err.Error(), "Alex Stone") {
		t.Errorf("the error should list who the export does name, got: %v", err)
	}
	var users int64
	if err := database.DB.Model(&models.User{}).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Errorf("admin validation left %d partial users behind", users)
	}
}

func TestSeedFromExportRefusesARemovedAdmin(t *testing.T) {
	requireDB(t)
	_, err := Run(Options{
		Now: fixedNow, FromCSV: writeExport(t, goodExport), AdminName: "Sam Reilly",
	})
	if err == nil {
		t.Fatal("a removed member was accepted as the only admin")
	}
}

func TestSeedFromExportRespectsExistingSessionCapacity(t *testing.T) {
	requireDB(t)

	creator := createRosterTestUser(t, 0)
	sessions, err := seedSessions(fixedNow, creator.ID, &Report{}, map[string]bool{"upcoming": true})
	if err != nil {
		t.Fatalf("preparing session: %v", err)
	}
	session := sessions["upcoming"]
	for i := 0; i < session.MaxPlayers-1; i++ {
		user := creator
		if i > 0 {
			user = createRosterTestUser(t, i)
		}
		rsvp := models.RSVP{
			SessionID: session.ID, UserID: user.ID, Status: models.RSVPStatusIn,
			RSVPTimestamp: session.RSVPDeadline.Add(-24 * time.Hour),
		}
		if err := database.DB.Create(&rsvp).Error; err != nil {
			t.Fatal(err)
		}
	}

	runFrom(t, goodExport, nil)

	var confirmed int64
	if err := database.DB.Model(&models.RSVP{}).
		Where("session_id = ? AND status = ?", session.ID, models.RSVPStatusIn).
		Count(&confirmed).Error; err != nil {
		t.Fatal(err)
	}
	if confirmed != int64(session.MaxPlayers) {
		t.Errorf("session has %d confirmed players, want capacity %d", confirmed, session.MaxPlayers)
	}
}

func createRosterTestUser(t *testing.T, i int) models.User {
	t.Helper()
	user := models.User{
		Auth0ID:          fmt.Sprintf("test|capacity-%d", i),
		Email:            fmt.Sprintf("capacity-%d@example.invalid", i),
		Name:             fmt.Sprintf("Capacity %d", i),
		Role:             models.RolePlayer,
		MembershipStatus: models.MembershipApproved,
		IsPlayer:         true,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	return user
}

// A bad export must abort before anything is written.
func TestSeedFromExportWritesNothingWhenTheFileDoesNotReconcile(t *testing.T) {
	requireDB(t)

	broken := `Date,Description,Category,Cost,Currency,Badminton Account,Karthik Chejerla

2026-01-12,Game,General,30.00,AUD,30.00,-25.00

2026-01-12,Total balance, , ,AUD,30.00,-25.00
`
	if _, err := Run(Options{
		Now: fixedNow, FromCSV: writeExport(t, broken), AdminName: "Karthik Chejerla",
	}); err == nil {
		t.Fatal("an export that does not balance was accepted")
	}

	var users int64
	if err := database.DB.Model(&models.User{}).Count(&users).Error; err != nil {
		t.Fatal(err)
	}
	if users != 0 {
		t.Errorf("%d users were written from a rejected export", users)
	}
}
