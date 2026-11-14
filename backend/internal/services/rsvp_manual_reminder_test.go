package services

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

func manualReminderFixture(t *testing.T) (*NotificationService, models.User, *models.Session) {
	t.Helper()
	s, ss, actor := rsvpScannerFixture(t)
	actor.Role = models.RoleAdmin
	if err := database.DB.Save(&actor).Error; err != nil {
		t.Fatal(err)
	}
	session := scannerSession(t, ss, actor, utils.NowInSydney().AddDate(0, 0, 7).Truncate(time.Minute))
	return s, actor, session
}
func TestManualRSVPReminderAudienceRetryAndCooldown(t *testing.T) {
	s, actor, session := manualReminderFixture(t)
	addDeliveryDevice(t, actor, "admin-device")
	for _, status := range []models.RSVPStatus{models.RSVPStatusIn, models.RSVPStatusOut, models.RSVPStatusMaybe, models.RSVPStatusWaitlisted} {
		member := newUser(t, string(status))
		if err := database.DB.Create(&models.RSVP{SessionID: session.ID, UserID: member.ID, Status: status, RSVPTimestamp: time.Now()}).Error; err != nil {
			t.Fatal(err)
		}
	}
	pending := newUser(t, "pending")
	database.DB.Model(&pending).Update("membership_status", models.MembershipPending)
	newUser(t, "no-device")
	preview, err := s.PreviewManualRSVPReminder(context.Background(), actor.ID, session.ID)
	if err != nil || preview.AudienceCount != 2 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	request := uuid.New()
	batch, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, request)
	if err != nil || batch.AudienceCount != 2 || batch.QueuedCount != 1 || batch.SuppressedCount != 1 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	retry, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, request)
	if err != nil || retry.ID != batch.ID {
		t.Fatalf("retry=%+v err=%v", retry, err)
	}
	var count int64
	database.DB.Model(&models.Notification{}).Count(&count)
	if count != 2 {
		t.Fatalf("created %d histories", count)
	}
	other := newUser(t, "other-admin")
	other.Role = models.RoleAdmin
	database.DB.Save(&other)
	_, err = s.SendManualRSVPReminder(context.Background(), other.ID, session.ID, uuid.New())
	var cooldown *RSVPReminderCooldownError
	if !errors.As(err, &cooldown) {
		t.Fatalf("expected shared cooldown: %v", err)
	}
	database.DB.Model(batch).Update("created_at", time.Now().Add(-time.Hour-time.Second))
	if _, err := s.SendManualRSVPReminder(context.Background(), other.ID, session.ID, uuid.New()); err != nil {
		t.Fatal(err)
	}
}
func TestManualRSVPReminderConcurrentAdmins(t *testing.T) {
	s, actor, session := manualReminderFixture(t)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, uuid.New())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			var cooldown *RSVPReminderCooldownError
			if !errors.As(err, &cooldown) {
				t.Fatal(err)
			}
		}
	}
	if successes != 1 {
		t.Fatalf("published %d batches", successes)
	}
}
func TestManualRSVPReminderEligibilityAndDispatch(t *testing.T) {
	for _, change := range []string{"in", "out", "maybe", "waitlisted", "cancelled", "deadline", "started", "removed", "paused"} {
		t.Run(change, func(t *testing.T) {
			s, actor, session := manualReminderFixture(t)
			addDeliveryDevice(t, actor, "device")
			if _, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, uuid.New()); err != nil {
				t.Fatal(err)
			}
			switch change {
			case "in", "out", "maybe", "waitlisted":
				database.DB.Create(&models.RSVP{SessionID: session.ID, UserID: actor.ID, Status: models.RSVPStatus(change), RSVPTimestamp: time.Now()})
			case "cancelled":
				database.DB.Model(session).UpdateColumn("status", models.SessionStatusCancelled)
			case "deadline":
				database.DB.Model(session).UpdateColumn("rsvp_deadline", time.Now().Add(-time.Second))
			case "started":
				database.DB.Model(session).UpdateColumn("starts_at", time.Now().Add(-time.Second))
			case "removed":
				database.DB.Model(&actor).UpdateColumn("membership_status", models.MembershipRemoved)
			case "paused":
				database.DB.Model(&models.Club{}).Where("1=1").Update("notifications_paused", true)
			}
			provider := &recordedReminderProvider{}
			if _, err := reminderDispatcher(s, time.Now, provider).Process(context.Background(), 10); err != nil {
				t.Fatal(err)
			}
			if len(provider.sends) != 0 {
				t.Fatal("ineligible reminder reached provider")
			}
		})
	}
}
func TestManualRSVPReminderDeliversThroughWorker(t *testing.T) {
	s, actor, session := manualReminderFixture(t)
	addDeliveryDevice(t, actor, "device")
	if _, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	provider := &recordedReminderProvider{}
	if _, err := reminderDispatcher(s, time.Now, provider).Process(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if len(provider.sends) != 1 {
		t.Fatalf("expected one send, got %d", len(provider.sends))
	}
}
func TestManualRSVPReminderRejectsClosedAndUnauthorized(t *testing.T) {
	s, actor, session := manualReminderFixture(t)
	member := newUser(t, "player")
	if _, err := s.SendManualRSVPReminder(context.Background(), member.ID, session.ID, uuid.New()); !errors.Is(err, ErrRSVPReminderNotAllowed) {
		t.Fatal(err)
	}
	for _, change := range []string{"deadline", "cancelled", "closed", "started"} {
		original := *session
		switch change {
		case "deadline":
			database.DB.Model(session).UpdateColumn("rsvp_deadline", time.Now())
		case "cancelled":
			database.DB.Model(session).UpdateColumn("status", models.SessionStatusCancelled)
		case "closed":
			database.DB.Model(session).UpdateColumn("status", models.SessionStatusClosed)
		case "started":
			database.DB.Model(session).UpdateColumn("starts_at", time.Now())
		}
		if _, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, uuid.New()); !errors.Is(err, ErrRSVPReminderUnavailable) {
			t.Fatalf("%s: %v", change, err)
		}
		if err := database.DB.Save(&original).Error; err != nil {
			t.Fatal(err)
		}
	}
}

