package services

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// Admin member management turns on two things that are easy to get subtly
// wrong: an invited row is a real member before anyone has signed in, and
// removing a member must not be a delete. These cover both, plus the guards
// that stop an admin locking themselves — or the whole club — out.

func invite(t *testing.T, us *UserService, email, name string) *models.User {
	t.Helper()
	user, err := us.InviteMember(InviteMemberInput{Email: email, Name: name})
	if err != nil {
		t.Fatalf("failed to invite %s: %v", email, err)
	}
	return user
}

func TestInviteMember_CreatesApprovedMemberWithLedgerAccount(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	user, err := us.InviteMember(InviteMemberInput{
		Email:       "  Newbie@Example.COM ",
		Name:        "Priya Raman",
		Nickname:    "Pri",
		PhoneNumber: "+61412345678",
	})
	if err != nil {
		t.Fatalf("failed to invite member: %v", err)
	}

	if user.Email != "newbie@example.com" {
		t.Fatalf("email should be normalised, got %q", user.Email)
	}
	if user.MembershipStatus != models.MembershipApproved || user.Role != models.RolePlayer {
		t.Fatalf("invited member should be an approved player, got status=%s role=%s",
			user.MembershipStatus, user.Role)
	}
	if user.HasSignedIn() {
		t.Fatal("an invited member has not signed in yet")
	}
	if user.DisplayName() != "Pri" {
		t.Fatalf("nickname should win for display, got %q", user.DisplayName())
	}

	// Chargeable from the moment they are invited, which needs an account.
	if _, err := NewLedgerService().PlayerAccountID(user.ID); err != nil {
		t.Fatalf("invited member should have a ledger account: %v", err)
	}
}

func TestInviteMember_RejectsDuplicatesAndBadInput(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	invite(t, us, "taken@example.com", "Taken Already")

	cases := []struct {
		name  string
		input InviteMemberInput
		want  string
	}{
		{"duplicate email, differing only in case", InviteMemberInput{Email: "TAKEN@example.com", Name: "Impostor"}, "already"},
		{"missing name", InviteMemberInput{Email: "someone@example.com"}, "name is required"},
		{"missing email", InviteMemberInput{Name: "No Address"}, "email is required"},
		{"malformed email", InviteMemberInput{Email: "not-an-email", Name: "Wrong Shape"}, "email address"},
		{"role the invite may not grant", InviteMemberInput{Email: "p@example.com", Name: "P", Role: models.RolePending}, "players or admins"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := us.InviteMember(tc.input); err == nil {
				t.Fatal("expected an error, got nil")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected an error mentioning %q, got %q", tc.want, err)
			}
		})
	}
}

// The whole point of an invite: signing in claims the row an admin prepared,
// rather than landing in the approval queue as a stranger.
func TestRegisterUser_AdoptsMatchingInvite(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	invited, err := us.InviteMember(InviteMemberInput{
		Email:    "adoptee@example.com",
		Name:     "Club Name",
		Nickname: "Smash",
	})
	if err != nil {
		t.Fatalf("failed to invite: %v", err)
	}

	user, isNew, err := us.RegisterUser(&Auth0Profile{
		Sub:           "google-oauth2|9001",
		Email:         "Adoptee@Example.com",
		Name:          "Google Legal Name",
		Picture:       "https://example.com/pic.jpg",
		EmailVerified: true,
	})
	if err != nil {
		t.Fatalf("failed to register against an invite: %v", err)
	}

	if isNew {
		t.Fatal("adopting an invite is not a new registration")
	}
	if user.ID != invited.ID {
		t.Fatalf("expected the invited row to be adopted, got a different user %s", user.ID)
	}
	if user.Auth0ID != "google-oauth2|9001" || !user.HasSignedIn() {
		t.Fatalf("expected the row to be bound to the Auth0 subject, got %q", user.Auth0ID)
	}
	if !user.IsApproved() || user.Role != models.RolePlayer {
		t.Fatalf("an adopted invite keeps its membership, got status=%s role=%s",
			user.MembershipStatus, user.Role)
	}
	// The admin's name and nickname are the deliberate ones; Google's picture
	// fills a gap the admin could not.
	if user.Name != "Club Name" || user.Nickname != "Smash" {
		t.Fatalf("the admin's naming should survive adoption, got name=%q nickname=%q", user.Name, user.Nickname)
	}
	if user.ProfilePicture != "https://example.com/pic.jpg" {
		t.Fatalf("expected the Auth0 picture to fill in, got %q", user.ProfilePicture)
	}

	var count int64
	if err := database.DB.Model(&models.User{}).Count(&count).Error; err != nil {
		t.Fatalf("failed to count users: %v", err)
	}
	if count != 1 {
		t.Fatalf("adoption must not leave a duplicate row, got %d users", count)
	}
}

