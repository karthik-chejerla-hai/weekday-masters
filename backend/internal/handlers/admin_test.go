package handlers

import (
	"net/http"
	"testing"
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

// sydneyDate formats a date the way the CreateSession endpoint expects it.
func sydneyDate(daysAhead int) string {
	return utils.NowInSydney().AddDate(0, 0, daysAhead).Format("2006-01-02")
}

// --- join requests --------------------------------------------------------

func TestListJoinRequests_ReturnsPendingUsersOnly(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	pending := makePending(t)
	makePlayer(t)

	var requests []models.User
	h.as(admin).get("/api/admin/join-requests").expect(http.StatusOK).decode(&requests)

	if len(requests) != 1 {
		t.Fatalf("expected 1 pending join request, got %d", len(requests))
	}
	if requests[0].ID != pending.ID {
		t.Fatalf("expected the pending user, got %s", requests[0].ID)
	}
}

func TestApproveJoinRequest_PromotesThePendingUserToPlayer(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	pending := makePending(t)

	var approved models.User
	h.as(admin).post("/api/admin/join-requests/"+pending.ID.String()+"/approve", nil).
		expect(http.StatusOK).decode(&approved)

	if approved.MembershipStatus != models.MembershipApproved {
		t.Fatalf("expected approved membership, got %s", approved.MembershipStatus)
	}
	if approved.Role != models.RolePlayer {
		t.Fatalf("expected the player role, got %s", approved.Role)
	}
}

func TestApproveJoinRequest_RejectsAnInvalidID(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).post("/api/admin/join-requests/not-a-uuid/approve", nil).
		expect(http.StatusBadRequest)
}

func TestRejectJoinRequest_MarksTheUserRejected(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	pending := makePending(t)

	var rejected models.User
	h.as(admin).post("/api/admin/join-requests/"+pending.ID.String()+"/reject", nil).
		expect(http.StatusOK).decode(&rejected)

	if rejected.MembershipStatus != models.MembershipRejected {
		t.Fatalf("expected rejected membership, got %s", rejected.MembershipStatus)
	}
}

func TestUpdateUserRole_PromotesAPlayerToAdmin(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	var promoted models.User
	h.as(admin).put("/api/admin/users/"+player.ID.String()+"/role",
		map[string]string{"role": "admin"}).expect(http.StatusOK).decode(&promoted)

	if promoted.Role != models.RoleAdmin {
		t.Fatalf("expected the admin role, got %s", promoted.Role)
	}
}

func TestUpdateUserRole_RejectsARoleOutsideTheAllowedSet(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	h.as(admin).put("/api/admin/users/"+player.ID.String()+"/role",
		map[string]string{"role": "superuser"}).expect(http.StatusBadRequest)
}

// --- session management ---------------------------------------------------

func TestCreateSession_CalculatesCapacityFromCourts(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	cases := []struct{ courts, want int }{{1, 6}, {2, 10}, {3, 16}}
	for _, tc := range cases {
		var session models.Session
		h.as(admin).post("/api/admin/sessions", map[string]any{
			"title":        "Friday Social",
			"session_date": sydneyDate(14),
			"start_time":   "18:00",
			"end_time":     "20:00",
			"courts":       tc.courts,
		}).expect(http.StatusCreated).decode(&session)

		if session.MaxPlayers != tc.want {
			t.Fatalf("expected %d courts to seat %d, got %d", tc.courts, tc.want, session.MaxPlayers)
		}
	}
}

func TestCreateSession_SetsTheRSVPDeadlineThreeDaysBefore(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	var session models.Session
	h.as(admin).post("/api/admin/sessions", map[string]any{
		"title":        "Friday Social",
		"session_date": sydneyDate(14),
		"start_time":   "18:00",
		"end_time":     "20:00",
		"courts":       2,
	}).expect(http.StatusCreated).decode(&session)

	deadline := session.RSVPDeadline.In(utils.SydneyLocation)
	wantDay := session.SessionDate.In(utils.SydneyLocation).AddDate(0, 0, -3)

	if deadline.Year() != wantDay.Year() || deadline.YearDay() != wantDay.YearDay() {
		t.Fatalf("expected the deadline on %s, got %s", wantDay.Format("2006-01-02"), deadline.Format("2006-01-02"))
	}
	if h, m, sec := deadline.Clock(); h != 23 || m != 59 || sec != 59 {
		t.Fatalf("expected the deadline at end of day Sydney time, got %s", deadline.Format("15:04:05"))
	}
}

