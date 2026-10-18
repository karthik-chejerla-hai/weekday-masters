package services

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

func reminderTestScheduler(t *testing.T, user models.User) (*SchedulerService, *atomic.Int32) {
	t.Helper()
	calls := &atomic.Int32{}
	ns := &NotificationService{fcmEnabled: true, frontendURL: "https://rally.test", fcmClient: stubPushClient(func(_ context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		calls.Add(1)
		return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
	})}
	if err := ns.RegisterPushToken(user.ID, "synthetic-reminder-device", "Test"); err != nil {
		t.Fatal(err)
	}
	return NewSchedulerService(SchedulerConfig{NotificationService: ns, SessionReminderHours24: 24, SessionReminderHours12: 12, DeadlineReminderHours: 6}), calls
}

func assertReminderCount(t *testing.T, userID uuid.UUID, want int64) {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.Notification{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != want {
		t.Fatalf("history count = %d, want %d", count, want)
	}
}

func TestSchedulerDeadlineReminderRepeatedAndConcurrent(t *testing.T) {
	requireDB(t)
	user := newUser(t, "deadline")
	session := newSession(t, NewSessionService(), user.ID, 1)
	scheduler, calls := reminderTestScheduler(t, user)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); scheduler.sendDeadlineReminders(context.Background(), *session) }()
	}
	wg.Wait()
	scheduler.sendDeadlineReminders(context.Background(), *session)
	assertReminderCount(t, user.ID, 1)
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", calls.Load())
	}
	session.RSVPDeadline = session.RSVPDeadline.Add(time.Hour)
	if err := database.DB.Model(session).Update("rsvp_deadline", session.RSVPDeadline).Error; err != nil {
		t.Fatal(err)
	}
	scheduler.sendDeadlineReminders(context.Background(), *session)
	assertReminderCount(t, user.ID, 2)
}

func TestSchedulerSessionReminderKindsAndBoundary(t *testing.T) {
	requireDB(t)
	user := newUser(t, "session")
	session := newSession(t, NewSessionService(), user.ID, 1)
	rsvpIn(t, NewRSVPService(nil), session.ID, user.ID)
	scheduler, calls := reminderTestScheduler(t, user)
	starts := session.StartsAt.In(utils.SydneyLocation)
	scheduler.sendSessionRemindersForWindow(starts.Add(-25*time.Hour), 24, "24h")
	assertReminderCount(t, user.ID, 0)
	scheduler.sendSessionRemindersForWindow(starts.Add(-24*time.Hour), 24, "24h")
	scheduler.sendSessionRemindersForWindow(starts.Add(-24*time.Hour), 24, "24h")
	scheduler.sendSessionRemindersForWindow(starts.Add(-12*time.Hour), 12, "12h")
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); scheduler.sendSessionReminders(*session, "24h") }()
	}
	wg.Wait()
	assertReminderCount(t, user.ID, 2)
	if calls.Load() != 2 {
		t.Fatalf("provider calls = %d, want 2", calls.Load())
	}
}

func TestSchedulerSuppressionDoesNotConsumeReminder(t *testing.T) {
	for _, stop := range []string{"deployment", "club", "membership", "rsvp", "preferences"} {
		t.Run(stop, func(t *testing.T) {
			requireDB(t)
			user := newUser(t, "suppressed")
			session := newSession(t, NewSessionService(), user.ID, 1)
			scheduler, calls := reminderTestScheduler(t, user)
			switch stop {
			case "deployment":
				scheduler.notificationService.disabled = true
			case "club":
				if err := database.DB.Model(&models.Club{}).Where("1=1").Update("notifications_paused", true).Error; err != nil {
					t.Fatal(err)
				}
			case "membership":
				if err := database.DB.Model(&user).Update("membership_status", models.MembershipPending).Error; err != nil {
					t.Fatal(err)
				}
			case "preferences":
				if _, err := scheduler.notificationService.UpdateUserPreferences(user.ID, map[string]interface{}{"push_enabled": false}); err != nil {
					t.Fatal(err)
				}
			case "rsvp":
				rsvpIn(t, NewRSVPService(nil), session.ID, user.ID)
			}
			scheduler.sendDeadlineReminders(context.Background(), *session)
			assertReminderCount(t, user.ID, 0)
			if calls.Load() != 0 {
				t.Fatal("suppressed reminder reached provider")
			}
			scheduler.notificationService.disabled = false
			if _, err := scheduler.notificationService.UpdateUserPreferences(user.ID, map[string]interface{}{"push_enabled": true}); err != nil {
				t.Fatal(err)
			}
			if err := database.DB.Model(&models.Club{}).Where("1=1").Update("notifications_paused", false).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.DB.Model(&user).Update("membership_status", models.MembershipApproved).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.DB.Where("session_id = ?", session.ID).Delete(&models.RSVP{}).Error; err != nil {
				t.Fatal(err)
			}
			scheduler.sendDeadlineReminders(context.Background(), *session)
			assertReminderCount(t, user.ID, 1)
			if calls.Load() != 1 {
				t.Fatalf("provider calls = %d, want 1", calls.Load())
			}
		})
	}
}

func TestSchedulerSessionReminderWindowCrossesSydneyMidnight(t *testing.T) {
	requireDB(t)
	user := newUser(t, "midnight")
	date := time.Date(2026, 10, 20, 0, 0, 0, 0, utils.SydneyLocation)
	session, err := NewSessionService().CreateSession(CreateSessionInput{Title: "Midnight", SessionDate: date, StartTime: "00:00", EndTime: "02:00", Courts: 1, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Create(&models.RSVP{SessionID: session.ID, UserID: user.ID, Status: models.RSVPStatusIn}).Error; err != nil {
		t.Fatal(err)
	}
	scheduler, calls := reminderTestScheduler(t, user)
	now := session.StartsAt.Add(-24*time.Hour - 30*time.Minute)
	scheduler.sendSessionRemindersForWindow(now, 24, "24h")
	assertReminderCount(t, user.ID, 1)
	if calls.Load() != 1 {
		t.Fatalf("provider calls = %d, want 1", calls.Load())
	}
}
