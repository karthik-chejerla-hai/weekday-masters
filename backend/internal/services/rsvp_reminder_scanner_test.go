package services

import (
	"context"
	"errors"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
	"sync"
	"testing"
	"time"
)

func rsvpScannerFixture(t *testing.T) (*NotificationService, *SessionService, models.User) {
	t.Helper()
	requireDB(t)
	if err := database.MigrateRSVPReminderSchema(database.DB); err != nil {
		t.Fatal(err)
	}
	return &NotificationService{fcmEnabled: true, fcmClient: &ttlPushClient{}}, NewSessionService(), newUser(t, "reminder")
}
func scannerSession(t *testing.T, ss *SessionService, user models.User, starts time.Time) *models.Session {
	t.Helper()
	session, err := ss.CreateSession(CreateSessionInput{Title: "Badminton", SessionDate: starts.In(utils.SydneyLocation), StartTime: starts.In(utils.SydneyLocation).Format("15:04"), EndTime: starts.Add(2 * time.Hour).In(utils.SydneyLocation).Format("15:04"), Courts: 1, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	return session
}
func scanRSVP(t *testing.T, s *NotificationService, now time.Time) *RSVPReminderScanSummary {
	t.Helper()
	summary, err := s.QueueDueRSVPReminders(context.Background(), now, 100)
	if err != nil {
		t.Fatal(err)
	}
	return summary
}
func countRSVPEvents(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.Notification{}).Where("notification_type = ?", models.NotificationRSVPReminder).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}
func stageOf(t *testing.T, session models.Session, index int) models.RSVPReminderStage {
	t.Helper()
	var stage models.RSVPReminderStage
	if err := database.DB.Where("session_id = ? AND stage = ?", session.ID, index).First(&stage).Error; err != nil {
		t.Fatal(err)
	}
	return stage
}

func TestRSVPScannerExactBoundariesAndEveryResponseStopsReminders(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	start := utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute)
	session := scannerSession(t, ss, user, start)
	start = *session.StartsAt
	for _, status := range []models.RSVPStatus{models.RSVPStatusIn, models.RSVPStatusOut, models.RSVPStatusMaybe, models.RSVPStatusWaitlisted} {
		member := newUser(t, string(status))
		if err := database.DB.Create(&models.RSVP{SessionID: session.ID, UserID: member.ID, Status: status, RSVPTimestamp: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	pending := newUser(t, "pending")
	database.DB.Model(&pending).Update("membership_status", models.MembershipPending)
	addDeliveryDevice(t, user, "device")
	if scanRSVP(t, s, start.Add(-96*time.Hour-time.Nanosecond)).Events != 0 {
		t.Fatal("reminder published early")
	}
	if got := scanRSVP(t, s, start.Add(-96*time.Hour)); got.Events != 1 {
		t.Fatalf("exact first boundary %v", got)
	}
	if scanRSVP(t, s, start.Add(-96*time.Hour)).Events != 0 {
		t.Fatal("first stage duplicated")
	}
	if scanRSVP(t, s, start.Add(-60*time.Hour-time.Nanosecond)).Events != 0 {
		t.Fatal("second reminder early")
	}
	if got := scanRSVP(t, s, start.Add(-60*time.Hour)); got.Events != 1 {
		t.Fatalf("exact second boundary %v", got)
	}
	if scanRSVP(t, s, start.Add(-48*time.Hour)).Events != 0 || countRSVPEvents(t) != 2 {
		t.Fatal("deadline generated reminders")
	}
}
func TestRSVPScannerOutageAndLateCreationPublishOnlyLatestUsefulStage(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	start := utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute)
	session := scannerSession(t, ss, user, start)
	start = *session.StartsAt
	if got := scanRSVP(t, s, start.Add(-54*time.Hour)); got.Events != 1 || got.Skipped != 1 {
		t.Fatalf("outage catch-up burst or missing skip: %+v", got)
	}
	if stageOf(t, *session, 1).State != models.RSVPReminderSkipped || stageOf(t, *session, 2).State != models.RSVPReminderPublished {
		t.Fatal("incorrect catch-up stage states")
	}
	if scanRSVP(t, s, start.Add(-48*time.Hour)).Events != 0 {
		t.Fatal("late stage after closure")
	}
}
func TestRSVPScannerConcurrentWorkersPublishOneAudience(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	session := scannerSession(t, ss, user, utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute))
	now := session.StartsAt.Add(-96 * time.Hour)
	var group sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			_, err := s.QueueDueRSVPReminders(context.Background(), now, 100)
			errs <- err
		}()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if countRSVPEvents(t) != 1 {
		t.Fatal("concurrent scanners duplicated logical stage")
	}
	// Audience is frozen at publication; a newly approved member waits for the
	// next stage instead of receiving a backfill of the published first stage.
	newUser(t, "newly-approved")
	if scanRSVP(t, s, now.Add(time.Minute)).Events != 0 {
		t.Fatal("published audience backfilled")
	}
	if scanRSVP(t, s, session.StartsAt.Add(-60*time.Hour)).Events != 2 {
		t.Fatal("next stage did not evaluate current approved audience")
	}
}
func TestRSVPScannerRolloutCutoffAtomicAndRepeatSafe(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	now := utils.NowInSydney().Truncate(time.Minute)
	session := scannerSession(t, ss, user, now.Add(80*time.Hour))
	cutoff := session.StartsAt.Add(-80 * time.Hour)
	custom := cutoff.Add(time.Hour)
	if err := database.DB.Model(session).UpdateColumn("rsvp_deadline", custom).Error; err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("stage rollout unavailable")
	err := database.RunNotificationMigration(database.DB, database.RSVP48HourMigrationVersion, func(tx *gorm.DB, _ time.Time) error {
		if err := database.ApplyRSVPDeadline48hWithin(tx, cutoff); err != nil {
			return err
		}
		if err := database.ApplyRSVPReminderCutoverWithin(tx, cutoff); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	var count int64
	database.DB.Model(&models.RSVPReminderStage{}).Count(&count)
	if count != 0 {
		t.Fatal("partial stage migration committed")
	}
	var unchanged models.Session
	if err := database.DB.First(&unchanged, "id = ?", session.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !unchanged.RSVPDeadline.Equal(custom) {
		t.Fatal("deadline normalization committed without stage cutover")
	}
	calls := 0
	apply := func(tx *gorm.DB, _ time.Time) error {
		calls++
		if err := database.ApplyRSVPDeadline48hWithin(tx, cutoff); err != nil {
			return err
		}
		return database.ApplyRSVPReminderCutoverWithin(tx, cutoff)
	}
	if err := database.RunNotificationMigration(database.DB, database.RSVP48HourMigrationVersion, apply); err != nil {
		t.Fatal(err)
	}
	if stageOf(t, *session, 1).State != models.RSVPReminderSkipped || stageOf(t, *session, 2).State != models.RSVPReminderPending {
		t.Fatal("cutover replayed past first stage or skipped future second")
	}
	if scanRSVP(t, s, cutoff).Events != 0 {
		t.Fatal("rollout replayed first reminder")
	}
	if scanRSVP(t, s, session.StartsAt.Add(-60*time.Hour)).Events != 1 {
		t.Fatal("future rollout stage omitted")
	}
	if err := database.RunNotificationMigration(database.DB, database.RSVP48HourMigrationVersion, apply); err != nil {
		t.Fatal(err)
	}
	if calls != 1 || stageOf(t, *session, 2).State != models.RSVPReminderPublished {
		t.Fatal("repeat migration reset publication")
	}
}
func TestRSVPScannerReschedulesUnsentButDoesNotReplayAcceptedStage(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	session := scannerSession(t, ss, user, utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute))
	addDeliveryDevice(t, user, "device")
	now := session.StartsAt.Add(-96 * time.Hour)
	scanRSVP(t, s, now)
	key := RSVPReminderEventKey(session.ID, user.ID, 1)
	var outbox models.NotificationOutbox
	if err := database.DB.Where("event_key = ?", key).First(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	later := session.SessionDate.AddDate(0, 0, 7)
	ss.now = func() time.Time { return now }
	updated, err := ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &later})
	if err != nil {
		t.Fatal(err)
	}
	scanRSVP(t, s, now)
	if err := database.DB.First(&outbox, "id = ?", outbox.ID).Error; err != nil {
		t.Fatal(err)
	}
	due, expires := RSVPReminderTimes(*updated.StartsAt, updated.RSVPDeadline, 1)
	if !outbox.DueAt.Equal(due) || !outbox.ExpiresAt.Equal(expires) || !outbox.NextAttemptAt.Equal(due) {
		t.Fatalf("unsent stage did not follow edited start: %+v", outbox)
	}
	if err := database.DB.Model(&outbox).Update("state", models.DeliveryCompleted).Error; err != nil {
		t.Fatal(err)
	}
	newest := later.AddDate(0, 0, 7)
	updated, err = ss.UpdateSession(session.ID, UpdateSessionInput{SessionDate: &newest})
	if err != nil {
		t.Fatal(err)
	}
	scanRSVP(t, s, updated.StartsAt.Add(-96*time.Hour))
	if countRSVPEvents(t) != 1 {
		t.Fatal("already completed stage repeated after reschedule")
	}
}
func TestRSVPScannerDSTAndCancellationClosure(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	for _, date := range []string{"2026-10-05", "2026-04-06"} {
		day, _ := utils.ParseDateInSydney(date)
		session := scannerSession(t, ss, user, time.Date(day.Year(), day.Month(), day.Day(), 18, 0, 0, 0, utils.SydneyLocation))
		now := session.StartsAt.Add(-96 * time.Hour)
		before := countRSVPEvents(t)
		scanRSVP(t, s, now)
		if countRSVPEvents(t) != before+1 {
			t.Fatal("DST elapsed due boundary failed")
		}
		stage := stageOf(t, *session, 1)
		if session.StartsAt.Sub(stage.DueAt) != 96*time.Hour || stage.ExpiresAt.Sub(stage.DueAt) != 36*time.Hour {
			t.Fatal("DST shifted reminder interval")
		}
		database.DB.Model(session).UpdateColumn("status", models.SessionStatusCancelled)
		if scanRSVP(t, s, session.StartsAt.Add(-60*time.Hour)).Events != 0 {
			t.Fatal("cancelled session reminder created")
		}
	}
}
func TestRSVPScannerCompletedSessionsDoNotStarveLaterDueSessions(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	base := utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute)
	first := scannerSession(t, ss, user, base)
	second := scannerSession(t, ss, user, base.Add(time.Hour))
	now := second.StartsAt.Add(-96 * time.Hour)
	if got, err := s.QueueDueRSVPReminders(context.Background(), now, 1); err != nil || got.Events != 1 {
		t.Fatalf("first batch failed: %+v %v", got, err)
	}
	if got, err := s.QueueDueRSVPReminders(context.Background(), now, 1); err != nil || got.Events != 1 {
		t.Fatalf("published first session starved second: %+v %v", got, err)
	}
	if stageOf(t, *first, 1).State != models.RSVPReminderPublished || stageOf(t, *second, 1).State != models.RSVPReminderPublished {
		t.Fatal("batch progress did not advance")
	}
}