func TestRegisterUser_RefusesToHijackASignedInAccount(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	if _, _, err := us.RegisterUser(&Auth0Profile{
		Sub:           "google-oauth2|first",
		Email:         "shared@example.com",
		Name:          "First Arrival",
		EmailVerified: true,
	}); err != nil {
		t.Fatalf("failed to register the first identity: %v", err)
	}

	_, _, err := us.RegisterUser(&Auth0Profile{
		Sub:           "auth0|second",
		Email:         "shared@example.com",
		Name:          "Second Arrival",
		EmailVerified: true,
	})
	if !errors.Is(err, ErrEmailAlreadyLinked) {
		t.Fatalf("expected ErrEmailAlreadyLinked, got %v", err)
	}
}

func TestRegisterUser_RefusesAnUnverifiedClaimOnAnInvite(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	invite(t, us, "unverified@example.com", "Waiting Member")

	_, _, err := us.RegisterUser(&Auth0Profile{
		Sub:           "auth0|nope",
		Email:         "unverified@example.com",
		Name:          "Unverified",
		EmailVerified: false,
	})
	if !errors.Is(err, ErrInviteEmailNotVerified) {
		t.Fatalf("expected ErrInviteEmailNotVerified, got %v", err)
	}
}

func TestUpdateMemberDetails_EditsAndRenamesTheLedgerAccount(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user := invite(t, us, "editable@example.com", "Original Name")

	nickname := "Ace"
	phone := " +61400000000 "
	name := "Corrected Name"
	updated, err := us.UpdateMemberDetails(user.ID, UpdateMemberInput{
		Name:        &name,
		Nickname:    &nickname,
		PhoneNumber: &phone,
	})
	if err != nil {
		t.Fatalf("failed to update member: %v", err)
	}
	if updated.Name != "Corrected Name" || updated.Nickname != "Ace" || updated.PhoneNumber != "+61400000000" {
		t.Fatalf("unexpected update result: %+v", updated)
	}

	// The balances list reads the account name, so a nickname that stopped at
	// the user row would show two names for one person.
	var account models.Account
	if err := database.DB.Where("user_id = ?", user.ID).First(&account).Error; err != nil {
		t.Fatalf("failed to load the ledger account: %v", err)
	}
	if account.Name != "Ace" {
		t.Fatalf("expected the ledger account to be renamed to the nickname, got %q", account.Name)
	}

	balances, err := NewLedgerService().AllPlayerBalances()
	if err != nil {
		t.Fatalf("failed to list balances: %v", err)
	}
	if len(balances) != 1 || balances[0].Name != "Ace" {
		t.Fatalf("expected the balances list to show the nickname, got %+v", balances)
	}
}

func TestUpdateMemberDetails_EmailIsEditableOnlyBeforeFirstSignIn(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user := invite(t, us, "typo@example.com", "Typo Victim")

	fixed := "Correct@Example.com"
	updated, err := us.UpdateMemberDetails(user.ID, UpdateMemberInput{Email: &fixed})
	if err != nil {
		t.Fatalf("an unclaimed invite's email should be correctable: %v", err)
	}
	if updated.Email != "correct@example.com" {
		t.Fatalf("expected a normalised email, got %q", updated.Email)
	}

	// Once Auth0 owns the identity, the address is not ours to restate.
	if _, _, err := us.RegisterUser(&Auth0Profile{
		Sub:           "google-oauth2|claimed",
		Email:         "correct@example.com",
		Name:          "Typo Victim",
		EmailVerified: true,
	}); err != nil {
		t.Fatalf("failed to claim the invite: %v", err)
	}

	other := "different@example.com"
	if _, err := us.UpdateMemberDetails(user.ID, UpdateMemberInput{Email: &other}); err == nil {
		t.Fatal("expected changing a signed-in member's email to be refused")
	}
}

