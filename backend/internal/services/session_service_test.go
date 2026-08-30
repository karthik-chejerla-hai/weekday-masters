package services

import (
	"testing"
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

func TestCreateSession_ValidationAndDefaults(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	// Invalid courts (< 1)
	_, err := ss.CreateSession(CreateSessionInput{
		Title:       "Invalid Court Session",
		SessionDate: time.Now().AddDate(0, 0, 7),
		StartTime:   "18:00",
		EndTime:     "20:00",
		Courts:      0,
		CreatedBy:   creator.ID,
	})
	if err == nil {
		t.Fatal("expected error creating session with 0 courts, got nil")
	}

	// Invalid courts (> 3)
	_, err = ss.CreateSession(CreateSessionInput{
		Title:       "Invalid Court Session 2",
		SessionDate: time.Now().AddDate(0, 0, 7),
		StartTime:   "18:00",
		EndTime:     "20:00",
		Courts:      4,
		CreatedBy:   creator.ID,
	})
	if err == nil {
		t.Fatal("expected error creating session with 4 courts, got nil")
	}

	// Valid creation (2 courts = 10 players)
	session, err := ss.CreateSession(CreateSessionInput{
		Title:       "Wednesday Match",
		Description: "Intermediate doubles",
		SessionDate: time.Now().AddDate(0, 0, 5),
		StartTime:   "19:00",
		EndTime:     "21:00",
		Courts:      2,
		CreatedBy:   creator.ID,
	})
	if err != nil {
		t.Fatalf("failed to create valid session: %v", err)
	}

	if session.MaxPlayers != 10 {
		t.Fatalf("expected MaxPlayers 10 for 2 courts, got %d", session.MaxPlayers)
	}
	if session.Status != models.SessionStatusOpen {
		t.Fatalf("expected status open, got %s", session.Status)
	}
}

func TestRecurringSessionGeneration(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	dayOfWeek := int(time.Wednesday)
	occurrences := 3

	session, err := ss.CreateSession(CreateSessionInput{
		Title:              "Weekly Wednesday",
		SessionDate:        time.Now().AddDate(0, 0, 2),
		StartTime:          "18:00",
		EndTime:            "20:00",
		Courts:             1,
		IsRecurring:        true,
		RecurringDayOfWeek: &dayOfWeek,
		Occurrences:        &occurrences,
		CreatedBy:          creator.ID,
	})
	if err != nil {
		t.Fatalf("failed to create recurring session: %v", err)
	}

	// Listing upcoming sessions should include the parent + 2 children = 3 sessions
	sessions, err := ss.ListUpcomingSessions()
	if err != nil {
		t.Fatalf("failed to list upcoming sessions: %v", err)
	}

	var matchCount int
	for _, s := range sessions {
		if s.ID == session.ID || (s.RecurringParentID != nil && *s.RecurringParentID == session.ID) {
			matchCount++
		}
	}

	if matchCount != 3 {
		t.Fatalf("expected 3 total occurrences generated, found %d", matchCount)
	}
}

func TestUpdateSession_CapacityAndDeadline(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	session := newSession(t, ss, creator.ID, 1)
	if session.MaxPlayers != 6 {
		t.Fatalf("expected 6 max players, got %d", session.MaxPlayers)
	}

	newCourts := 3
	newTitle := "Updated Title"
	newDate := time.Now().AddDate(0, 0, 14)

	updated, err := ss.UpdateSession(session.ID, UpdateSessionInput{
		Title:       &newTitle,
		Courts:      &newCourts,
		SessionDate: &newDate,
	})
	if err != nil {
		t.Fatalf("failed to update session: %v", err)
	}

	if updated.Title != newTitle {
		t.Fatalf("expected title %q, got %q", newTitle, updated.Title)
	}
	if updated.Courts != 3 || updated.MaxPlayers != 16 {
		t.Fatalf("expected 3 courts and 16 max players, got %d courts and %d max players", updated.Courts, updated.MaxPlayers)
	}
}

func TestCancelSessionAndListing(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	session := newSession(t, ss, creator.ID, 1)

	cancelled, err := ss.CancelSession(session.ID, "Venue closed for maintenance")
	if err != nil {
		t.Fatalf("failed to cancel session: %v", err)
	}

	if cancelled.Status != models.SessionStatusCancelled {
		t.Fatalf("expected status cancelled, got %s", cancelled.Status)
	}
	if cancelled.CancellationReason != "Venue closed for maintenance" {
		t.Fatalf("expected reason stored, got %q", cancelled.CancellationReason)
	}

	// Should show in ListCancelledUpcomingSessions
	cancelledList, err := ss.ListCancelledUpcomingSessions()
	if err != nil {
		t.Fatalf("failed to list cancelled sessions: %v", err)
	}
	if len(cancelledList) != 1 || cancelledList[0].ID != session.ID {
		t.Fatalf("expected 1 cancelled session in list, got %d", len(cancelledList))
	}
}

func TestDeleteSession_WithAndWithoutRSVPs(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	player := newUser(t, "player")

	// 1. Session without RSVPs should be hard deleted
	session1 := newSession(t, ss, creator.ID, 1)
	if err := ss.DeleteSession(session1.ID); err != nil {
		t.Fatalf("failed to delete empty session: %v", err)
	}

	_, err := ss.GetSessionByID(session1.ID)
	if err == nil {
		t.Fatal("expected error retrieving deleted session, got nil")
	}

	// 2. Session with RSVPs should be soft-cancelled instead of hard-deleted
	session2 := newSession(t, ss, creator.ID, 1)
	rsvpIn(t, rs, session2.ID, player.ID)

	if err := ss.DeleteSession(session2.ID); err != nil {
		t.Fatalf("failed to delete session with rsvps: %v", err)
	}

	found, err := ss.GetSessionByID(session2.ID)
	if err != nil {
		t.Fatalf("session with rsvps should still exist: %v", err)
	}
	if found.Status != models.SessionStatusCancelled {
		t.Fatalf("expected session to be marked cancelled, got %s", found.Status)
	}
}

func TestCreateSession_DefaultDeadlineAllowsShortNotice(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	// Two days out: the 3-day default deadline lands in the past. Admins must
	// still be able to create the session.
	session, err := ss.CreateSession(CreateSessionInput{
		Title:       "Short Notice",
		SessionDate: time.Now().AddDate(0, 0, 2),
		StartTime:   "18:00",
		EndTime:     "20:00",
		Courts:      1,
		CreatedBy:   creator.ID,
	})
	if err != nil {
		t.Fatalf("expected short-notice session to be created, got %v", err)
	}
	if !session.RSVPDeadline.Before(utils.NowInSydney()) {
		t.Fatal("expected the defaulted deadline to be in the past for this fixture")
	}
}

func TestCreateSession_CustomDeadline(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	sessionDate := time.Now().AddDate(0, 0, 10)
	custom := utils.NowInSydney().AddDate(0, 0, 5).Truncate(time.Second)

	session, err := ss.CreateSession(CreateSessionInput{
		Title:        "Custom Deadline",
		SessionDate:  sessionDate,
		StartTime:    "18:00",
		EndTime:      "20:00",
		Courts:       1,
		CreatedBy:    creator.ID,
		RSVPDeadline: &custom,
	})
	if err != nil {
		t.Fatalf("failed to create session with custom deadline: %v", err)
	}
	if !session.RSVPDeadline.Equal(custom) {
		t.Fatalf("expected deadline %v, got %v", custom, session.RSVPDeadline)
	}

	// The 3-day default must not have been applied.
	if session.RSVPDeadline.Equal(utils.CalculateRSVPDeadline(sessionDate)) {
		t.Fatal("custom deadline was overwritten by the default")
	}
}

func TestCreateSession_RejectsExplicitPastDeadline(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	past := utils.NowInSydney().AddDate(0, 0, -1)
	_, err := ss.CreateSession(CreateSessionInput{
		Title:        "Past Deadline",
		SessionDate:  time.Now().AddDate(0, 0, 10),
		StartTime:    "18:00",
		EndTime:      "20:00",
		Courts:       1,
		CreatedBy:    creator.ID,
		RSVPDeadline: &past,
	})
	if err == nil {
		t.Fatal("expected an error for an explicitly past deadline, got nil")
	}
}

func TestRecurringSessions_InheritCustomDeadlineOffset(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	// Parent sits 14 days out with a deadline 5 days before it at 18:00 Sydney.
	parentDate := time.Now().In(utils.SydneyLocation).AddDate(0, 0, 14)
	deadlineDay := parentDate.AddDate(0, 0, -5)
	custom := time.Date(
		deadlineDay.Year(), deadlineDay.Month(), deadlineDay.Day(),
		18, 0, 0, 0, utils.SydneyLocation,
	)

	dayOfWeek := int(parentDate.Weekday())
	occurrences := 3

	parent, err := ss.CreateSession(CreateSessionInput{
		Title:              "Weekly With Custom Deadline",
		SessionDate:        parentDate,
		StartTime:          "18:00",
		EndTime:            "20:00",
		Courts:             1,
		IsRecurring:        true,
		RecurringDayOfWeek: &dayOfWeek,
		Occurrences:        &occurrences,
		CreatedBy:          creator.ID,
		RSVPDeadline:       &custom,
	})
	if err != nil {
		t.Fatalf("failed to create recurring session: %v", err)
	}

	var children []models.Session
	if err := database.DB.
		Where("recurring_parent_id = ?", parent.ID).
		Order("session_date asc").
		Find(&children).Error; err != nil {
		t.Fatalf("failed to load children: %v", err)
	}
	if len(children) != 2 {
		t.Fatalf("expected 2 child sessions, got %d", len(children))
	}

	for _, child := range children {
		childDate := child.SessionDate.In(utils.SydneyLocation)
		deadline := child.RSVPDeadline.In(utils.SydneyLocation)

		// Same 5-day offset, same 18:00 wall-clock time as the parent.
		wantDay := time.Date(childDate.Year(), childDate.Month(), childDate.Day(), 0, 0, 0, 0, time.UTC).
			AddDate(0, 0, -5)
		gotDay := time.Date(deadline.Year(), deadline.Month(), deadline.Day(), 0, 0, 0, 0, time.UTC)

		if !gotDay.Equal(wantDay) {
			t.Fatalf("child %s: expected deadline day %v, got %v",
				child.SessionDate.Format("2006-01-02"), wantDay.Format("2006-01-02"), gotDay.Format("2006-01-02"))
		}
		if deadline.Hour() != 18 || deadline.Minute() != 0 {
			t.Fatalf("child %s: expected deadline at 18:00 Sydney, got %02d:%02d",
				child.SessionDate.Format("2006-01-02"), deadline.Hour(), deadline.Minute())
		}
	}
}

func TestUpdateSession_DeadlineRules(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")

	session := newSession(t, ss, creator.ID, 2)
	original := session.RSVPDeadline

	// Updating an unrelated field leaves the deadline alone.
	desc := "Updated description"
	updated, err := ss.UpdateSession(session.ID, UpdateSessionInput{Description: &desc})
	if err != nil {
		t.Fatalf("failed to update description: %v", err)
	}
	if !updated.RSVPDeadline.Equal(original) {
		t.Fatalf("expected deadline unchanged, got %v (was %v)", updated.RSVPDeadline, original)
	}

	// An explicit future deadline is applied.
	future := utils.NowInSydney().AddDate(0, 0, 3).Truncate(time.Second)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{RSVPDeadline: &future})
	if err != nil {
		t.Fatalf("failed to update deadline: %v", err)
	}
	if !updated.RSVPDeadline.Equal(future) {
		t.Fatalf("expected deadline %v, got %v", future, updated.RSVPDeadline)
	}

	// An explicit past deadline is rejected.
	past := utils.NowInSydney().AddDate(0, 0, -1)
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{RSVPDeadline: &past}); err == nil {
		t.Fatal("expected an error for an explicitly past deadline, got nil")
	}

	// Moving the date with an explicit deadline keeps the explicit one.
	newDate := time.Now().AddDate(0, 0, 21)
	keep := utils.NowInSydney().AddDate(0, 0, 9).Truncate(time.Second)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &newDate, RSVPDeadline: &keep})
	if err != nil {
		t.Fatalf("failed to update date and deadline: %v", err)
	}
	if !updated.RSVPDeadline.Equal(keep) {
		t.Fatalf("expected explicit deadline %v to survive a date change, got %v", keep, updated.RSVPDeadline)
	}

	// Moving the date without one recalculates from the default.
	laterDate := time.Now().AddDate(0, 0, 28)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &laterDate})
	if err != nil {
		t.Fatalf("failed to update date: %v", err)
	}
	if !updated.RSVPDeadline.Equal(utils.CalculateRSVPDeadline(laterDate)) {
		t.Fatalf("expected recalculated default deadline, got %v", updated.RSVPDeadline)
	}
}

