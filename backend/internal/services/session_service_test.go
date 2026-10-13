package services

import (
	"encoding/json"
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

	cancelled, err := ss.CancelSession(session.ID, "Venue closed for maintenance", creator.ID)
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

	// 2. Sessions with RSVPs require explicit cancellation and an announcement.
	session2 := newSession(t, ss, creator.ID, 1)
	rsvpIn(t, rs, session2.ID, player.ID)

	if err := ss.DeleteSession(session2.ID); err == nil {
		t.Fatal("expected deletion to require the cancel action")
	}

	found, err := ss.GetSessionByID(session2.ID)
	if err != nil {
		t.Fatalf("session with rsvps should still exist: %v", err)
	}
	if found.Status != models.SessionStatusOpen {
		t.Fatalf("expected session to remain open, got %s", found.Status)
	}
}

func TestDeleteSession_RemovesReminderStagesAndSuppressesPendingIntents(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	session := newSession(t, ss, creator.ID, 1)
	now := utils.NowInSydney()
	stage := models.RSVPReminderStage{SessionID: session.ID, Stage: 1, State: models.RSVPReminderPending, StartsAt: *session.StartsAt, DueAt: now, ExpiresAt: session.RSVPDeadline}
	if err := database.DB.Create(&stage).Error; err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]string{"type": "rsvp_reminder", "session_id": session.ID.String()})
	notification := models.Notification{UserID: creator.ID, NotificationType: models.NotificationRSVPReminder, Title: "RSVP", Body: "Please respond", Data: string(data)}
	if err := database.DB.Create(&notification).Error; err != nil {
		t.Fatal(err)
	}
	outbox := models.NotificationOutbox{NotificationID: notification.ID, EventKey: "rsvp-reminder-test", DueAt: now, ExpiresAt: now.Add(time.Hour), State: models.DeliveryPending, NextAttemptAt: now}
	if err := database.DB.Create(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if err := ss.DeleteSession(session.ID); err != nil {
		t.Fatalf("delete should clean up reminder rows: %v", err)
	}
	var stageCount, notificationCount int64
	database.DB.Model(&models.RSVPReminderStage{}).Where("session_id = ?", session.ID).Count(&stageCount)
	database.DB.Model(&models.Notification{}).Where("id = ?", notification.ID).Count(&notificationCount)
	var persisted models.NotificationOutbox
	if err := database.DB.First(&persisted, "id = ?", outbox.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stageCount != 0 || notificationCount != 1 || persisted.State != models.DeliverySuppressed {
		t.Fatalf("expected stages removed, history retained and intent suppressed; stages=%d history=%d state=%s", stageCount, notificationCount, persisted.State)
	}
}

func TestCreateSession_DefaultDeadlineAllowsShortNotice(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	session, err := ss.CreateSession(CreateSessionInput{Title: "Short notice", SessionDate: utils.NowInSydney().AddDate(0, 0, 1), StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID})
	if err != nil {
		t.Fatal(err)
	}
	if session.StartsAt.Sub(session.RSVPDeadline) != 48*time.Hour || !session.RSVPDeadline.Before(utils.NowInSydney()) {
		t.Fatal("short-notice default must remain fixed and already closed")
	}
}
func TestSessionRejectsObsoleteCustomDeadlineInput(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	deadline := utils.NowInSydney().AddDate(0, 0, 4)
	if _, err := ss.CreateSession(CreateSessionInput{Title: "Custom", SessionDate: utils.NowInSydney().AddDate(0, 0, 10), StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID, RSVPDeadline: &deadline}); err != ErrFixedRSVPDeadline {
		t.Fatalf("obsolete create input accepted: %v", err)
	}
	session := newSession(t, ss, creator.ID, 1)
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{RSVPDeadline: &deadline}); err != ErrFixedRSVPDeadline {
		t.Fatalf("obsolete update input accepted: %v", err)
	}
}
func TestRecurringSessionsUseResolved48HourDeadlineAcrossDST(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	for _, date := range []string{"2026-09-28", "2026-03-30"} {
		day, err := utils.ParseDateInSydney(date)
		if err != nil {
			t.Fatal(err)
		}
		weekday := int(day.Weekday())
		occurrences := 3
		parent, err := ss.CreateSession(CreateSessionInput{Title: "DST recurring", SessionDate: day, StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID, IsRecurring: true, RecurringDayOfWeek: &weekday, Occurrences: &occurrences})
		if err != nil {
			t.Fatal(err)
		}
		var sessions []models.Session
		if err := database.DB.Where("id = ? OR recurring_parent_id = ?", parent.ID, parent.ID).Find(&sessions).Error; err != nil {
			t.Fatal(err)
		}
		if len(sessions) != 3 {
			t.Fatalf("expected 3 occurrences, got %d", len(sessions))
		}
		for _, session := range sessions {
			if session.StartsAt.Sub(session.RSVPDeadline) != 48*time.Hour {
				t.Fatalf("DST recurrence deadline drifted: %+v", session)
			}
		}
	}
}
func TestUpdateSessionDeadlineTracksDateAndStartOnlyWhileWindowOpen(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, utils.SydneyLocation)
	ss.now = func() time.Time { return now }
	day := now.AddDate(0, 0, 10)
	session, err := ss.CreateSession(CreateSessionInput{Title: "Open", SessionDate: day, StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID})
	if err != nil {
		t.Fatal(err)
	}
	description := "Edited"
	updated, err := ss.UpdateSession(session.ID, UpdateSessionInput{Description: &description})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.RSVPDeadline.Equal(session.RSVPDeadline) {
		t.Fatal("description changed deadline")
	}
	start := "19:00"
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{StartTime: &start})
	if err != nil {
		t.Fatal(err)
	}
	if updated.StartsAt.Sub(updated.RSVPDeadline) != 48*time.Hour || !updated.RSVPDeadline.Equal(session.RSVPDeadline.Add(time.Hour)) {
		t.Fatal("start edit did not recalculate deadline")
	}
	later := day.AddDate(0, 0, 7)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &later})
	if err != nil {
		t.Fatal(err)
	}
	if updated.StartsAt.Sub(updated.RSVPDeadline) != 48*time.Hour {
		t.Fatal("date edit did not recalculate deadline")
	}
	// Moving an open session to short notice legitimately closes its new window.
	short := now.AddDate(0, 0, 1)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &short})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.RSVPDeadline.Before(now) {
		t.Fatal("short-notice edit should close window")
	}
	originalClosed := updated.RSVPDeadline
	later = now.AddDate(0, 0, 30)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &later})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.RSVPDeadline.Equal(originalClosed) {
		t.Fatal("later edit reopened closed RSVP window")
	}
}
func TestUpdateSessionRetainsLegacyClosedDeadline(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	now := time.Date(2030, 1, 1, 12, 0, 0, 0, utils.SydneyLocation)
	ss.now = func() time.Time { return now }
	day := now.AddDate(0, 0, 10)
	session, err := ss.CreateSession(CreateSessionInput{Title: "Closed legacy", SessionDate: day, StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: creator.ID})
	if err != nil {
		t.Fatal(err)
	}
	closed := now.Add(-time.Hour)
	if err := database.DB.Model(session).UpdateColumn("rsvp_deadline", closed).Error; err != nil {
		t.Fatal(err)
	}
	later := day.AddDate(0, 0, 7)
	start := "19:00"
	updated, err := ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &later, StartTime: &start})
	if err != nil {
		t.Fatal(err)
	}
	if !updated.RSVPDeadline.Equal(closed) {
		t.Fatal("legacy closed window reopened")
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

func TestUpdateSessionCannotReopenClosedStatusWindow(t *testing.T) {
	_, ss, _ := newTestServices(t)
	creator := newUser(t, "admin")
	session := newSession(t, ss, creator.ID, 1)
	closed := models.SessionStatusClosed
	if err := database.DB.Model(session).UpdateColumn("status", closed).Error; err != nil {
		t.Fatal(err)
	}
	open := models.SessionStatusOpen
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{Status: &open}); err == nil {
		t.Fatal("closed status window reopened")
	}
}