func TestUpdateMemberDetails_RejectsCollisionsAndEmptyNames(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	invite(t, us, "first@example.com", "First")
	second := invite(t, us, "second@example.com", "Second")

	taken := "first@example.com"
	if _, err := us.UpdateMemberDetails(second.ID, UpdateMemberInput{Email: &taken}); err == nil {
		t.Fatal("expected an error taking another member's email")
	}

	blank := "   "
	if _, err := us.UpdateMemberDetails(second.ID, UpdateMemberInput{Name: &blank}); err == nil {
		t.Fatal("expected an error blanking a member's name")
	}

	if _, err := us.UpdateMemberDetails(uuid.New(), UpdateMemberInput{}); !errors.Is(err, ErrMemberNotFound) {
		t.Fatalf("expected ErrMemberNotFound, got %v", err)
	}
}

func TestRemoveMember_KeepsTheRowAndFreesUpcomingSpots(t *testing.T) {
	requireDB(t)
	rsvps := NewRSVPService(nil)
	us := NewUserService("").WithRSVPs(rsvps)

	admin := invite(t, us, "admin@example.com", "The Admin")
	if _, err := us.UpdateMemberDetails(admin.ID, UpdateMemberInput{Role: rolePtr(models.RoleAdmin)}); err != nil {
		t.Fatalf("failed to make an admin: %v", err)
	}

	leaver := invite(t, us, "leaver@example.com", "Departing Player")
	stayer := invite(t, us, "stayer@example.com", "Waitlisted Player")

	// One court seats six; filling it makes the waitlist real.
	session, err := NewSessionService().CreateSession(CreateSessionInput{
		Title:       "Thursday",
		SessionDate: time.Now().AddDate(0, 0, 10),
		StartTime:   "18:00",
		EndTime:     "20:00",
		Courts:      1,
		CreatedBy:   admin.ID,
	})
	if err != nil {
		t.Fatalf("failed to create a session: %v", err)
	}

	for i := 0; i < 5; i++ {
		filler := invite(t, us, "filler"+string(rune('a'+i))+"@example.com", "Filler")
		mustRSVP(t, rsvps, session.ID, filler.ID, models.RSVPStatusIn)
	}
	mustRSVP(t, rsvps, session.ID, leaver.ID, models.RSVPStatusIn)

	waitlisted := mustRSVP(t, rsvps, session.ID, stayer.ID, models.RSVPStatusIn)
	if waitlisted.Status != models.RSVPStatusWaitlisted {
		t.Fatalf("expected the seventh player to be waitlisted, got %s", waitlisted.Status)
	}

	removed, err := us.RemoveMember(leaver.ID, admin.ID)
	if err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}
	if removed.MembershipStatus != models.MembershipRemoved {
		t.Fatalf("expected a removed membership status, got %s", removed.MembershipStatus)
	}

	// Not a delete: the row is what the ledger and session history point at.
	var stillThere models.User
	if err := database.DB.First(&stillThere, "id = ?", leaver.ID).Error; err != nil {
		t.Fatalf("removal must not delete the row: %v", err)
	}

	promoted, err := rsvps.GetUserRSVPForSession(session.ID, stayer.ID)
	if err != nil {
		t.Fatalf("failed to read the waitlisted RSVP: %v", err)
	}
	if promoted.Status != models.RSVPStatusIn {
		t.Fatalf("expected the freed spot to promote the waitlisted player, got %s", promoted.Status)
	}

	if _, err := rsvps.GetUserRSVPForSession(session.ID, leaver.ID); err == nil {
		t.Fatal("expected the removed member's RSVP to be gone")
	}

	// And they drop out of the club-facing lists.
	approved, err := us.ListApprovedMembers()
	if err != nil {
		t.Fatalf("failed to list approved members: %v", err)
	}
	for _, m := range approved {
		if m.ID == leaver.ID {
			t.Fatal("a removed member should not appear in the approved list")
		}
	}
}

func TestRemoveMember_RefusesWhileMoneyIsOutstanding(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	admin := invite(t, us, "boss@example.com", "Boss")
	player := invite(t, us, "owed@example.com", "Owed Player")

	if _, err := NewLedgerService().RecordTopup(CashInput{
		UserID:      player.ID,
		AmountCents: 2500,
		CreatedBy:   admin.ID,
	}); err != nil {
		t.Fatalf("failed to record a top-up: %v", err)
	}

	_, err := us.RemoveMember(player.ID, admin.ID)
	if err == nil {
		t.Fatal("expected removal to be refused while the member is in credit")
	}
	if !strings.Contains(err.Error(), "$25.00") {
		t.Fatalf("the refusal should name the amount, got %q", err)
	}

	// Paying them out clears the way.
	if _, err := NewLedgerService().RecordWithdrawal(CashInput{
		UserID:      player.ID,
		AmountCents: 2500,
		CreatedBy:   admin.ID,
	}); err != nil {
		t.Fatalf("failed to record a withdrawal: %v", err)
	}
	if _, err := us.RemoveMember(player.ID, admin.ID); err != nil {
		t.Fatalf("expected removal to succeed once square, got %v", err)
	}
}

