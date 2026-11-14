package services

import (
	"context"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"strings"
	"sync"
	"testing"
	"time"
)

func publicationAdmin(t *testing.T) models.User {
	t.Helper()
	actor := newUser(t, "publisher")
	if err := database.DB.Model(&actor).Update("role", models.RoleAdmin).Error; err != nil {
		t.Fatal(err)
	}
	actor.Role = models.RoleAdmin
	return actor
}
func publicationRows(t *testing.T, announcements, notifications, outboxes int64) {
	t.Helper()
	for _, item := range []struct {
		model any
		count int64
	}{{&models.Announcement{}, announcements}, {&models.Notification{}, notifications}, {&models.NotificationOutbox{}, outboxes}} {
		var count int64
		if err := database.DB.Model(item.model).Count(&count).Error; err != nil || count != item.count {
			t.Fatalf("%T count %d want %d: %v", item.model, count, item.count, err)
		}
	}
}
func TestAnnouncementAtomicAudienceAndRestartRetry(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	member := newUser(t, "member")
	pending := newUser(t, "pending")
	database.DB.Model(&pending).Update("membership_status", models.MembershipPending)
	addDeliveryDevice(t, actor, "actor-address")
	ns := &NotificationService{}
	service := NewAnnouncementService(ns)
	request := uuid.New()
	first, err := service.Publish(actor.ID, request, " Court closure ", "Friday's courts are closed.")
	if err != nil {
		t.Fatal(err)
	}
	if first.AudienceCount != 2 || first.QueuedCount != 1 || first.SuppressedCount != 1 || first.DeliveryStatus != "partially_queued" {
		t.Fatal(first)
	}
	publicationRows(t, 1, 2, 2)
	database.DB.Model(&pending).Update("membership_status", models.MembershipApproved)
	addDeliveryDevice(t, member, "new-member-address")
	retry, err := NewAnnouncementService(ns).Publish(actor.ID, request, "Court closure", "Friday's courts are closed.")
	if err != nil {
		t.Fatal(err)
	}
	if retry.ID != first.ID || retry.AudienceCount != first.AudienceCount || retry.QueuedCount != first.QueuedCount || retry.SuppressedCount != first.SuppressedCount {
		t.Fatal("retry changed publication snapshot", retry)
	}
	publicationRows(t, 1, 2, 2)
	if _, err := service.Publish(actor.ID, request, "Different message", "Friday's courts are closed."); !errors.Is(err, ErrAnnouncementRequestConflict) {
		t.Fatal(err)
	}
	var notices []models.Notification
	database.DB.Find(&notices)
	for _, notice := range notices {
		if notice.UserID == pending.ID || notice.PushSent {
			t.Fatal("late audience or delivery claim", notice)
		}
		if got := WithNotificationNavigation(notice).DestinationPath; got != "/notifications/"+notice.ID.String() {
			t.Fatal(got)
		}
	}
}
func TestAnnouncementConcurrentRequestPublishesExactlyOnce(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	newUser(t, "member")
	request := uuid.New()
	service := NewAnnouncementService(nil)
	var wg sync.WaitGroup
	ids := make(chan uuid.UUID, 8)
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := service.Publish(actor.ID, request, "News", "A club message")
			if err != nil {
				failures <- err
				return
			}
			ids <- result.ID
		}()
	}
	wg.Wait()
	close(ids)
	close(failures)
	for err := range failures {
		t.Error(err)
	}
	var first uuid.UUID
	for id := range ids {
		if first == uuid.Nil {
			first = id
		}
		if id != first {
			t.Fatal("duplicate publication")
		}
	}
	publicationRows(t, 1, 2, 2)
}
func TestAnnouncementFanoutFailureRollsBackAllRows(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	newUser(t, "member")
	const callback = "test:announcement_outbox_failure"
	calls := 0
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notification_outboxes" {
			calls++
			if calls == 2 {
				tx.AddError(errors.New("outbox unavailable"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)
	if _, err := NewAnnouncementService(nil).Publish(actor.ID, uuid.New(), "News", "Club message"); err == nil {
		t.Fatal("fanout failure accepted")
	}
	publicationRows(t, 0, 0, 0)
}
func TestAnnouncementProviderFailureDoesNotUndoPublication(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	addDeliveryDevice(t, actor, "active")
	calls := 0
	ns := &NotificationService{fcmEnabled: true, fcmClient: stubPushClient(func(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		calls++
		return nil, errors.New("network outcome unknown")
	})}
	request := uuid.New()
	result, err := NewAnnouncementService(ns).Publish(actor.ID, request, "News", "Club message")
	if err != nil {
		t.Fatal(err)
	}
	if calls != 0 || result.QueuedCount != 1 {
		t.Fatal("publication called provider or lost queue", calls, result)
	}
	if _, err := ns.ProcessNotificationWork(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatal(calls)
	}
	publicationRows(t, 1, 1, 1)
	retry, err := NewAnnouncementService(ns).Publish(actor.ID, request, "News", "Club message")
	if err != nil || retry.ID != result.ID || retry.QueuedCount != 1 {
		t.Fatal(retry, err)
	}
}
func TestAnnouncementPauseAndNoDeviceNeverReplay(t *testing.T) {
	for _, mode := range []string{"deployment", "club", "no_device"} {
		t.Run(mode, func(t *testing.T) {
			requireDB(t)
			actor := publicationAdmin(t)
			ns := &NotificationService{}
			if mode != "no_device" {
				addDeliveryDevice(t, actor, "active")
			}
			if mode == "deployment" {
				ns.disabled = true
			}
			if mode == "club" {
				database.DB.Model(&models.Club{}).Where("true").Update("notifications_paused", true)
			}
			result, err := NewAnnouncementService(ns).Publish(actor.ID, uuid.New(), "News", "Club message")
			if err != nil || result.SuppressedCount != 1 || result.QueuedCount != 0 {
				t.Fatal(result, err)
			}
			ns.disabled = false
			database.DB.Model(&models.Club{}).Where("true").Update("notifications_paused", false)
			if mode == "no_device" {
				addDeliveryDevice(t, actor, "new")
			}
			provider := &fakeDeliveryProvider{}
			d := &NotificationDispatcher{DB: database.DB, Provider: provider, Policy: ns.queueDeliveryPolicy}
			processDelivery(t, d)
			if len(provider.calls) != 0 {
				t.Fatal("suppressed publication replayed")
			}
			publicationRows(t, 1, 1, 1)
		})
	}
}
func TestAnnouncementSendTimeMembershipAndFixedAudience(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	member := newUser(t, "member")
	addDeliveryDevice(t, actor, "actor")
	addDeliveryDevice(t, member, "member")
	ns := &NotificationService{}
	result, err := NewAnnouncementService(ns).Publish(actor.ID, uuid.New(), "News", "Message")
	if err != nil || result.QueuedCount != 2 {
		t.Fatal(result, err)
	}
	database.DB.Model(&member).Update("membership_status", models.MembershipRemoved)
	newUser(t, "late-member")
	provider := &fakeDeliveryProvider{}
	d := &NotificationDispatcher{DB: database.DB, Provider: provider, Policy: ns.queueDeliveryPolicy}
	processDelivery(t, d)
	if provider.calls["actor"] != 1 || provider.calls["member"] != 0 {
		t.Fatal(provider.calls)
	}
	publicationRows(t, 1, 2, 2)
}
func TestAnnouncementValidationAndSenderAuthorization(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	player := newUser(t, "member")
	service := NewAnnouncementService(nil)
	if _, err := service.Publish(player.ID, uuid.New(), "Title", "Body"); !errors.Is(err, ErrAnnouncementNotAllowed) {
		t.Fatal(err)
	}
	for _, input := range []struct {
		id          uuid.UUID
		title, body string
	}{{uuid.Nil, "Title", "Body"}, {uuid.New(), " ", "Body"}, {uuid.New(), "Title", strings.Repeat("a", 5001)}} {
		if _, err := service.Publish(actor.ID, input.id, input.title, input.body); !errors.Is(err, ErrAnnouncementInvalid) {
			t.Fatal(err)
		}
	}
	publicationRows(t, 0, 0, 0)
}
func TestCancellationOutboxFailureRollsBackStatusAndAudience(t *testing.T) {
	_, sessions, _ := newTestServices(t)
	actor := newUser(t, "admin")
	newUser(t, "member")
	session := newSession(t, sessions, actor.ID, 1)
	const callback = "test:cancellation_outbox_failure"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notification_outboxes" {
			tx.AddError(errors.New("outbox unavailable"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)
	if _, err := sessions.CancelSession(session.ID, "Rain", actor.ID); err == nil {
		t.Fatal("cancellation accepted incomplete delivery intent")
	}
	var stored models.Session
	database.DB.First(&stored, "id = ?", session.ID)
	if stored.Status != models.SessionStatusOpen || stored.CancellationReason != "" {
		t.Fatal(stored)
	}
	publicationRows(t, 0, 0, 0)
}
func TestCancellationHistoryAndOutboxSurviveRestart(t *testing.T) {
	_, sessions, _ := newTestServices(t)
	actor := newUser(t, "actor")
	member := newUser(t, "member")
	addDeliveryDevice(t, actor, "actor")
	addDeliveryDevice(t, member, "member")
	if _, err := sessions.CancelSession(newSession(t, sessions, actor.ID, 1).ID, "Rain", actor.ID); err != nil {
		t.Fatal(err)
	}
	publicationRows(t, 1, 2, 2)
	provider := &fakeDeliveryProvider{}
	dispatcher := &NotificationDispatcher{DB: database.DB, Provider: provider, Policy: (&NotificationService{}).queueDeliveryPolicy, Now: func() time.Time { return time.Now().UTC() }}
	processDelivery(t, dispatcher)
	if provider.calls["actor"] != 1 || provider.calls["member"] != 1 {
		t.Fatal(provider.calls)
	}
	var notices []models.Notification
	database.DB.Find(&notices)
	for _, notice := range notices {
		if !strings.HasPrefix(WithNotificationNavigation(notice).DestinationPath, "/sessions/") {
			t.Fatal("cancellation lost session destination")
		}
	}
}

func TestAnnouncementMigrationDoesNotReplayHistoricalPublications(t *testing.T) {
	requireDB(t)
	actor := publicationAdmin(t)
	legacy := models.Announcement{Title: "Historical message", Body: "Already recorded", CreatedBy: actor.ID, SentAt: time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)}
	if err := database.DB.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	created, err := NewAnnouncementService(nil).Publish(actor.ID, uuid.New(), "Current", "Message")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := database.Migrate(); err != nil {
			t.Fatal(err)
		}
	}
	var saved models.Announcement
	if err := database.DB.First(&saved, "id = ?", legacy.ID).Error; err != nil {
		t.Fatal(err)
	}
	if saved.PublicationKey != nil || saved.Title != legacy.Title || !saved.SentAt.Equal(legacy.SentAt) {
		t.Fatal("migration rewrote historical publication", saved)
	}
	publicationRows(t, 2, 1, 1)
	var current models.Announcement
	if err := database.DB.First(&current, "id = ?", created.ID).Error; err != nil {
		t.Fatal(err)
	}
	if current.AudienceCount != created.AudienceCount || current.SuppressedCount != created.SuppressedCount || current.RequestID == nil || *current.RequestID != *created.RequestID {
		t.Fatal("migration reset new publication", current)
	}
}
