package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

// --- users ----------------------------------------------------------------

func TestGetMe_ReturnsTheAuthenticatedUser(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var got models.User
	h.as(player).get("/api/users/me").expect(http.StatusOK).decode(&got)

	if got.ID != player.ID {
		t.Fatalf("expected user %s, got %s", player.ID, got.ID)
	}
	if got.Email != player.Email {
		t.Fatalf("expected email %s, got %s", player.Email, got.Email)
	}
}

func TestGetMe_RequiresAnAuthenticatedUser(t *testing.T) {
	h := newHarness(t)

	h.as(nil).get("/api/users/me").expect(http.StatusUnauthorized)
}

func TestUpdateMe_SavesThePhoneNumber(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var got models.User
	h.as(player).
		put("/api/users/me", map[string]string{"phone_number": "+61412345678"}).
		expect(http.StatusOK).
		decode(&got)

	if got.PhoneNumber != "+61412345678" {
		t.Fatalf("expected the phone number to be saved, got %q", got.PhoneNumber)
	}
}

func TestUpdateMe_RejectsAnUnauthenticatedRequestBeforeReadingTheBody(t *testing.T) {
	h := newHarness(t)

	h.as(nil).put("/api/users/me", map[string]string{"phone_number": "+61412345678"}).
		expect(http.StatusUnauthorized)
}

func TestUpdateMe_RejectsAMalformedBody(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	// phone_number must be a string; a number is a bind failure, not a 500.
	h.as(player).put("/api/users/me", map[string]any{"phone_number": 12345}).
		expect(http.StatusBadRequest)
}

func TestListMembers_ReturnsOnlyApprovedMembers(t *testing.T) {
	h := newHarness(t)
	approved := makePlayer(t)
	makePending(t)

	var members []models.User
	h.as(approved).get("/api/users").expect(http.StatusOK).decode(&members)

	if len(members) != 1 {
		t.Fatalf("expected 1 approved member, got %d", len(members))
	}
	if members[0].ID != approved.ID {
		t.Fatalf("expected member %s, got %s", approved.ID, members[0].ID)
	}
}

// --- sessions -------------------------------------------------------------

func TestListSessions_ReturnsUpcomingSessions(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	makeSession(t, admin.ID, 2)

	var sessions []models.Session
	h.as(admin).get("/api/sessions").expect(http.StatusOK).decode(&sessions)

	if len(sessions) != 1 {
		t.Fatalf("expected 1 upcoming session, got %d", len(sessions))
	}
}

func TestListSessions_ReturnsAnEmptyListRatherThanAnError(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var sessions []models.Session
	h.as(player).get("/api/sessions").expect(http.StatusOK).decode(&sessions)

	if len(sessions) != 0 {
		t.Fatalf("expected no sessions, got %d", len(sessions))
	}
}

func TestGetSession_ReturnsTheSessionWithItsRSVPSummary(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 1)

	var body struct {
		Session     models.Session `json:"session"`
		RSVPSummary struct {
			MaxPlayers int `json:"max_players"`
		} `json:"rsvp_summary"`
	}
	h.as(admin).get("/api/sessions/" + session.ID.String()).expect(http.StatusOK).decode(&body)

	if body.Session.ID != session.ID {
		t.Fatalf("expected session %s, got %s", session.ID, body.Session.ID)
	}
	// One court seats six.
	if body.RSVPSummary.MaxPlayers != 6 {
		t.Fatalf("expected a 6-player cap for one court, got %d", body.RSVPSummary.MaxPlayers)
	}
}

func TestGetSession_RejectsAnIDThatIsNotAUUID(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).get("/api/sessions/not-a-uuid").expect(http.StatusBadRequest)
}

func TestGetSession_IsNotFoundForAnUnknownID(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).get("/api/sessions/" + uuid.NewString()).expect(http.StatusNotFound)
}

func TestListCancelledSessions_ReturnsCancelledOnes(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 2)

	h.as(admin).post("/api/admin/sessions/"+session.ID.String()+"/cancel",
		map[string]string{"reason": "Court flooded"}).expect(http.StatusOK)

	var cancelled []models.Session
	h.as(admin).get("/api/sessions/cancelled").expect(http.StatusOK).decode(&cancelled)

	if len(cancelled) != 1 {
		t.Fatalf("expected 1 cancelled session, got %d", len(cancelled))
	}

	// And it should no longer show up as upcoming.
	var upcoming []models.Session
	h.as(admin).get("/api/sessions").expect(http.StatusOK).decode(&upcoming)
	if len(upcoming) != 0 {
		t.Fatalf("expected a cancelled session to drop out of the upcoming list, got %d", len(upcoming))
	}
}

// --- RSVPs ----------------------------------------------------------------

func TestCreateRSVP_RecordsTheResponse(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	var rsvp models.RSVP
	h.as(player).post("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "in"}).expect(http.StatusOK).decode(&rsvp)

	if rsvp.Status != models.RSVPStatusIn {
		t.Fatalf("expected status in, got %s", rsvp.Status)
	}
	if rsvp.UserID != player.ID {
		t.Fatalf("expected the RSVP to belong to the caller, got %s", rsvp.UserID)
	}
}

func TestCreateRSVP_RejectsAStatusOutsideTheAllowedSet(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(player).post("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "perhaps"}).expect(http.StatusBadRequest)
}

func TestCreateRSVP_RejectsAnInvalidSessionID(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).post("/api/sessions/not-a-uuid/rsvp",
		map[string]string{"status": "in"}).expect(http.StatusBadRequest)
}

func TestCreateRSVP_RequiresAuthentication(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 1)

	h.as(nil).post("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "in"}).expect(http.StatusUnauthorized)
}

func TestUpdateRSVP_ChangesAnExistingResponse(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(player).post("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "in"}).expect(http.StatusOK)

	var updated models.RSVP
	h.as(player).put("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "maybe"}).expect(http.StatusOK).decode(&updated)

	if updated.Status != models.RSVPStatusMaybe {
		t.Fatalf("expected status maybe, got %s", updated.Status)
	}
}

func TestGetMyRSVP_ReturnsTheCallersOwnResponse(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(player).post("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "in"}).expect(http.StatusOK)

	var rsvp models.RSVP
	h.as(player).get("/api/sessions/" + session.ID.String() + "/rsvp/me").
		expect(http.StatusOK).decode(&rsvp)

	if rsvp.UserID != player.ID {
		t.Fatalf("expected the caller's RSVP, got one for %s", rsvp.UserID)
	}
}

func TestGetMyRSVP_IsNotFoundBeforeTheCallerHasResponded(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(player).get("/api/sessions/" + session.ID.String() + "/rsvp/me").
		expect(http.StatusNotFound)
}

func TestDeleteRSVP_RemovesTheResponse(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(player).post("/api/sessions/"+session.ID.String()+"/rsvp",
		map[string]string{"status": "in"}).expect(http.StatusOK)

	h.as(player).del("/api/sessions/" + session.ID.String() + "/rsvp").expect(http.StatusOK)

	h.as(player).get("/api/sessions/" + session.ID.String() + "/rsvp/me").
		expect(http.StatusNotFound)
}

func TestDeleteRSVP_RejectsAnInvalidSessionID(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).del("/api/sessions/not-a-uuid/rsvp").expect(http.StatusBadRequest)
}