func TestRemoveMember_GuardsAgainstLockout(t *testing.T) {
	requireDB(t)
	us := NewUserService("")

	admin := invite(t, us, "only-admin@example.com", "Only Admin")
	if _, err := us.UpdateMemberDetails(admin.ID, UpdateMemberInput{Role: rolePtr(models.RoleAdmin)}); err != nil {
		t.Fatalf("failed to make an admin: %v", err)
	}

	if _, err := us.RemoveMember(admin.ID, admin.ID); err == nil {
		t.Fatal("expected removing yourself to be refused")
	}

	// Someone else trying to remove the club's only admin fails for the other
	// reason: there would be nobody left who could promote a replacement.
	other := invite(t, us, "someone@example.com", "Someone Else")
	if _, err := us.RemoveMember(admin.ID, other.ID); err == nil {
		t.Fatal("expected removing the last admin to be refused")
	}
	if _, err := us.UpdateMemberDetails(admin.ID, UpdateMemberInput{Role: rolePtr(models.RolePlayer)}); err == nil {
		t.Fatal("expected demoting the last admin to be refused")
	}

	// With a second admin in place, both become allowed.
	if _, err := us.UpdateMemberDetails(other.ID, UpdateMemberInput{Role: rolePtr(models.RoleAdmin)}); err != nil {
		t.Fatalf("failed to promote a second admin: %v", err)
	}
	if _, err := us.RemoveMember(admin.ID, other.ID); err != nil {
		t.Fatalf("expected removal to succeed with another admin present: %v", err)
	}
}

func TestReinstateMember_RestoresAccess(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	admin := invite(t, us, "chief@example.com", "Chief")
	player := invite(t, us, "returning@example.com", "Returning Player")

	if _, err := us.ReinstateMember(player.ID); err == nil {
		t.Fatal("expected reinstating a member who was never removed to be refused")
	}

	if _, err := us.RemoveMember(player.ID, admin.ID); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	back, err := us.ReinstateMember(player.ID)
	if err != nil {
		t.Fatalf("failed to reinstate member: %v", err)
	}
	if !back.IsApproved() || back.Role != models.RolePlayer {
		t.Fatalf("expected an approved player, got status=%s role=%s", back.MembershipStatus, back.Role)
	}
}

func TestListAllMembers_IncludesEveryStatus(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	admin := invite(t, us, "lister@example.com", "Lister")
	gone := invite(t, us, "gone@example.com", "Gone")
	if _, err := us.RemoveMember(gone.ID, admin.ID); err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	all, err := us.ListAllMembers()
	if err != nil {
		t.Fatalf("failed to list members: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected removed members to still be listed for admins, got %d", len(all))
	}
}

// A pre-existing row predates the nickname column, so it holds NULL. Reading it
// back through GORM must not error.
func TestNicknameNullFromBeforeTheColumnExisted(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user, err := us.InviteMember(InviteMemberInput{Email: "legacy@example.com", Name: "Legacy Row"})
	if err != nil {
		t.Fatalf("failed to invite: %v", err)
	}
	if err := database.DB.Exec("UPDATE users SET nickname = NULL WHERE id = ?", user.ID).Error; err != nil {
		t.Fatalf("failed to null the column: %v", err)
	}

	var loaded models.User
	if err := database.DB.First(&loaded, "id = ?", user.ID).Error; err != nil {
		t.Fatalf("reading a NULL nickname failed: %v", err)
	}
	if loaded.Nickname != "" || loaded.DisplayName() != "Legacy" {
		t.Fatalf("expected an empty nickname falling back to the first name, got %q/%q", loaded.Nickname, loaded.DisplayName())
	}

	members, err := us.ListApprovedMembers()
	if err != nil {
		t.Fatalf("listing members with a NULL nickname failed: %v", err)
	}
	if len(members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(members))
	}

	balances, err := NewLedgerService().AllPlayerBalances()
	if err != nil {
		t.Fatalf("listing balances with a NULL nickname failed: %v", err)
	}
	if len(balances) != 1 || balances[0].Name != "Legacy" {
		t.Fatalf("expected the balances list to fall back to the first name, got %+v", balances)
	}
}

