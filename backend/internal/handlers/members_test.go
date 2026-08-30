package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// The member-management endpoints are where an admin's typos and second
// thoughts land, so these assert the status codes and the shape of the refusals
// as much as the happy path.

func TestInviteMember_CreatesAnApprovedMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	var invited models.User
	h.as(admin).post("/api/admin/users", map[string]any{
		"email":        "Newcomer@Example.com",
		"name":         "Wei Zhang",
		"nickname":     "Wei",
		"phone_number": "+61411222333",
	}).expect(http.StatusCreated).decode(&invited)

	if invited.Email != "newcomer@example.com" {
		t.Fatalf("expected a normalised email, got %q", invited.Email)
	}
	if invited.MembershipStatus != models.MembershipApproved || invited.Role != models.RolePlayer {
		t.Fatalf("expected an approved player, got status=%s role=%s",
			invited.MembershipStatus, invited.Role)
	}
	if invited.HasSignedIn() {
		t.Fatal("an invited member has not signed in yet")
	}
}

func TestInviteMember_RejectsMissingFieldsAndDuplicates(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).post("/api/admin/users", map[string]any{"name": "No Email"}).
		expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/users", map[string]any{"email": "x@example.com"}).
		expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/users", map[string]any{
		"email": "x@example.com", "name": "Bad Role", "role": "pending",
	}).expect(http.StatusBadRequest)

	body := h.as(admin).post("/api/admin/users", map[string]any{
		"email": admin.Email, "name": "Impostor",
	}).expect(http.StatusBadRequest).errorMessage()
	if !strings.Contains(body, "already") {
		t.Fatalf("expected a duplicate-email message, got %q", body)
	}
}

func TestListMembers_ShowsEveryStatusUnlikeTheClubList(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	makePending(t)
	makePlayer(t)

	var all []models.User
	h.as(admin).get("/api/admin/users").expect(http.StatusOK).decode(&all)
	if len(all) != 3 {
		t.Fatalf("expected the admin list to include the pending user, got %d", len(all))
	}

	var approved []models.User
	h.as(admin).get("/api/users").expect(http.StatusOK).decode(&approved)
	if len(approved) != 2 {
		t.Fatalf("expected the club list to exclude the pending user, got %d", len(approved))
	}
}

func TestUpdateMember_EditsDetails(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	var updated models.User
	h.as(admin).put("/api/admin/users/"+player.ID.String(), map[string]any{
		"nickname":     "Smash",
		"phone_number": "+61400111222",
	}).expect(http.StatusOK).decode(&updated)

	if updated.Nickname != "Smash" || updated.PhoneNumber != "+61400111222" {
		t.Fatalf("unexpected update result: %+v", updated)
	}
	// An omitted field is left alone rather than blanked.
	if updated.Name != player.Name {
		t.Fatalf("expected the name to survive a partial edit, got %q", updated.Name)
	}
}

func TestUpdateMember_RefusesAnEmailChangeOnASignedInMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	body := h.as(admin).put("/api/admin/users/"+player.ID.String(), map[string]any{
		"email": "hijack@example.com",
	}).expect(http.StatusBadRequest).errorMessage()

	if !strings.Contains(body, "sign-in provider") {
		t.Fatalf("expected the message to explain why, got %q", body)
	}
}

func TestUpdateMember_ValidatesTheRoleAndTheID(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	h.as(admin).put("/api/admin/users/"+player.ID.String(), map[string]any{"role": "wizard"}).
		expect(http.StatusBadRequest)
	h.as(admin).put("/api/admin/users/not-a-uuid", map[string]any{"nickname": "x"}).
		expect(http.StatusBadRequest)
	h.as(admin).put("/api/admin/users/"+uuid.NewString(), map[string]any{"nickname": "x"}).
		expect(http.StatusNotFound)
}

func TestRemoveMember_RevokesAccessWithoutDeleting(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	var removed models.User
	h.as(admin).del("/api/admin/users/" + player.ID.String()).
		expect(http.StatusOK).decode(&removed)

	if removed.MembershipStatus != models.MembershipRemoved {
		t.Fatalf("expected a removed membership, got %s", removed.MembershipStatus)
	}

	// The row is still there to be listed and reinstated.
	var all []models.User
	h.as(admin).get("/api/admin/users").expect(http.StatusOK).decode(&all)
	if len(all) != 2 {
		t.Fatalf("expected the removed member's row to survive, got %d users", len(all))
	}

	var back models.User
	h.as(admin).post("/api/admin/users/"+player.ID.String()+"/reinstate", nil).
		expect(http.StatusOK).decode(&back)
	if !back.IsApproved() {
		t.Fatalf("expected reinstatement to approve, got %s", back.MembershipStatus)
	}
}

func TestRemoveMember_RefusesToRemoveYourself(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	makeAdmin(t) // so the last-admin guard is not what fires

	body := h.as(admin).del("/api/admin/users/" + admin.ID.String()).
		expect(http.StatusBadRequest).errorMessage()
	if !strings.Contains(body, "yourself") {
		t.Fatalf("expected a self-removal message, got %q", body)
	}
}

func TestRemoveMember_ReportsAnUnknownMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).del("/api/admin/users/" + uuid.NewString()).expect(http.StatusNotFound)
	h.as(admin).del("/api/admin/users/not-a-uuid").expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/users/"+uuid.NewString()+"/reinstate", nil).
		expect(http.StatusNotFound)
}

func TestReinstateMember_RefusesAMemberWhoWasNeverRemoved(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	h.as(admin).post("/api/admin/users/"+player.ID.String()+"/reinstate", nil).
		expect(http.StatusBadRequest)
}

// --- self-service nickname -------------------------------------------------

func TestUpdateMe_SetsAndClearsTheMembersOwnNickname(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)
	player.Name = "Priya Raman"
	if err := database.DB.Save(player).Error; err != nil {
		t.Fatalf("failed to name the player: %v", err)
	}

	var updated models.User
	h.as(player).put("/api/users/me", map[string]any{"nickname": "Smash"}).
		expect(http.StatusOK).decode(&updated)
	if updated.Nickname != "Smash" {
		t.Fatalf("expected the nickname to be set, got %q", updated.Nickname)
	}

	// Clearing it is allowed; the first name takes over.
	h.as(player).put("/api/users/me", map[string]any{"nickname": ""}).
		expect(http.StatusOK).decode(&updated)
	if updated.Nickname != "" || updated.DisplayName() != "Priya" {
		t.Fatalf("expected a fall back to the first name, got %q/%q",
			updated.Nickname, updated.DisplayName())
	}
}

func TestUpdateMe_DoesNotBlankTheFieldItWasNotSent(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var updated models.User
	h.as(player).put("/api/users/me", map[string]any{"nickname": "Ace"}).
		expect(http.StatusOK).decode(&updated)
	h.as(player).put("/api/users/me", map[string]any{"phone_number": "+61400000000"}).
		expect(http.StatusOK).decode(&updated)

	if updated.Nickname != "Ace" {
		t.Fatalf("a phone-only save should leave the nickname alone, got %q", updated.Nickname)
	}
	if updated.PhoneNumber != "+61400000000" {
		t.Fatalf("expected the phone number to be saved, got %q", updated.PhoneNumber)
	}
}

func TestUpdateMe_RejectsAnOverlongNickname(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).put("/api/users/me", map[string]any{
		"nickname": strings.Repeat("x", 101),
	}).expect(http.StatusBadRequest)
}