func TestRSVPScannerPublicationRollsBackWholeAudience(t *testing.T) {
	s, ss, user := rsvpScannerFixture(t)
	newUser(t, "second recipient")
	session := scannerSession(t, ss, user, utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute))
	const callback = "test:fail_rsvp_second_recipient"
	created := 0
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notifications" {
			created++
			if created == 2 {
				tx.AddError(errors.New("history unavailable"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)
	summary, err := s.QueueDueRSVPReminders(context.Background(), session.StartsAt.Add(-96*time.Hour), 100)
	if err == nil {
		t.Fatal("expected audience failure")
	}
	if summary.Events != 0 || summary.Published != 0 {
		t.Fatalf("rolled-back audience reported as committed: %+v", summary)
	}
	if countRSVPEvents(t) != 0 {
		t.Fatal("partial recipient history committed")
	}
	var count int64
	database.DB.Model(&models.NotificationOutbox{}).Count(&count)
	if count != 0 {
		t.Fatal("partial push work committed")
	}
	database.DB.Model(&models.RSVPReminderStage{}).Count(&count)
	if count != 0 {
		t.Fatal("failed publication marked stage complete")
	}
}

func TestRSVPScannerPausedPublicationDoesNotReplayOnResume(t *testing.T) {
	for _, mode := range []string{"deployment", "club"} {
		t.Run(mode, func(t *testing.T) {
			s, ss, user := rsvpScannerFixture(t)
			session := scannerSession(t, ss, user, utils.NowInSydney().AddDate(0, 0, 6).Truncate(time.Minute))
			addDeliveryDevice(t, user, "device")
			if mode == "deployment" {
				s.disabled = true
			} else {
				if err := database.DB.Model(&models.Club{}).Where("id IS NOT NULL").Update("notifications_paused", true).Error; err != nil {
					t.Fatal(err)
				}
			}
			now := session.StartsAt.Add(-96 * time.Hour)
			if scanRSVP(t, s, now).Events != 1 {
				t.Fatal("pause erased member history")
			}
			var outbox models.NotificationOutbox
			if err := database.DB.Where("event_key = ?", RSVPReminderEventKey(session.ID, user.ID, 1)).First(&outbox).Error; err != nil {
				t.Fatal(err)
			}
			if outbox.State != models.DeliverySuppressed {
				t.Fatal("paused stage queued live delivery")
			}
			s.disabled = false
			if err := database.DB.Model(&models.Club{}).Where("id IS NOT NULL").Update("notifications_paused", false).Error; err != nil {
				t.Fatal(err)
			}
			if scanRSVP(t, s, now.Add(time.Minute)).Events != 0 {
				t.Fatal("pause lift replayed previous stage")
			}
			if scanRSVP(t, s, session.StartsAt.Add(-60*time.Hour)).Events != 1 {
				t.Fatal("future stage lost after resume")
			}
		})
	}
}
