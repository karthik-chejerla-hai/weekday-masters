package services

import (
	"sync"
	"testing"
	"time"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

func TestClubDeadlineChangeRollsBackAllSettingsWhenSessionUpdateFails(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	var before models.Club
	if err := database.DB.First(&before).Error; err != nil {
		t.Fatal(err)
	}
	// Reject the new deadline at the database boundary, after the save begins.
	if err := database.DB.Exec("ALTER TABLE sessions ADD CONSTRAINT test_deadline_reject CHECK (rsvp_deadline < starts_at)").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.DB.Exec("ALTER TABLE sessions DROP CONSTRAINT test_deadline_reject") })
	if _, err := UpdateClubSettings(map[string]any{"rsvp_deadline_hours": 0, "name": "Must roll back"}); err == nil {
		t.Fatal("expected failed session update")
	}
	var after models.Club
	if err := database.DB.First(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after.Name != before.Name || after.RSVPDeadlineHours != before.RSVPDeadlineHours {
		t.Fatal("partial club settings survived rollback")
	}
	var saved models.Session
	if err := database.DB.First(&saved, "id = ?", session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.RSVPDeadline.Equal(session.RSVPDeadline) {
		t.Fatal("deadline changed after failed transaction")
	}
}

func TestClubDefaultChangeSerializesWithSessionCreation(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	start := make(chan struct{})
	errs := make(chan error, 9)
	var workers sync.WaitGroup
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			_, err := ss.CreateSession(CreateSessionInput{Title: "Concurrent session", SessionDate: utils.NowInSydney().AddDate(0, 0, 14), StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: admin.ID})
			errs <- err
		}()
	}
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		_, err := UpdateClubSettings(map[string]any{"rsvp_deadline_hours": 36})
		errs <- err
	}()
	close(start)
	workers.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	var sessions []models.Session
	if err := database.DB.Find(&sessions).Error; err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 8 {
		t.Fatalf("expected eight sessions, got %d", len(sessions))
	}
	for _, session := range sessions {
		if session.StartsAt.Sub(session.RSVPDeadline) != 36*time.Hour {
			t.Fatal("concurrent create missed default change")
		}
	}
}

func TestClubDefaultAppliesToRefreshAfterCustomParentHasStarted(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := utils.NowInSydney().AddDate(0, 0, -1)
	day := int(date.Weekday())
	parent := models.Session{Title: "Old custom series", SessionDate: date, StartTime: "18:00", EndTime: "20:00", Courts: 1,
		RSVPDeadline: date.AddDate(0, 0, -2), IsRecurring: true, RecurringDayOfWeek: &day, Status: models.SessionStatusOpen, CreatedBy: admin.ID}
	custom := parent.RSVPDeadline
	parent.RecurringRSVPDeadline = &custom
	if err := database.DB.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateClubSettings(map[string]any{"rsvp_deadline_hours": 24}); err != nil {
		t.Fatal(err)
	}
	if err := ss.RefreshRecurringSessions(); err != nil {
		t.Fatal(err)
	}
	var children []models.Session
	if err := database.DB.Where("recurring_parent_id = ?", parent.ID).Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("expected three children, got %d", len(children))
	}
	for _, child := range children {
		if child.StartsAt.Sub(child.RSVPDeadline) != 24*time.Hour {
			t.Fatal("refreshed child used old custom rule")
		}
	}
	var saved models.Session
	if err := database.DB.First(&saved, "id = ?", parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.RSVPDeadline.Equal(parent.RSVPDeadline) {
		t.Fatal("past parent deadline changed")
	}
}

