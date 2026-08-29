package main

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/testsupport"
	"github.com/weekday-masters/backend/internal/utils"
)

func TestMain(m *testing.M) {
	testsupport.Setup("seed_test")
	os.Exit(m.Run())
}

func requireDB(t *testing.T) {
	t.Helper()
	testsupport.RequireDB(t)
}

func makeUser(t *testing.T, name string, role models.UserRole) models.User {
	t.Helper()
	user := models.User{
		Auth0ID:          "auth0|" + uuid.NewString(),
		Email:            uuid.NewString() + "@example.com",
		Name:             name,
		Role:             role,
		IsPlayer:         true,
		MembershipStatus: models.MembershipApproved,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("creating %s: %v", name, err)
	}
	return user
}

func count(t *testing.T, table string) int64 {
	t.Helper()
	var n int64
	if err := database.DB.Table(table).Count(&n).Error; err != nil {
		t.Fatalf("counting %s: %v", table, err)
	}
	return n
}

// openingBalances posts a starting position, which is the thing that can only
// happen once.
func openingBalances(t *testing.T, admin models.User, players ...models.User) {
	t.Helper()
	ledger := services.NewLedgerService()

	opening := make([]services.OpeningPlayerBalance, 0, len(players))
	var total int64
	for i, p := range players {
		cents := int64((i + 1) * 1000)
		if _, err := ledger.EnsurePlayerAccount(p.ID, p.Name); err != nil {
			t.Fatalf("account for %s: %v", p.Name, err)
		}
		opening = append(opening, services.OpeningPlayerBalance{UserID: p.ID, BalanceCents: cents})
		total += cents
	}

	if _, err := ledger.RecordOpeningBalances(services.OpeningBalancesInput{
		Players:    opening,
		BankCents:  total,
		OccurredAt: utils.NowInSydney(),
		CreatedBy:  admin.ID,
	}); err != nil {
		t.Fatalf("opening balances: %v", err)
	}
}

// The development loop this exists for: seed, find something wrong, seed again.
// Without a reset the second run is refused, because opening balances are
// accepted exactly once and the ledger has no delete.
func TestResetLedgerAllowsASecondSeed(t *testing.T) {
	requireDB(t)
	admin := makeUser(t, "Admin", models.RoleAdmin)
	player := makeUser(t, "Player", models.RolePlayer)

	openingBalances(t, admin, player)
	if count(t, "transactions") == 0 {
		t.Fatal("no transaction was posted")
	}

	// A second attempt is refused while the first still stands.
	ledger := services.NewLedgerService()
	_, err := ledger.RecordOpeningBalances(services.OpeningBalancesInput{
		Players:    []services.OpeningPlayerBalance{{UserID: player.ID, BalanceCents: 1}},
		BankCents:  1,
		OccurredAt: utils.NowInSydney(),
		CreatedBy:  admin.ID,
	})
	if ledgerErr, ok := services.AsLedgerError(err); !ok || ledgerErr.Code != services.CodeOpeningBalancesRecorded {
		t.Fatalf("second seed err = %v, want opening_balances_already_recorded", err)
	}

	if err := resetLedger(); err != nil {
		t.Fatalf("resetting: %v", err)
	}

	for _, table := range ledgerTables {
		if n := count(t, table); n != 0 {
			t.Errorf("%s still holds %d rows after the reset", table, n)
		}
	}

	// Accounts survive on purpose: balances are derived from entries, so an
	// account with none reads as zero, and the next seed reuses it.
	if count(t, "accounts") == 0 {
		t.Error("the reset took the accounts with it")
	}
	if count(t, "users") != 2 {
		t.Error("the reset deleted users")
	}

	openingBalances(t, admin, player)
}

func TestDropMemberRemovesThemAndTheirMoney(t *testing.T) {
	requireDB(t)
	admin := makeUser(t, "Admin", models.RoleAdmin)
	staying := makeUser(t, "Staying", models.RolePlayer)
	leaving := makeUser(t, "Leaving", models.RolePlayer)

	openingBalances(t, admin, staying, leaving)

	if err := dropMember(leaving); err != nil {
		t.Fatalf("dropping: %v", err)
	}

	var gone int64
	if err := database.DB.Model(&models.User{}).Where("id = ?", leaving.ID).Count(&gone).Error; err != nil {
		t.Fatal(err)
	}
	if gone != 0 {
		t.Error("the member is still there")
	}

	var accounts int64
	if err := database.DB.Model(&models.Account{}).Where("user_id = ?", leaving.ID).
		Count(&accounts).Error; err != nil {
		t.Fatal(err)
	}
	if accounts != 0 {
		t.Error("their account outlived them")
	}

	// Everyone else is untouched.
	if count(t, "users") != 2 {
		t.Errorf("users = %d, want the admin and the member who stayed", count(t, "users"))
	}
}

// A session is club history. Deleting the person who scheduled it must not take
// everyone else's record of it with them.
func TestDropMemberKeepsTheSessionsTheyCreated(t *testing.T) {
	requireDB(t)
	admin := makeUser(t, "Admin", models.RoleAdmin)
	organiser := makeUser(t, "Organiser", models.RolePlayer)

	sessions := services.NewSessionService()
	session, err := sessions.CreateSession(services.CreateSessionInput{
		Title:       "Tuesday",
		SessionDate: utils.NowInSydney().AddDate(0, 0, 10),
		StartTime:   "20:00",
		EndTime:     "22:00",
		Courts:      1,
		CreatedBy:   organiser.ID,
	})
	if err != nil {
		t.Fatalf("creating a session: %v", err)
	}

	if err := dropMember(organiser); err != nil {
		t.Fatalf("dropping: %v", err)
	}

	var kept models.Session
	if err := database.DB.First(&kept, "id = ?", session.ID).Error; err != nil {
		t.Fatalf("the session went with them: %v", err)
	}
	if kept.CreatedBy != admin.ID {
		t.Errorf("created_by = %s, want it reassigned to the admin %s", kept.CreatedBy, admin.ID)
	}
}

func TestSplitNames(t *testing.T) {
	got := splitNames(" Aditya Tadimalla , Karthik Indian ,, ")
	want := []string{"Aditya Tadimalla", "Karthik Indian"}
	if len(got) != len(want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("name %d = %q, want %q", i, got[i], want[i])
		}
	}
	if len(splitNames("")) != 0 {
		t.Error("an empty string should name nobody")
	}
}