func TestManualRSVPReminderRechecksPreviewAndDoesNotConsumeEmptyCooldown(t *testing.T) {
	s, actor, session := manualReminderFixture(t)
	ctx := context.Background()
	preview, err := s.PreviewManualRSVPReminder(ctx, actor.ID, session.ID)
	if err != nil || preview.AudienceCount != 1 {
		t.Fatalf("preview=%+v err=%v", preview, err)
	}
	response := models.RSVP{SessionID: session.ID, UserID: actor.ID, Status: models.RSVPStatusOut, RSVPTimestamp: time.Now()}
	if err := database.DB.Create(&response).Error; err != nil {
		t.Fatal(err)
	}
	batch, err := s.SendManualRSVPReminder(ctx, actor.ID, session.ID, uuid.New())
	if err != nil || batch.AudienceCount != 0 || batch.NextAllowedAt != nil {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	member := newUser(t, "new-recipient")
	batch, err = s.SendManualRSVPReminder(ctx, actor.ID, session.ID, uuid.New())
	if err != nil || batch.AudienceCount != 1 {
		t.Fatalf("batch=%+v err=%v", batch, err)
	}
	var notification models.Notification
	if err := database.DB.Where("user_id = ?", member.ID).First(&notification).Error; err != nil {
		t.Fatal(err)
	}
}
func TestManualRSVPReminderConcurrentSameRequestAndConflict(t *testing.T) {
	s, actor, session := manualReminderFixture(t)
	request := uuid.New()
	var wg sync.WaitGroup
	batches := make(chan *models.RSVPReminderBatch, 4)
	failures := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			batch, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, request)
			batches <- batch
			failures <- err
		}()
	}
	wg.Wait()
	close(batches)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	var first uuid.UUID
	for batch := range batches {
		if first == uuid.Nil {
			first = batch.ID
		}
		if first != batch.ID {
			t.Fatal("retry created a different batch")
		}
	}
	other := newUser(t, "another-admin")
	other.Role = models.RoleAdmin
	if err := database.DB.Save(&other).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendManualRSVPReminder(context.Background(), other.ID, session.ID, request); !errors.Is(err, ErrRSVPReminderRequest) {
		t.Fatalf("expected request conflict: %v", err)
	}
}