func rolePtr(role models.UserRole) *models.UserRole { return &role }

func mustRSVP(t *testing.T, s *RSVPService, sessionID, userID uuid.UUID, status models.RSVPStatus) *models.RSVP {
	t.Helper()
	rsvp, err := s.CreateOrUpdateRSVP(RSVPInput{SessionID: sessionID, UserID: userID, Status: status}, false)
	if err != nil {
		t.Fatalf("failed to RSVP: %v", err)
	}
	return rsvp
}

// --- self-service nickname -------------------------------------------------

func TestDisplayName_DefaultsToTheFirstName(t *testing.T) {
	cases := []struct {
		name     string
		nickname string
		full     string
		want     string
	}{
		{"first name is the default", "", "Priya Raman", "Priya"},
		{"a chosen nickname wins", "Smash", "Priya Raman", "Smash"},
		{"a mononym is its own first name", "", "Ronaldinho", "Ronaldinho"},
		{"middle names do not leak in", "", "Anna Maria de Souza", "Anna"},
		{"stray whitespace does not become the name", "", "  Wei Zhang ", "Wei"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user := models.User{Name: tc.full, Nickname: tc.nickname}
			if got := user.DisplayName(); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestUpdateProfile_MemberChoosesTheirOwnNickname(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user := invite(t, us, "self@example.com", "Priya Raman")

	// Until they choose one, the club sees their first name.
	if user.DisplayName() != "Priya" {
		t.Fatalf("expected the first name by default, got %q", user.DisplayName())
	}

	nickname := "  Smash  "
	updated, err := us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &nickname})
	if err != nil {
		t.Fatalf("failed to set a nickname: %v", err)
	}
	if updated.Nickname != "Smash" || updated.DisplayName() != "Smash" {
		t.Fatalf("unexpected nickname result: %+v", updated)
	}

	// It has to reach the ledger, which is where money is read under a name.
	balances, err := NewLedgerService().AllPlayerBalances()
	if err != nil {
		t.Fatalf("failed to list balances: %v", err)
	}
	if len(balances) != 1 || balances[0].Name != "Smash" {
		t.Fatalf("expected the balances list to follow the nickname, got %+v", balances)
	}

	// Clearing it falls back to the first name rather than leaving them blank.
	cleared := ""
	back, err := us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &cleared})
	if err != nil {
		t.Fatalf("failed to clear the nickname: %v", err)
	}
	if back.DisplayName() != "Priya" {
		t.Fatalf("expected the first name back, got %q", back.DisplayName())
	}

	balances, err = NewLedgerService().AllPlayerBalances()
	if err != nil {
		t.Fatalf("failed to list balances: %v", err)
	}
	if balances[0].Name != "Priya" {
		t.Fatalf("expected the balances list to fall back too, got %q", balances[0].Name)
	}
}

// Saving one field must not blank the other — the profile form sends whichever
// the member touched.
func TestUpdateProfile_LeavesOmittedFieldsAlone(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user, err := us.InviteMember(InviteMemberInput{
		Email:       "partial@example.com",
		Name:        "Wei Zhang",
		Nickname:    "Wei",
		PhoneNumber: "+61400000000",
	})
	if err != nil {
		t.Fatalf("failed to invite: %v", err)
	}

	phone := "+61411111111"
	updated, err := us.UpdateProfile(user.ID, UpdateProfileInput{PhoneNumber: &phone})
	if err != nil {
		t.Fatalf("failed to update the phone number: %v", err)
	}
	if updated.PhoneNumber != phone || updated.Nickname != "Wei" {
		t.Fatalf("expected the nickname to survive a phone-only save, got %+v", updated)
	}
}

func TestUpdateProfile_RefusesAnOverlongNickname(t *testing.T) {
	requireDB(t)
	us := NewUserService("")
	user := invite(t, us, "verbose@example.com", "Verbose Person")

	long := strings.Repeat("x", MaxNicknameLength+1)
	if _, err := us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &long}); err == nil {
		t.Fatal("expected an over-long nickname to be refused")
	}

	atLimit := strings.Repeat("x", MaxNicknameLength)
	if _, err := us.UpdateProfile(user.ID, UpdateProfileInput{Nickname: &atLimit}); err != nil {
		t.Fatalf("a nickname at the limit should be accepted, got %v", err)
	}
}
