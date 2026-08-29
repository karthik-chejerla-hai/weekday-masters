package services

import (
	"errors"
	"testing"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// A club moving off Splitwise seeds rows that already carry balances. The first
// real login has to land on the seeded row rather than beside it — a second row
// for the same person would leave the balance stranded on an account nobody can
// reach, and the ledger is append-only, so there is no tidy way back.

// seedRow creates the kind of row cmd/seed writes: approved, carrying a real
// address, with a placeholder subject no Auth0 token can hold.
func seedRow(t *testing.T, name, email string) *models.User {
	t.Helper()
	user := models.User{
		Auth0ID:          SeedAuth0Prefix + name,
		Email:            email,
		Name:             name,
		Role:             models.RolePlayer,
		IsPlayer:         true,
		MembershipStatus: models.MembershipApproved,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatalf("seeding %s: %v", name, err)
	}
	return &user
}

func countUsers(t *testing.T, email string) int64 {
	t.Helper()
	var n int64
	if err := database.DB.Model(&models.User{}).Where("email = ?", email).Count(&n).Error; err != nil {
		t.Fatalf("counting %s: %v", email, err)
	}
	return n
}

func TestRegisterUserClaimsSeededRow(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	seeded := seedRow(t, "Hari P.", "hari@example.com")

	claimed, err := us.RegisterUser(&Auth0Profile{
		Sub:           "google-oauth2|11642",
		Email:         "hari@example.com",
		Name:          "Hari Prasad",
		Picture:       "https://example.com/hari.jpg",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("registering: %v", err)
	}

	if claimed.ID != seeded.ID {
		t.Fatalf("claimed a different row: got %s, want the seeded %s", claimed.ID, seeded.ID)
	}
	if got := countUsers(t, "hari@example.com"); got != 1 {
		t.Errorf("%d rows for the address, want 1 — the balance must not be split across two", got)
	}
	if claimed.Auth0ID != "google-oauth2|11642" {
		t.Errorf("auth0_id = %q, want the real subject", claimed.Auth0ID)
	}
	if claimed.Name != "Hari Prasad" || claimed.ProfilePicture == "" {
		t.Errorf("display fields not refreshed: name=%q picture=%q", claimed.Name, claimed.ProfilePicture)
	}
	if claimed.MembershipStatus != models.MembershipApproved {
		t.Errorf("status = %s, want the seeded approval to survive", claimed.MembershipStatus)
	}
}

// The email is the whole basis of the claim, so it has to be one Auth0 vouches
// for. An unverified address is an assertion by the caller, and honouring it
// would hand a stranger whatever balance sat on that row.
func TestUnverifiedEmailCannotClaimASeededRow(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	seeded := seedRow(t, "Srikanth", "srikanth@example.com")

	_, err := us.RegisterUser(&Auth0Profile{
		Sub:           "google-oauth2|impostor",
		Email:         "srikanth@example.com",
		Name:          "Not Srikanth",
		EmailVerified: false,
	})
	if !errors.Is(err, ErrEmailAlreadyRegistered) {
		t.Fatalf("err = %v, want ErrEmailAlreadyRegistered", err)
	}
	if got := countUsers(t, "srikanth@example.com"); got != 1 {
		t.Errorf("%d rows for the address, want the seeded one only", got)
	}

	var untouched models.User
	if err := database.DB.First(&untouched, "id = ?", seeded.ID).Error; err != nil {
		t.Fatalf("reloading the seeded row: %v", err)
	}
	if untouched.Auth0ID != SeedAuth0Prefix+"Srikanth" {
		t.Errorf("seeded subject = %q, want it left alone", untouched.Auth0ID)
	}
}

// Claiming is once only: the WHERE clause carries the seed prefix, so a second
// login on the same address finds nothing left to take.
func TestSeededRowIsClaimedOnlyOnce(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	seeded := seedRow(t, "Prabhu S.", "prabhu@example.com")

	first, err := us.RegisterUser(&Auth0Profile{
		Sub: "google-oauth2|first", Email: "prabhu@example.com",
		Name: "Prabhu", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("first login: %v", err)
	}
	if first.ID != seeded.ID {
		t.Fatal("first login did not claim the seeded row")
	}

	_, err = us.RegisterUser(&Auth0Profile{
		Sub: "google-oauth2|second", Email: "prabhu@example.com",
		Name: "Someone Else", EmailVerified: true,
	})
	if !errors.Is(err, ErrEmailAlreadyRegistered) {
		t.Fatalf("second login err = %v, want ErrEmailAlreadyRegistered — the row was "+
			"already claimed and its balance must not change hands", err)
	}

	var still models.User
	if err := database.DB.First(&still, "id = ?", seeded.ID).Error; err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if still.Auth0ID != "google-oauth2|first" {
		t.Errorf("auth0_id = %q, want the first claimant to keep it", still.Auth0ID)
	}
}

// The seeder approves the players it writes, but it does not know which of them
// is the admin. That is ADMIN_EMAIL's job, and it has to survive the claim.
func TestClaimingPromotesTheConfiguredAdmin(t *testing.T) {
	requireDB(t)
	adminEmail := "karthik@example.com"
	us := NewUserService(adminEmail)

	seeded := seedRow(t, "Karthik C.", adminEmail)

	claimed, err := us.RegisterUser(&Auth0Profile{
		Sub: "google-oauth2|karthik", Email: adminEmail,
		Name: "Karthik C.", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("registering: %v", err)
	}

	if claimed.ID != seeded.ID {
		t.Fatal("the admin did not claim their seeded row")
	}
	if claimed.Role != models.RoleAdmin {
		t.Errorf("role = %s, want admin", claimed.Role)
	}
}

// An address nobody was seeded under registers as it always did.
func TestRegisterUserWithoutASeededRowIsUnchanged(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	seedRow(t, "Naresh K.", "naresh@example.com")

	created, err := us.RegisterUser(&Auth0Profile{
		Sub: "google-oauth2|newcomer", Email: "newcomer@example.com",
		Name: "Newcomer", EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("registering: %v", err)
	}
	if created.Role != models.RolePending || created.MembershipStatus != models.MembershipPending {
		t.Errorf("got role=%s status=%s, want a pending join request",
			created.Role, created.MembershipStatus)
	}
}