func TestCreateSession_AcceptsAnExplicitRSVPDeadline(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	want := utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Second)

	var session models.Session
	h.as(admin).post("/api/admin/sessions", map[string]any{
		"title":         "Friday Social",
		"session_date":  sydneyDate(14),
		"start_time":    "18:00",
		"end_time":      "20:00",
		"courts":        2,
		"rsvp_deadline": want.Format(time.RFC3339),
	}).expect(http.StatusCreated).decode(&session)

	if !session.RSVPDeadline.Equal(want) {
		t.Fatalf("expected the deadline %s, got %s",
			want.Format(time.RFC3339), session.RSVPDeadline.Format(time.RFC3339))
	}
}

func TestCreateSession_RejectsAMalformedRSVPDeadline(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).post("/api/admin/sessions", map[string]any{
		"title":         "Friday Social",
		"session_date":  sydneyDate(14),
		"start_time":    "18:00",
		"end_time":      "20:00",
		"courts":        2,
		"rsvp_deadline": "8 April, sometime after lunch",
	}).expect(http.StatusBadRequest)
}

func TestCreateSession_RejectsAnRSVPDeadlineInThePast(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	past := utils.NowInSydney().AddDate(0, 0, -1)

	h.as(admin).post("/api/admin/sessions", map[string]any{
		"title":         "Friday Social",
		"session_date":  sydneyDate(14),
		"start_time":    "18:00",
		"end_time":      "20:00",
		"courts":        2,
		"rsvp_deadline": past.Format(time.RFC3339),
	}).expect(http.StatusBadRequest)
}

// A session created at short notice has a defaulted deadline that is already
// past. That must not block the admin from creating it.
func TestCreateSession_AllowsShortNoticeWithoutAnExplicitDeadline(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	var session models.Session
	h.as(admin).post("/api/admin/sessions", map[string]any{
		"title":        "Tomorrow Night",
		"session_date": sydneyDate(1),
		"start_time":   "18:00",
		"end_time":     "20:00",
		"courts":       2,
	}).expect(http.StatusCreated).decode(&session)

	if !session.RSVPDeadline.Before(utils.NowInSydney()) {
		t.Fatal("expected the defaulted deadline to be in the past for this fixture")
	}
}

func TestUpdateSession_RejectsAMalformedRSVPDeadline(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 2)

	h.as(admin).put("/api/admin/sessions/"+session.ID.String(), map[string]any{
		"rsvp_deadline": "next Tuesday-ish",
	}).expect(http.StatusBadRequest)
}

func TestUpdateSession_AppliesAnExplicitRSVPDeadline(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 2)

	want := utils.NowInSydney().AddDate(0, 0, 4).Truncate(time.Second)

	var updated models.Session
	h.as(admin).put("/api/admin/sessions/"+session.ID.String(), map[string]any{
		"rsvp_deadline": want.Format(time.RFC3339),
	}).expect(http.StatusOK).decode(&updated)

	if !updated.RSVPDeadline.Equal(want) {
		t.Fatalf("expected the deadline %s, got %s",
			want.Format(time.RFC3339), updated.RSVPDeadline.Format(time.RFC3339))
	}
}

func TestCreateSession_RejectsIncompleteAndOutOfRangeRequests(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	cases := []struct {
		name string
		body map[string]any
	}{
		{
			name: "no title",
			body: map[string]any{"session_date": sydneyDate(14), "start_time": "18:00", "end_time": "20:00", "courts": 2},
		},
		{
			name: "no courts",
			body: map[string]any{"title": "x", "session_date": sydneyDate(14), "start_time": "18:00", "end_time": "20:00"},
		},
		{
			name: "more courts than the venue has",
			body: map[string]any{"title": "x", "session_date": sydneyDate(14), "start_time": "18:00", "end_time": "20:00", "courts": 4},
		},
		{
			name: "date is not YYYY-MM-DD",
			body: map[string]any{"title": "x", "session_date": "12/04/2026", "start_time": "18:00", "end_time": "20:00", "courts": 2},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h.as(admin).post("/api/admin/sessions", tc.body).expect(http.StatusBadRequest)
		})
	}
}