// "Upcoming" has to mean "has not finished", not "is dated today or later".
// Tonight's session should drop off the upcoming list once it ends, otherwise
// the History tab never receives it and the dashboard keeps advertising a game
// that already happened.
func TestListSessionsExcludesSessionsThatHaveFinished(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")

	today := utils.NowInSydney()

	finished, err := ss.CreateSession(CreateSessionInput{
		Title:       "Finished earlier today",
		SessionDate: today,
		StartTime:   "06:00",
		EndTime:     "08:00",
		Courts:      1,
		CreatedBy:   admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	stillToCome, err := ss.CreateSession(CreateSessionInput{
		Title:       "Next week",
		SessionDate: today.AddDate(0, 0, 7),
		StartTime:   "20:00",
		EndTime:     "22:00",
		Courts:      1,
		CreatedBy:   admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Guard the premise: if this test ever runs before 8am the fixture is not
	// actually in the past and the assertion below would be vacuous.
	if today.Hour() < 9 {
		t.Skip("runs before 09:00 Sydney, so the 06:00-08:00 fixture is not yet in the past")
	}

	sessions, err := ss.ListUpcomingSessions()
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range sessions {
		if s.ID == finished.ID {
			t.Errorf("a session that ended at 08:00 is still listed as upcoming")
		}
	}

	var foundFuture bool
	for _, s := range sessions {
		if s.ID == stillToCome.ID {
			foundFuture = true
		}
	}
	if !foundFuture {
		t.Error("next week's session is missing from the upcoming list")
	}
}

// The resolved timestamps must be populated by the model hook, not left for
// callers to remember.
func TestCreateSessionResolvesTimestamps(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")

	session, err := ss.CreateSession(CreateSessionInput{
		Title:       "Tuesday Social",
		SessionDate: utils.NowInSydney().AddDate(0, 0, 7),
		StartTime:   "20:00",
		EndTime:     "22:00",
		Courts:      1,
		CreatedBy:   admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if session.StartsAt == nil || session.EndsAt == nil {
		t.Fatal("session was written without resolved start and end instants")
	}
	if got := session.EndsAt.Sub(*session.StartsAt); got != 2*time.Hour {
		t.Errorf("resolved duration = %s, want 2h", got)
	}
	if got := session.StartsAt.In(utils.SydneyLocation).Format("15:04"); got != "20:00" {
		t.Errorf("resolved start reads %s in Sydney, want 20:00", got)
	}
}