func TestRSVPSettingsMigrationPreservesLegacyDeadlines(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	if err := database.DB.Model(session).UpdateColumn("is_recurring", true).Error; err != nil {
		t.Fatal(err)
	}
	// Emulate the schema before configurable deadlines existed.
	if err := database.DB.Exec("ALTER TABLE sessions DROP COLUMN recurring_rsvp_deadline").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Exec("ALTER TABLE clubs DROP COLUMN rsvp_deadline_hours").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := database.Migrate(); err != nil {
			t.Error(err)
		}
	})
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	var saved models.Session
	if err := database.DB.First(&saved, "id = ?", session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !saved.RSVPDeadline.Equal(session.RSVPDeadline) {
		t.Fatal("migration changed a legacy deadline")
	}
	if saved.RecurringRSVPDeadline == nil || !saved.RecurringRSVPDeadline.Equal(session.RSVPDeadline) {
		t.Fatal("migration lost the legacy recurring rule")
	}
	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		t.Fatal(err)
	}
	if club.RSVPDeadlineHours != 72 {
		t.Fatal("migration did not initialize default")
	}
	if _, err := UpdateClubSettings(map[string]any{"rsvp_deadline_hours": 24}); err != nil {
		t.Fatal(err)
	}
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	club = models.Club{}
	if err := database.DB.First(&club).Error; err != nil {
		t.Fatal(err)
	}
	if club.RSVPDeadlineHours != 24 {
		t.Fatal("repeat migration reset chosen default")
	}
}

func TestEditingParentDeadlineDoesNotChangeLaterRecurringDates(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := utils.NowInSydney().AddDate(0, 0, 14)
	day, occurrences := int(date.Weekday()), 1
	parent, err := ss.CreateSession(CreateSessionInput{Title: "One initial occurrence", SessionDate: date, StartTime: "18:00", EndTime: "20:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	custom := parent.StartsAt.Add(-24 * time.Hour)
	if _, err := ss.UpdateSession(parent.ID, UpdateSessionInput{RSVPDeadline: &custom}); err != nil {
		t.Fatal(err)
	}
	if err := ss.RefreshRecurringSessions(); err != nil {
		t.Fatal(err)
	}
	var children []models.Session
	if err := database.DB.Where("recurring_parent_id = ?", parent.ID).Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("got %d children", len(children))
	}
	for _, child := range children {
		if child.StartsAt.Sub(child.RSVPDeadline) != 72*time.Hour {
			t.Fatal("one-session edit changed later recurring dates")
		}
	}
}

func TestEditingCustomParentDeadlinePreservesTheOriginalRecurringRule(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := utils.NowInSydney().AddDate(0, 0, 14)
	original := time.Date(date.Year(), date.Month(), date.Day()-5, 18, 0, 0, 0, utils.SydneyLocation)
	day, occurrences := int(date.Weekday()), 1
	parent, err := ss.CreateSession(CreateSessionInput{Title: "Custom series", SessionDate: date, StartTime: "18:00", EndTime: "20:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &original})
	if err != nil {
		t.Fatal(err)
	}
	edited := parent.StartsAt.Add(-24 * time.Hour)
	if _, err := ss.UpdateSession(parent.ID, UpdateSessionInput{RSVPDeadline: &edited}); err != nil {
		t.Fatal(err)
	}
	if err := ss.RefreshRecurringSessions(); err != nil {
		t.Fatal(err)
	}
	var children []models.Session
	if err := database.DB.Where("recurring_parent_id = ?", parent.ID).Order("session_date").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("got %d children", len(children))
	}
	for i, child := range children {
		if !child.RSVPDeadline.Equal(original.AddDate(0, 0, 7*(i+1))) {
			t.Fatal("individual edit replaced original recurring rule")
		}
	}
}

func TestMovingCustomParentPreservesItsRecurringCalendarOffset(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := utils.NowInSydney().AddDate(0, 0, 14)
	original := time.Date(date.Year(), date.Month(), date.Day()-5, 18, 0, 0, 0, utils.SydneyLocation)
	day, occurrences := int(date.Weekday()), 1
	parent, err := ss.CreateSession(CreateSessionInput{Title: "Moved series", SessionDate: date, StartTime: "18:00", EndTime: "20:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &original})
	if err != nil {
		t.Fatal(err)
	}
	moved := date.AddDate(0, 0, 7)
	if _, err := ss.UpdateSession(parent.ID, UpdateSessionInput{SessionDate: &moved}); err != nil {
		t.Fatal(err)
	}
	if err := ss.RefreshRecurringSessions(); err != nil {
		t.Fatal(err)
	}
	var children []models.Session
	if err := database.DB.Where("recurring_parent_id = ?", parent.ID).Order("session_date").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("got %d children", len(children))
	}
	for i, child := range children {
		if !child.RSVPDeadline.Equal(original.AddDate(0, 0, 7*(i+2))) {
			t.Fatal("moving parent changed the custom calendar offset")
		}
	}
}
