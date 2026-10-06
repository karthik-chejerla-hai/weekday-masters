package services

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
	"gorm.io/gorm"
)

func cancellationCounts(t *testing.T, announcements, notifications int64) {
	t.Helper()
	for _, item := range []struct {
		model any
		want  int64
	}{{&models.Announcement{}, announcements}, {&models.Notification{}, notifications}} {
		var count int64
		if err := database.DB.Model(item.model).Count(&count).Error; err != nil || count != item.want {
			t.Fatalf("%T count = %d, want %d: %v", item.model, count, item.want, err)
		}
	}
}

func TestCancellationAudienceAndNextSydneySession(t *testing.T) {
	rs, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	players := newUsers(t, 5)
	for i, status := range []models.MembershipStatus{models.MembershipPending, models.MembershipRejected, models.MembershipRemoved} {
		if err := database.DB.Model(&players[i]).Update("membership_status", status).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A session near midnight makes an accidental UTC display land on the previous date.
	date, _ := utils.ParseDateInSydney("2036-10-05")
	session, err := ss.CreateSession(CreateSessionInput{Title: "Recurring game", SessionDate: date, StartTime: "00:30", EndTime: "02:30", Courts: 1, CreatedBy: admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	rsvpIn(t, rs, session.ID, players[3].ID)
	// No RSVP for players[4]: they must still get the announcement.
	for _, fixture := range []struct {
		offset int
		status models.SessionStatus
	}{{-1, models.SessionStatusOpen}, {1, models.SessionStatusCancelled}, {2, models.SessionStatusClosed}, {3, models.SessionStatusOpen}} {
		other, err := ss.CreateSession(CreateSessionInput{Title: "Other game", SessionDate: date.AddDate(0, 0, fixture.offset), StartTime: "00:30", EndTime: "02:30", Courts: 1, CreatedBy: admin.ID})
		if err != nil {
			t.Fatal(err)
		}
		if err := database.DB.Model(other).Update("status", fixture.status).Error; err != nil {
			t.Fatal(err)
		}
	}
	cancelled, err := ss.CancelSession(session.ID, "  Court flooded\nPlease stay home.  ", admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cancelled.Status != models.SessionStatusCancelled || cancelled.CancellationReason != "Court flooded\nPlease stay home." {
		t.Fatalf("unexpected cancellation: %+v", cancelled)
	}
	cancellationCounts(t, 1, 3)
	var announcement models.Announcement
	if err := database.DB.First(&announcement).Error; err != nil {
		t.Fatal(err)
	}
	if announcement.CreatedBy != admin.ID {
		t.Fatal("acting admin was not recorded")
	}
	for _, text := range []string{"Sunday, 5 October 2036 at 00:30 AEST", "Court flooded\nPlease stay home.", "Tuesday, 7 October 2036 at 00:30 AEDT"} {
		if !strings.Contains(announcement.Body, text) {
			t.Fatalf("announcement missing %q: %s", text, announcement.Body)
		}
	}
	var notifications []models.Notification
	if err := database.DB.Find(&notifications).Error; err != nil {
		t.Fatal(err)
	}
	expected := map[uuid.UUID]bool{admin.ID: true, players[3].ID: true, players[4].ID: true}
	for _, notice := range notifications {
		if !expected[notice.UserID] || notice.NotificationType != models.NotificationAdminAnnouncement || notice.Body != announcement.Body {
			t.Fatalf("unexpected notice: %+v", notice)
		}
		delete(expected, notice.UserID)
		var data map[string]string
		if err := json.Unmarshal([]byte(notice.Data), &data); err != nil {
			t.Fatal(err)
		}
		if data["session_id"] != session.ID.String() || data["announcement_id"] != announcement.ID.String() {
			t.Fatalf("incorrect links: %v", data)
		}
	}
	if len(expected) > 0 {
		t.Fatal("missing recipients")
	}
	if _, err := rs.CreateOrUpdateRSVP(RSVPInput{SessionID: session.ID, UserID: players[4].ID, Status: models.RSVPStatusIn}, true); err == nil {
		t.Fatal("admin could RSVP to cancelled session")
	}
	if err := rs.DeleteRSVP(session.ID, players[3].ID, true); err == nil {
		t.Fatal("cancelled RSVP was deleted")
	}
	if statusOf(t, rs, session.ID, players[3].ID).Status != models.RSVPStatusIn {
		t.Fatal("attendance history changed")
	}
	upcoming, err := ss.ListUpcomingSessions()
	if err != nil || len(upcoming) != 3 {
		t.Fatalf("other dates changed: %d %v", len(upcoming), err)
	}
}

func TestCancellationConcurrentRequestsAnnounceOnce(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := ss.CancelSession(session.ID, "  ", admin.ID); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	cancellationCounts(t, 1, 1)
	again, err := ss.CancelSession(session.ID, "A different reason", admin.ID)
	if err != nil || again.CancellationReason != "" {
		t.Fatalf("retry changed reason: %+v %v", again, err)
	}
	cancellationCounts(t, 1, 1)
	var announcement models.Announcement
	if err := database.DB.First(&announcement).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(announcement.Body, "No reason provided.") || !strings.Contains(announcement.Body, "No next session is scheduled.") {
		t.Fatal(announcement.Body)
	}
}

func TestCancellationRollsBackWhenHistoryCannotBeStored(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	const callback = "test:fail_cancellation_history"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notifications" {
			tx.AddError(errors.New("history unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)
	if _, err := ss.CancelSession(session.ID, "Rain", admin.ID); err == nil {
		t.Fatal("expected history failure")
	}
	stored, err := ss.GetSessionByID(session.ID)
	if err != nil || stored.Status != models.SessionStatusOpen || stored.CancellationReason != "" {
		t.Fatalf("partial cancellation: %+v %v", stored, err)
	}
	cancellationCounts(t, 0, 0)
}

func TestCancellationGuardsAndNoWaitlistPromotion(t *testing.T) {
	rs, ss, notifier := newTestServices(t)
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	players := newUsers(t, 7)
	for _, p := range players {
		rsvpIn(t, rs, session.ID, p.ID)
	}
	if _, err := ss.CancelSession(session.ID, strings.Repeat("x", 1001), admin.ID); err == nil {
		t.Fatal("accepted long reason")
	}
	cancelled := models.SessionStatusCancelled
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{Status: &cancelled}); err == nil {
		t.Fatal("allowed silent cancellation")
	}
	if _, err := ss.CancelSession(session.ID, "Rain", admin.ID); err != nil {
		t.Fatal(err)
	}
	open := models.SessionStatusOpen
	if _, err := ss.UpdateSession(session.ID, UpdateSessionInput{Status: &open}); err == nil {
		t.Fatal("reopened cancelled session")
	}
	if err := ss.DeleteSession(session.ID); err == nil {
		t.Fatal("deleted cancellation history")
	}
	// Free capacity through SQL to check the promotion guard independently.
	if err := database.DB.Model(session).Update("max_players", 10).Error; err != nil {
		t.Fatal(err)
	}
	if err := rs.PromoteFromWaitlist(session.ID); err != nil {
		t.Fatal(err)
	}
	if statusOf(t, rs, session.ID, players[6].ID).Status != models.RSVPStatusWaitlisted || len(notifier.notifiedUsers()) != 0 {
		t.Fatal("promoted a cancelled session waitlist")
	}
}

func TestCancellationRejectsFinishedAndSettledSessions(t *testing.T) {
	for _, mode := range []string{"finished", "settled", "reversed"} {
		t.Run(mode, func(t *testing.T) {
			_, ss, _ := newTestServices(t)
			admin := newUser(t, "admin")
			session := newSession(t, ss, admin.ID, 1)
			if mode == "finished" {
				session.SessionDate = utils.NowInSydney().AddDate(0, 0, -1)
				if err := database.DB.Save(session).Error; err != nil {
					t.Fatal(err)
				}
			} else {
				settlement := models.Settlement{SessionID: session.ID, TransactionID: uuid.New(), SettledBy: admin.ID}
				if mode == "reversed" {
					now := time.Now()
					settlement.ReversedAt = &now
				}
				if err := database.DB.Create(&settlement).Error; err != nil {
					t.Fatal(err)
				}
			}
			if _, err := ss.CancelSession(session.ID, "Rain", admin.ID); err == nil {
				t.Fatal("cancelled historical session")
			}
			cancellationCounts(t, 0, 0)
		})
	}
}

func TestCancellationOutboundSettings(t *testing.T) {
	for _, mode := range []string{"enabled", "disabled", "paused", "opted_out", "provider_failure"} {
		t.Run(mode, func(t *testing.T) {
			_, ss, _ := newTestServices(t)
			admin := newUser(t, "admin")
			session := newSession(t, ss, admin.ID, 1)
			var requests atomic.Int32
			ns := &NotificationService{fcmEnabled: true, fcmClient: stubPushClient(func(_ context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
				requests.Add(1)
				if !strings.Contains(message.Notification.Body, "<img") {
					t.Error("push body lost reason")
				}
				if mode == "provider_failure" {
					return nil, errors.New("provider failure")
				}
				return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
			})}
			if err := ns.RegisterPushToken(admin.ID, "test-token", "test"); err != nil {
				t.Fatal(err)
			}

			ns.disabled = mode == "disabled"
			ss.WithNotifier(ns)
			if mode == "paused" {
				if err := database.DB.Model(&models.Club{}).Where("1 = 1").Update("notifications_paused", true).Error; err != nil {
					t.Fatal(err)
				}
			}
			if mode == "opted_out" {
				if _, err := ns.UpdateUserPreferences(admin.ID, map[string]interface{}{"email_admin_announcements": false, "push_admin_announcements": false}); err != nil {
					t.Fatal(err)
				}
				ns.fcmEnabled = true
			}
			if _, err := ss.CancelSession(session.ID, "<img src=x onerror=alert(1)>\nRain", admin.ID); err != nil {
				t.Fatal(err)
			}
			// Retrying never sends again, including when the provider rejected the first attempt.
			if _, err := ss.CancelSession(session.ID, "Changed", admin.ID); err != nil {
				t.Fatal(err)
			}
			want := int32(0)
			if mode == "enabled" || mode == "provider_failure" {
				want = 1
			}
			if requests.Load() != want {
				t.Fatalf("provider calls=%d, want %d", requests.Load(), want)
			}
			cancellationCounts(t, 1, 1)
			var notice models.Notification
			if err := database.DB.First(&notice).Error; err != nil {
				t.Fatal(err)
			}
			if notice.PushSent != (mode == "enabled") || notice.EmailSent {
				t.Fatalf("incorrect delivery result: %+v", notice)
			}
		})
	}
}

func TestCancellationLeavesOtherRecurringDatesScheduled(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	day, occurrences := 0, 3
	session, err := ss.CreateSession(CreateSessionInput{Title: "Weekly game", SessionDate: utils.NowInSydney().AddDate(0, 0, 10), StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: admin.ID, IsRecurring: true, RecurringDayOfWeek: &day, Occurrences: &occurrences})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ss.CancelSession(session.ID, "Venue closed", admin.ID); err != nil {
		t.Fatal(err)
	}
	upcoming, err := ss.ListUpcomingSessions()
	if err != nil || len(upcoming) != 2 {
		t.Fatalf("changed recurring dates: %+v %v", upcoming, err)
	}
	for _, child := range upcoming {
		if child.RecurringParentID == nil || *child.RecurringParentID != session.ID || child.Status != models.SessionStatusOpen {
			t.Fatal("changed sibling occurrence")
		}
	}
}

func TestCancellationNextSessionHasNotAlreadyStarted(t *testing.T) {
	_, ss, _ := newTestServices(t)
	admin := newUser(t, "admin")
	session := newSession(t, ss, admin.ID, 1)
	other := newSession(t, ss, admin.ID, 1)
	now := time.Now()
	// Resolved timestamps represent an ongoing session and another game that has
	// already ended. Cancellation must not announce the ended game as next.
	if err := database.DB.Model(session).Updates(map[string]any{"starts_at": now.Add(-2 * time.Hour), "ends_at": now.Add(time.Hour)}).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(other).Updates(map[string]any{"starts_at": now.Add(-time.Hour), "ends_at": now.Add(-time.Minute)}).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ss.CancelSession(session.ID, "Rain", admin.ID); err != nil {
		t.Fatal(err)
	}
	var announcement models.Announcement
	if err := database.DB.First(&announcement).Error; err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(announcement.Body, "No next session is scheduled.") {
		t.Fatal(announcement.Body)
	}
}

func TestConcurrentCancellationsDeliverWithNewPreferences(t *testing.T) {
	_, ss, _ := newTestServices(t)
	member := newUser(t, "member")
	sessions := []*models.Session{
		newSession(t, ss, member.ID, 1),
		newSession(t, ss, member.ID, 1),
	}
	var requests atomic.Int32
	bodies := make(chan string, 2)
	ns := &NotificationService{fcmEnabled: true, fcmClient: stubPushClient(func(_ context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		requests.Add(1)
		bodies <- message.Notification.Body
		return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
	})}
	if err := ns.RegisterPushToken(member.ID, "test-token", "test"); err != nil {
		t.Fatal(err)
	}

	ss.WithNotifier(ns)

	// Both cancellations must read missing preferences before either can insert.
	var arrivals atomic.Int32
	release := make(chan struct{})
	const callback = "test:concurrent_cancellation_preferences"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Table != "user_notification_preferences" {
			return
		}
		if arrivals.Add(1) == 2 {
			close(release)
		}
		select {
		case <-release:
		case <-time.After(5 * time.Second):
			tx.AddError(errors.New("concurrent preference creation timed out"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)

	var wg sync.WaitGroup
	for _, session := range sessions {
		wg.Add(1)
		go func(session *models.Session) {
			defer wg.Done()
			if _, err := ss.CancelSession(session.ID, "Venue closed: "+session.ID.String(), member.ID); err != nil {
				t.Error(err)
			}
		}(session)
	}
	wg.Wait()

	if arrivals.Load() != 2 {
		t.Fatalf("preference creation attempts = %d, want 2", arrivals.Load())
	}
	if requests.Load() != 2 {
		t.Fatalf("provider calls = %d, want 2", requests.Load())
	}
	close(bodies)
	delivered := make(map[uuid.UUID]int)
	for body := range bodies {
		for _, session := range sessions {
			if strings.Contains(body, session.ID.String()) {
				delivered[session.ID]++
			}
		}
	}
	for _, session := range sessions {
		if delivered[session.ID] != 1 {
			t.Fatalf("cancellation %s delivered %d times, want 1", session.ID, delivered[session.ID])
		}
	}
	cancellationCounts(t, 2, 2)
	var sent, preferences int64
	if err := database.DB.Model(&models.Notification{}).Where("push_sent = ?", true).Count(&sent).Error; err != nil || sent != 2 {
		t.Fatalf("sent notifications = %d, want 2: %v", sent, err)
	}
	if err := database.DB.Model(&models.UserNotificationPreferences{}).Where("user_id = ?", member.ID).Count(&preferences).Error; err != nil || preferences != 1 {
		t.Fatalf("preferences = %d, want 1: %v", preferences, err)
	}
}