func TestManualRSVPReminderRetrySurvivesSessionClosureAndDeletion(t *testing.T) {
	for _, change := range []string{"deadline", "cancelled", "started", "deleted"} {
		t.Run(change, func(t *testing.T) {
			s, actor, session := manualReminderFixture(t)
			request := uuid.New()
			original, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, request)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "deadline":
				err = database.DB.Model(session).UpdateColumn("rsvp_deadline", time.Now().Add(-time.Second)).Error
			case "cancelled":
				err = database.DB.Model(session).UpdateColumn("status", models.SessionStatusCancelled).Error
			case "started":
				err = database.DB.Model(session).UpdateColumn("starts_at", time.Now().Add(-time.Second)).Error
			case "deleted":
				err = NewSessionService().DeleteSession(session.ID)
			}
			if err != nil {
				t.Fatal(err)
			}
			retry, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, request)
			if err != nil || retry.ID != original.ID || retry.AudienceCount != original.AudienceCount || retry.QueuedCount != original.QueuedCount || retry.SuppressedCount != original.SuppressedCount {
				t.Fatalf("saved result changed: original=%+v retry=%+v err=%v", original, retry, err)
			}
			var batches, notifications int64
			if err := database.DB.Model(&models.RSVPReminderBatch{}).Count(&batches).Error; err != nil {
				t.Fatal(err)
			}
			if err := database.DB.Model(&models.Notification{}).Count(&notifications).Error; err != nil {
				t.Fatal(err)
			}
			if batches != 1 || notifications != int64(original.AudienceCount) {
				t.Fatalf("retry created work: batches=%d notifications=%d", batches, notifications)
			}
			if _, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, uuid.New()); err == nil {
				t.Fatal("new request accepted for unavailable session")
			}
			// Recovering an existing batch still requires the original, approved admin.
			other := newUser(t, "other-admin")
			other.Role = models.RoleAdmin
			if err := database.DB.Save(&other).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.SendManualRSVPReminder(context.Background(), other.ID, session.ID, request); !errors.Is(err, ErrRSVPReminderRequest) {
				t.Fatalf("another admin recovered a private request: %v", err)
			}
			if err := database.DB.Model(&actor).UpdateColumn("role", models.RolePlayer).Error; err != nil {
				t.Fatal(err)
			}
			if _, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, request); !errors.Is(err, ErrRSVPReminderNotAllowed) {
				t.Fatalf("demoted actor recovered result: %v", err)
			}
		})
	}
}

func TestManualRSVPReminderFarFutureCapsProviderTTL(t *testing.T) {
	s, actor, _ := manualReminderFixture(t)
	session := scannerSession(t, NewSessionService(), actor, utils.NowInSydney().AddDate(0, 0, 60).Truncate(time.Minute))
	addDeliveryDevice(t, actor, "future-session-device")
	if _, err := s.SendManualRSVPReminder(context.Background(), actor.ID, session.ID, uuid.New()); err != nil {
		t.Fatal(err)
	}
	var outbox models.NotificationOutbox
	if err := database.DB.First(&outbox).Error; err != nil {
		t.Fatal(err)
	}
	if !outbox.ExpiresAt.Equal(session.RSVPDeadline) {
		t.Fatal("provider limit shortened the business event lifetime")
	}
	if _, err := reminderDispatcher(s, time.Now, notificationFCMProvider{s}).Process(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	client := s.fcmClient.(*ttlPushClient)
	if client.message == nil || client.message.Webpush.Headers["TTL"] != "2419200" {
		t.Fatalf("provider TTL was not capped: %+v", client.message)
	}
}