func TestCreateSession_RequiresAnAuthenticatedCreator(t *testing.T) {
	h := newHarness(t)

	h.as(nil).post("/api/admin/sessions", map[string]any{
		"title": "x", "session_date": sydneyDate(14),
		"start_time": "18:00", "end_time": "20:00", "courts": 2,
	}).expect(http.StatusUnauthorized)
}

func TestUpdateSession_ChangesTitleAndRecalculatesCapacity(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 1)

	newTitle := "Renamed"
	var updated models.Session
	h.as(admin).put("/api/admin/sessions/"+session.ID.String(), map[string]any{
		"title":  newTitle,
		"courts": 3,
	}).expect(http.StatusOK).decode(&updated)

	if updated.Title != newTitle {
		t.Fatalf("expected the title to change, got %q", updated.Title)
	}
	if updated.MaxPlayers != 16 {
		t.Fatalf("expected three courts to seat 16, got %d", updated.MaxPlayers)
	}
}

func TestUpdateSession_RejectsAnInvalidID(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).put("/api/admin/sessions/not-a-uuid", map[string]any{"courts": 2}).
		expect(http.StatusBadRequest)
}

func TestCancelSession_MarksItCancelledAndKeepsTheReason(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 2)

	h.as(admin).post("/api/admin/sessions/"+session.ID.String()+"/cancel",
		map[string]string{"reason": "Court flooded"}).expect(http.StatusOK)

	var body struct {
		Session models.Session `json:"session"`
	}
	h.as(admin).get("/api/sessions/" + session.ID.String()).expect(http.StatusOK).decode(&body)

	if body.Session.Status != models.SessionStatusCancelled {
		t.Fatalf("expected the session to be cancelled, got %s", body.Session.Status)
	}
	if body.Session.CancellationReason != "Court flooded" {
		t.Fatalf("expected the reason to be kept, got %q", body.Session.CancellationReason)
	}
}

func TestDeleteSession_RemovesIt(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 2)

	h.as(admin).del("/api/admin/sessions/" + session.ID.String()).expect(http.StatusOK)
	h.as(admin).get("/api/sessions/" + session.ID.String()).expect(http.StatusNotFound)
}

func TestDeleteSession_RejectsAnInvalidID(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).del("/api/admin/sessions/not-a-uuid").expect(http.StatusBadRequest)
}

func TestAddPlayerRSVP_LetsAnAdminRSVPOnSomeoneElsesBehalf(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(admin).post("/api/admin/sessions/"+session.ID.String()+"/rsvp/"+player.ID.String(),
		map[string]any{"status": "in"}).expect(http.StatusOK)

	var rsvp models.RSVP
	h.as(player).get("/api/sessions/" + session.ID.String() + "/rsvp/me").
		expect(http.StatusOK).decode(&rsvp)

	if rsvp.Status != models.RSVPStatusIn {
		t.Fatalf("expected the player to be marked in, got %s", rsvp.Status)
	}
	if !rsvp.AddedByAdmin {
		t.Fatal("expected the RSVP to be flagged as added by an admin")
	}
}

func TestAddPlayerRSVP_RejectsABadStatus(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := makeSession(t, admin.ID, 1)

	h.as(admin).post("/api/admin/sessions/"+session.ID.String()+"/rsvp/"+player.ID.String(),
		map[string]any{"status": "perhaps"}).expect(http.StatusBadRequest)
}

func TestAddPlayerRSVP_RejectsAUserIDThatIsNotAUUID(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	session := makeSession(t, admin.ID, 1)

	h.as(admin).post("/api/admin/sessions/"+session.ID.String()+"/rsvp/not-a-uuid",
		map[string]any{"status": "in"}).expect(http.StatusBadRequest)
}

// --- club -----------------------------------------------------------------

func TestUpdateClub_AppliesOnlyTheFieldsProvided(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	var before models.Club
	if err := database.DB.First(&before).Error; err != nil {
		t.Fatalf("expected the migration to seed a club: %v", err)
	}

	var after models.Club
	h.as(admin).put("/api/admin/club", map[string]any{"venue_name": "New Courts"}).
		expect(http.StatusOK).decode(&after)

	if after.VenueName != "New Courts" {
		t.Fatalf("expected the venue name to change, got %q", after.VenueName)
	}
	// A field that was not in the request body must be left alone.
	if after.Name != before.Name {
		t.Fatalf("expected the club name to be untouched, got %q (was %q)", after.Name, before.Name)
	}
}
