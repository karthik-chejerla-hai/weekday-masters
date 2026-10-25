package services

import (
	"errors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"net/url"
	"os"
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
	date := utils.NowInSydney().AddDate(0, 0, -1).Truncate(time.Microsecond)
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
	requireDB(t)
	originalDB := database.DB
	// Schema changes invalidate cached SELECT * plans. Give this test its own
	// schema and connection, as the production migration process has.
	if err := originalDB.Exec("CREATE SCHEMA rsvp_deadline_migration_test").Error; err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if database.DB != originalDB {
			if pool, err := database.DB.DB(); err != nil {
				t.Error(err)
			} else if err := pool.Close(); err != nil {
				t.Error(err)
			}
		}
		database.DB = originalDB
		if err := originalDB.Exec("DROP SCHEMA rsvp_deadline_migration_test CASCADE").Error; err != nil {
			t.Error(err)
		}
	})
	dsn, err := url.Parse(os.Getenv("TEST_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	query := dsn.Query()
	query.Set("search_path", "rsvp_deadline_migration_test,public")
	dsn.RawQuery = query.Encode()
	isolated, err := gorm.Open(postgres.New(postgres.Config{DSN: dsn.String(), PreferSimpleProtocol: true}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	database.DB = isolated
	if err := database.Migrate(); err != nil {
		t.Fatal(err)
	}
	ss := NewSessionService()
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
	if club.RSVPDeadlineHours != 50 {
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
		if child.StartsAt.Sub(child.RSVPDeadline) != 50*time.Hour {
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
	date := time.Date(2027, time.September, 28, 0, 0, 0, 0, utils.SydneyLocation)
	original := time.Date(2027, time.September, 26, 1, 30, 0, 0, utils.SydneyLocation)
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

func TestMovingCustomParentIntoClockGapRollsBackEdit(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := time.Date(2027, time.September, 28, 0, 0, 0, 0, utils.SydneyLocation)
	original := time.Date(2027, time.September, 26, 2, 30, 0, 0, utils.SydneyLocation)
	day, occurrences := int(date.Weekday()), 1
	parent, err := ss.CreateSession(CreateSessionInput{Title: "Original series", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &original})
	if err != nil {
		t.Fatal(err)
	}
	moved := date.AddDate(0, 0, 7)
	title := "Must not save"
	// A valid occurrence override must not hide an invalid series rule.
	override := time.Date(2027, time.October, 3, 1, 30, 0, 0, utils.SydneyLocation)
	_, err = ss.UpdateSession(parent.ID, UpdateSessionInput{SessionDate: &moved, Title: &title, RSVPDeadline: &override})
	if !errors.Is(err, errRecurringDeadline) {
		t.Fatalf("expected recurring clock gap error, got %v", err)
	}
	var saved models.Session
	if err := database.DB.First(&saved, "id = ?", parent.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.SessionDate.In(utils.SydneyLocation).Format("2006-01-02") != date.Format("2006-01-02") || saved.Title != parent.Title ||
		!saved.RSVPDeadline.Equal(original) || saved.RecurringRSVPDeadline == nil || !saved.RecurringRSVPDeadline.Equal(original) ||
		saved.StartsAt == nil || !saved.StartsAt.Equal(*parent.StartsAt) {
		t.Fatal("rejected date edit changed the parent or recurring rule")
	}
}

func TestRecurringStartEditRejectsAnInvalidStoredDeadlineRule(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := utils.NowInSydney().AddDate(0, 0, 14)
	original := time.Date(date.Year(), date.Month(), date.Day(), 18, 0, 0, 0, utils.SydneyLocation)
	day, occurrences := int(date.Weekday()), 1
	parent, err := ss.CreateSession(CreateSessionInput{Title: "Series rule validation", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &original})
	if err != nil {
		t.Fatal(err)
	}
	start := "17:00"
	replacement := original.Add(-2 * time.Hour)
	if _, err := ss.UpdateSession(parent.ID, UpdateSessionInput{StartTime: &start, RSVPDeadline: &replacement}); err == nil {
		t.Fatal("expected the start change to reject a recurring deadline after the new start")
	}
	saved, err := ss.GetSessionByID(parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.StartTime != "20:00" || !saved.RSVPDeadline.Equal(original) {
		t.Fatal("rejected edit changed the parent")
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
		if child.RSVPDeadline.After(*child.StartsAt) {
			t.Fatal("child deadline is after start")
		}
	}
}

func TestRefreshRejectsInvalidLegacyRulesWithoutBlockingValidSeries(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	date := utils.NowInSydney().AddDate(0, 0, 14)
	rule := time.Date(date.Year(), date.Month(), date.Day(), 18, 0, 0, 0, utils.SydneyLocation)
	day := int(date.Weekday())
	parent := models.Session{Title: "Invalid legacy rule", SessionDate: date, StartTime: "17:00", EndTime: "20:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, CreatedBy: admin.ID, Status: models.SessionStatusOpen, RSVPDeadline: rule, RecurringRSVPDeadline: &rule}
	if err := database.DB.Create(&parent).Error; err != nil {
		t.Fatal(err)
	}
	occurrences := 1
	valid, err := ss.CreateSession(CreateSessionInput{Title: "Valid series", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.RefreshRecurringSessions(); err == nil {
		t.Fatal("expected invalid legacy series to be rejected")
	}
	var count int64
	if err := database.DB.Model(&models.Session{}).Where("recurring_parent_id = ?", parent.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("refresh created children with invalid deadlines")
	}
	if err := database.DB.Model(&models.Session{}).Where("recurring_parent_id = ?", valid.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("invalid series blocked a valid series: got %d children", count)
	}

}

func TestRecurringCustomDeadlineGapRollsBackCreation(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "gapseries")
	date := time.Date(2027, time.September, 21, 0, 0, 0, 0, utils.SydneyLocation)
	custom := time.Date(2027, time.September, 19, 2, 30, 59, 0, utils.SydneyLocation)
	day, occurrences := int(time.Tuesday), 3
	_, err := ss.CreateSession(CreateSessionInput{Title: "Gap series", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &custom})
	if !errors.Is(err, errRecurringDeadline) {
		t.Fatalf("expected invalid recurring rule, got %v", err)
	}
	var count int64
	if err := database.DB.Model(&models.Session{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("failed create left %d parent or child sessions", count)
	}
}

func TestRecurringCustomClockOutsideGapKeepsSydneyTime(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "validclock")
	date := time.Date(2027, time.September, 21, 0, 0, 0, 0, utils.SydneyLocation)
	custom := time.Date(2027, time.September, 19, 1, 30, 59, 0, utils.SydneyLocation)
	day, occurrences := int(time.Tuesday), 4
	parent, err := ss.CreateSession(CreateSessionInput{Title: "Valid custom clock", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &custom})
	if err != nil {
		t.Fatal(err)
	}
	var children []models.Session
	if err := database.DB.Where("recurring_parent_id = ?", parent.ID).Order("session_date").Find(&children).Error; err != nil {
		t.Fatal(err)
	}
	if len(children) != 3 {
		t.Fatalf("got %d children", len(children))
	}
	for _, child := range children {
		deadline := child.RSVPDeadline.In(utils.SydneyLocation)
		if deadline.Weekday() != time.Sunday || deadline.Hour() != 1 || deadline.Minute() != 30 || deadline.Second() != 59 {
			t.Fatalf("custom clock changed: %v", deadline)
		}
	}
}

func TestRecurringRefreshGapRollsBackOnlyInvalidSeries(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "refreshgap")
	date := time.Date(2027, time.September, 21, 0, 0, 0, 0, utils.SydneyLocation)
	custom := time.Date(2027, time.September, 19, 2, 30, 59, 0, utils.SydneyLocation)
	day, occurrences := int(time.Tuesday), 1
	invalid, err := ss.CreateSession(CreateSessionInput{Title: "Invalid on refresh", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID, RSVPDeadline: &custom})
	if err != nil {
		t.Fatal(err)
	}
	valid, err := ss.CreateSession(CreateSessionInput{Title: "Default hours remain valid", SessionDate: date, StartTime: "20:00", EndTime: "22:00", Courts: 1, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences, CreatedBy: admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.RefreshRecurringSessions(); !errors.Is(err, errRecurringDeadline) {
		t.Fatalf("expected invalid series error, got %v", err)
	}
	var invalidChildren int64
	if err := database.DB.Model(&models.Session{}).Where("recurring_parent_id = ?", invalid.ID).Count(&invalidChildren).Error; err != nil {
		t.Fatal(err)
	}
	if invalidChildren != 0 {
		t.Fatalf("invalid series kept %d partial children", invalidChildren)
	}
	var validChildren []models.Session
	if err := database.DB.Where("recurring_parent_id = ?", valid.ID).Find(&validChildren).Error; err != nil {
		t.Fatal(err)
	}
	if len(validChildren) != 3 {
		t.Fatalf("valid series has %d children", len(validChildren))
	}
	for _, child := range validChildren {
		if child.StartsAt.Sub(child.RSVPDeadline) != 50*time.Hour {
			t.Fatalf("elapsed-hour default changed across DST: %+v", child)
		}
	}
}
