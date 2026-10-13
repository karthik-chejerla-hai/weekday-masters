package services

import (
	"context"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
	"strconv"
	"sync"
	"testing"
	"time"
)

type deliveryClock struct {
	mu    sync.Mutex
	value time.Time
}

func (c *deliveryClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.value }
func (c *deliveryClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = c.value.Add(d)
}

type fakeDeliveryProvider struct {
	mu      sync.Mutex
	calls   map[string]int
	results map[string][]DeliveryProviderResult
	ttl     []time.Duration
	hook    func(string)
}

func (p *fakeDeliveryProvider) Send(_ context.Context, _ models.Notification, token string, ttl time.Duration) DeliveryProviderResult {
	p.mu.Lock()
	if p.calls == nil {
		p.calls = map[string]int{}
	}
	index := p.calls[token]
	p.calls[token]++
	p.ttl = append(p.ttl, ttl)
	result := DeliveryProviderResult{State: models.DeliveryAccepted}
	if options := p.results[token]; len(options) > 0 {
		if index >= len(options) {
			index = len(options) - 1
		}
		result = options[index]
	}
	p.mu.Unlock()
	if p.hook != nil {
		p.hook(token)
	}
	return result
}
func deliveryFixture(t *testing.T) (*NotificationDispatcher, *deliveryClock, *fakeDeliveryProvider, models.User) {
	t.Helper()
	requireDB(t)
	if err := database.MigrateNotificationDeliverySchema(database.DB); err != nil {
		t.Fatal(err)
	}
	clock := &deliveryClock{value: time.Now().UTC().Truncate(time.Second)}
	provider := &fakeDeliveryProvider{results: map[string][]DeliveryProviderResult{}}
	dispatcher := &NotificationDispatcher{DB: database.DB, Provider: provider, Now: clock.now}
	return dispatcher, clock, provider, newUser(t, "delivery")
}
func addDeliveryDevice(t *testing.T, user models.User, token string) models.UserPushToken {
	t.Helper()
	device := models.UserPushToken{UserID: user.ID, Token: token}
	if err := database.DB.Create(&device).Error; err != nil {
		t.Fatal(err)
	}
	return device
}
func queueDelivery(t *testing.T, user models.User, key string, now time.Time) *models.NotificationOutbox {
	t.Helper()
	var outbox *models.NotificationOutbox
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		outbox, err = QueueNotificationWithin(tx, &models.Notification{UserID: user.ID, NotificationType: models.NotificationAdminAnnouncement, Title: "Hello", Body: "Announcement", Data: "{}"}, key, now, now.Add(24*time.Hour))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return outbox
}
func outboxState(t *testing.T, id uuid.UUID) models.DeliveryState {
	t.Helper()
	var outbox models.NotificationOutbox
	if err := database.DB.First(&outbox, "id = ?", id).Error; err != nil {
		t.Fatal(err)
	}
	return outbox.State
}
func processDelivery(t *testing.T, d *NotificationDispatcher) {
	t.Helper()
	if _, err := d.Process(context.Background(), 25); err != nil {
		t.Fatal(err)
	}
}

func TestDeliveryOutboxConcurrentAtomicIdentityAndRollback(t *testing.T) {
	_, clock, _, user := deliveryFixture(t)
	var group sync.WaitGroup
	failures := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			failures <- database.DB.Transaction(func(tx *gorm.DB) error {
				_, err := QueueNotificationWithin(tx, &models.Notification{UserID: user.ID, NotificationType: models.NotificationAdminAnnouncement, Title: "One", Body: "One", Data: "{}"}, "one-event", clock.now(), clock.now().Add(time.Hour))
				return err
			})
		}()
	}
	group.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	for _, model := range []any{&models.Notification{}, &models.NotificationOutbox{}} {
		var count int64
		if err := database.DB.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("duplicate records: %d", count)
		}
	}
	sentinel := errors.New("origin rollback")
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := QueueNotificationWithin(tx, &models.Notification{UserID: user.ID, Title: "Rollback", Body: "Rollback", Data: "{}"}, "rolled-back", clock.now(), clock.now().Add(time.Hour)); err != nil {
			return err
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	var count int64
	database.DB.Model(&models.NotificationOutbox{}).Where("event_key = ?", "rolled-back").Count(&count)
	if count != 0 {
		t.Fatal("rolled back origin queued work")
	}
}
func TestDeliveryConcurrentWorkersAndPartialRetries(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "accepted")
	addDeliveryDevice(t, user, "transient")
	provider.results["transient"] = []DeliveryProviderResult{{State: models.DeliveryTransient, Code: "transient"}, {State: models.DeliveryAccepted}}
	outbox := queueDelivery(t, user, "partial", clock.now())
	var group sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		group.Add(1)
		go func() { defer group.Done(); _, err := d.Process(context.Background(), 25); errs <- err }()
	}
	group.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if outboxState(t, outbox.ID) != models.DeliveryPending {
		t.Fatal("transient device did not remain pending")
	}
	if provider.calls["accepted"] != 1 || provider.calls["transient"] != 1 {
		t.Fatalf("duplicate initial submission: %v", provider.calls)
	}
	processDelivery(t, d)
	if provider.calls["transient"] != 1 {
		t.Fatal("retry before due")
	}
	clock.advance(time.Minute)
	processDelivery(t, d)
	if outboxState(t, outbox.ID) != models.DeliveryCompleted || provider.calls["accepted"] != 1 || provider.calls["transient"] != 2 {
		t.Fatalf("incorrect retry aggregation: %v", provider.calls)
	}
	if provider.ttl[len(provider.ttl)-1] != 24*time.Hour-time.Minute {
		t.Fatal("retry TTL not based on remaining expiry")
	}
}
func TestDeliveryRecoverUnsentAndFenceInflightUncertainty(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	device := addDeliveryDevice(t, user, "device")
	outbox := queueDelivery(t, user, "restart", clock.now())
	stale, err := d.claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	clock.advance(notificationLeaseDuration + time.Second)
	processDelivery(t, d)
	if provider.calls[device.Token] != 1 || outboxState(t, outbox.ID) != models.DeliveryCompleted {
		t.Fatal("unsent lease not recovered")
	}
	if _, err := d.aggregate(context.Background(), stale); !errors.Is(err, ErrNotificationLeaseLost) {
		t.Fatalf("stale worker could persist: %v", err)
	}
	second := queueDelivery(t, user, "inflight", clock.now())
	claimed, err := d.claim(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	attempt := models.NotificationDeliveryAttempt{OutboxID: second.ID, EnrollmentID: device.ID, State: models.DeliverySending, SendingOwner: claimed.LeaseOwner, AttemptCount: 1, NextAttemptAt: clock.now()}
	if err := database.DB.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	clock.advance(notificationLeaseDuration + time.Second)
	processDelivery(t, d)
	if outboxState(t, second.ID) != models.DeliveryUncertain || provider.calls[device.Token] != 1 {
		t.Fatal("inflight recovery replayed possibly accepted device")
	}
	if err := d.finishAttempt(context.Background(), claimed, &attempt, deliveryDevice{ID: device.ID, UserID: user.ID, Token: device.Token, Generation: 1}, DeliveryProviderResult{State: models.DeliveryAccepted}); !errors.Is(err, ErrNotificationLeaseLost) {
		t.Fatalf("stale acceptance persisted: %v", err)
	}
}
func TestDeliveryRetriesBoundedAndExpiry(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "always-transient")
	provider.results["always-transient"] = []DeliveryProviderResult{{State: models.DeliveryTransient, Code: "transient"}}
	outbox := queueDelivery(t, user, "retry-cap", clock.now())
	processDelivery(t, d)
	for _, delay := range deliveryRetryDelays {
		clock.advance(delay)
		processDelivery(t, d)
	}
	if provider.calls["always-transient"] != 4 || outboxState(t, outbox.ID) != models.DeliveryFailed {
		t.Fatalf("unbounded retries %v", provider.calls)
	}
	expired := queueDelivery(t, user, "expired", clock.now())
	clock.advance(24 * time.Hour)
	processDelivery(t, d)
	if provider.calls["always-transient"] != 4 || outboxState(t, expired.ID) != models.DeliveryExpired {
		t.Fatal("expired event submitted")
	}
}
func TestDeliveryRotatedTokenInvalidationIsConditional(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	device := addDeliveryDevice(t, user, "old")
	provider.results["old"] = []DeliveryProviderResult{{State: models.DeliveryFailed, Code: "unregistered", InvalidRegistration: true}}
	provider.hook = func(token string) {
		if token == "old" {
			if err := database.DB.Model(&models.UserPushToken{}).Where("id = ?", device.ID).Updates(map[string]any{"token": "rotated", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
				t.Error(err)
			}
		}
	}
	oldOutbox := queueDelivery(t, user, "invalid-old", clock.now())
	processDelivery(t, d)
	var active int64
	database.DB.Model(&models.UserPushToken{}).Where("id = ? AND token = ? AND invalidated_at IS NULL", device.ID, "rotated").Count(&active)
	if active != 1 {
		t.Fatal("old provider result invalidated rotated token")
	}
	if outboxState(t, oldOutbox.ID) != models.DeliveryPending {
		t.Fatal("rotated registration lost rejected event")
	}
	provider.hook = nil
	clock.advance(time.Minute)
	processDelivery(t, d)
	if provider.calls["old"] != 1 || provider.calls["rotated"] != 1 || outboxState(t, oldOutbox.ID) != models.DeliveryCompleted {
		t.Fatalf("rotated healthy address did not receive bounded retry: %v", provider.calls)
	}
	provider.calls["rotated"] = 0
	provider.results["rotated"] = []DeliveryProviderResult{{State: models.DeliveryTransient}, {State: models.DeliveryAccepted}}
	outbox := queueDelivery(t, user, "pending-rotation", clock.now())
	processDelivery(t, d)
	if err := database.DB.Model(&models.UserPushToken{}).Where("id = ?", device.ID).Updates(map[string]any{"token": "newest", "generation": gorm.Expr("generation + 1")}).Error; err != nil {
		t.Fatal(err)
	}
	clock.advance(time.Minute)
	processDelivery(t, d)
	if provider.calls["newest"] != 1 || outboxState(t, outbox.ID) != models.DeliveryCompleted {
		t.Fatal("pending attempt did not follow same-owner rotation")
	}
}
func TestDeliveryPolicyRecheckedBeforeEveryDevice(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "one")
	addDeliveryDevice(t, user, "two")
	stopped := false
	d.Policy = func(_ *gorm.DB, _ models.Notification, _ time.Time) (string, error) {
		if stopped {
			return "paused", nil
		}
		return "", nil
	}
	provider.hook = func(string) { stopped = true }
	outbox := queueDelivery(t, user, "stop-between-devices", clock.now())
	processDelivery(t, d)
	if len(provider.calls) != 1 || outboxState(t, outbox.ID) != models.DeliveryPartial {
		t.Fatalf("stop did not suppress unsent devices: %v", provider.calls)
	}
}
func TestNotificationMigrationVersionCutoffAndRollback(t *testing.T) {
	_, _, _, _ = deliveryFixture(t)
	calls := 0
	var cutoff time.Time
	apply := func(_ *gorm.DB, at time.Time) error { calls++; cutoff = at; return nil }
	if err := database.RunNotificationMigration(database.DB, "once", apply); err != nil {
		t.Fatal(err)
	}
	if err := database.RunNotificationMigration(database.DB, "once", apply); err != nil {
		t.Fatal(err)
	}
	var record models.NotificationMigrationRecord
	if err := database.DB.First(&record, "version = ?", "once").Error; err != nil {
		t.Fatal(err)
	}
	if calls != 1 || record.Cutoff.Sub(cutoff) >= time.Microsecond || record.Cutoff.Sub(cutoff) <= -time.Microsecond {
		t.Fatal("migration repeated or cutoff changed")
	}
	if err := database.RunNotificationMigration(database.DB, "failure", func(*gorm.DB, time.Time) error { return errors.New("failure") }); err == nil {
		t.Fatal("failure accepted")
	}
	var count int64
	database.DB.Model(&models.NotificationMigrationRecord{}).Where("version = ?", "failure").Count(&count)
	if count != 0 {
		t.Fatal("failed migration marked complete")
	}
}

// The FCM wrapper carries TTL and a stable event tag for every submission.
type ttlPushClient struct{ message *messaging.MulticastMessage }

func (c *ttlPushClient) SendEachForMulticast(_ context.Context, m *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	c.message = m
	return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
}
func TestFCMDeliveryRemainingTTLAndTag(t *testing.T) {
	client := &ttlPushClient{}
	service := &NotificationService{fcmClient: client, fcmEnabled: true, frontendURL: "https://rally.example"}
	notification := models.Notification{ID: uuid.New(), NotificationType: models.NotificationAdminAnnouncement, Title: "Hello", Body: "Hello", Data: "{}"}
	result := (notificationFCMProvider{service}).Send(context.Background(), notification, "private", 61*time.Second+time.Millisecond)
	if result.State != models.DeliveryAccepted || client.message.Webpush.Headers["TTL"] != "61" || client.message.Webpush.Notification.Tag != notification.ID.String() {
		t.Fatal("FCM lifespan contract missing")
	}
	client.message = nil
	result = (notificationFCMProvider{service}).Send(context.Background(), notification, "private", time.Millisecond)
	if result.State != models.DeliveryExpired || client.message != nil {
		t.Fatal("near-expired message submitted")
	}
}

func TestDeliverySuppressedOriginNeverReplaysAfterPauseLift(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "device")
	client := &ttlPushClient{}
	service := &NotificationService{disabled: true, fcmEnabled: true, fcmClient: client}
	var outbox *models.NotificationOutbox
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		outbox, err = service.QueueNotificationWithin(tx, &models.Notification{UserID: user.ID, NotificationType: models.NotificationAdminAnnouncement, Title: "Paused", Body: "Paused", Data: "{}"}, "paused-origin", clock.now(), clock.now().Add(time.Hour))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if outbox.State != models.DeliverySuppressed {
		t.Fatal("paused origin not suppressed")
	}
	service.disabled = false
	processDelivery(t, d)
	if len(provider.calls) != 0 || outboxState(t, outbox.ID) != models.DeliverySuppressed {
		t.Fatal("unpausing replayed suppressed history")
	}
}
func TestDeliveryAcceptedAndUncertainDevicesRemainPartial(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "accepted")
	addDeliveryDevice(t, user, "uncertain")
	provider.results["uncertain"] = []DeliveryProviderResult{{State: models.DeliveryUncertain, Code: "timeout"}}
	outbox := queueDelivery(t, user, "uncertain-partial", clock.now())
	processDelivery(t, d)
	clock.advance(time.Minute)
	processDelivery(t, d)
	if outboxState(t, outbox.ID) != models.DeliveryPartial || provider.calls["accepted"] != 1 || provider.calls["uncertain"] != 1 {
		t.Fatalf("terminal uncertainty replayed: %v", provider.calls)
	}
}
func TestDeliveryAcceptancePersistenceFailureRecoversUncertain(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "accepted")
	outbox := queueDelivery(t, user, "persistence-failure", clock.now())
	const callback = "test:fail_delivery_acceptance"
	if err := database.DB.Callback().Update().Before("gorm:update").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema != nil && tx.Statement.Schema.Table == "notification_delivery_attempts" {
			if changes, ok := tx.Statement.Dest.(map[string]any); ok && changes["state"] == models.DeliveryAccepted {
				tx.AddError(errors.New("acceptance database unavailable"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, err := d.Process(context.Background(), 25)
	if err == nil {
		t.Fatal("expected acceptance persistence failure")
	}
	database.DB.Callback().Update().Remove(callback)
	clock.advance(notificationLeaseDuration + time.Second)
	processDelivery(t, d)
	if outboxState(t, outbox.ID) != models.DeliveryUncertain || provider.calls["accepted"] != 1 {
		t.Fatal("unknown acceptance automatically resent")
	}
}
func TestDeliveryProviderCalledOutsideOutboxLocks(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "device")
	outbox := queueDelivery(t, user, "locks", clock.now())
	provider.hook = func(string) {
		if err := database.DB.Transaction(func(tx *gorm.DB) error {
			var current models.NotificationOutbox
			return tx.Raw("SELECT * FROM notification_outboxes WHERE id = ? FOR UPDATE NOWAIT", outbox.ID).Scan(&current).Error
		}); err != nil {
			t.Error("provider invoked under outbox lock", err)
		}
	}
	processDelivery(t, d)
}

func TestDeliveryRemovedMembershipRecheckedBeforeNextDevice(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "one")
	addDeliveryDevice(t, user, "two")
	service := &NotificationService{fcmEnabled: true, fcmClient: &ttlPushClient{}}
	d.Policy = service.queueDeliveryPolicy
	provider.hook = func(string) {
		if err := database.DB.Model(&models.User{}).Where("id = ?", user.ID).Update("membership_status", models.MembershipRemoved).Error; err != nil {
			t.Error(err)
		}
	}
	outbox := queueDelivery(t, user, "membership-change", clock.now())
	processDelivery(t, d)
	if len(provider.calls) != 1 || outboxState(t, outbox.ID) != models.DeliveryPartial {
		t.Fatal("removed member received next device submission")
	}
}
func TestDeliveryNoDevicesSuppressesWithoutInferringDefaultPreferences(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	if err := database.DB.Create(&models.UserNotificationPreferences{UserID: user.ID}).Error; err != nil {
		t.Fatal(err)
	}
	outbox := queueDelivery(t, user, "no-devices", clock.now())
	processDelivery(t, d)
	if outboxState(t, outbox.ID) != models.DeliverySuppressed || len(provider.calls) != 0 {
		t.Fatal("default preferences implied delivery availability")
	}
}
func TestDeliveryInvalidatedTokenRetiresOnlyAttemptedRegistration(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	device := addDeliveryDevice(t, user, "invalid")
	other := addDeliveryDevice(t, user, "good")
	provider.results["invalid"] = []DeliveryProviderResult{{State: models.DeliveryFailed, Code: "unregistered", InvalidRegistration: true}}
	queueDelivery(t, user, "invalid-registration", clock.now())
	processDelivery(t, d)
	var invalidated, active int64
	database.DB.Model(&models.UserPushToken{}).Where("id = ? AND invalidated_at IS NOT NULL", device.ID).Count(&invalidated)
	database.DB.Model(&models.UserPushToken{}).Where("id = ? AND invalidated_at IS NULL", other.ID).Count(&active)
	if invalidated != 1 || active != 1 {
		t.Fatal("provider invalidation affected wrong device")
	}
}

func TestDeliveryOriginIndependentOfAPIProviderInitialization(t *testing.T) {
	d, clock, provider, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "active")
	apiService := &NotificationService{} // API process may fail Firebase initialization.
	var outbox *models.NotificationOutbox
	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		outbox, err = apiService.QueueNotificationWithin(tx, &models.Notification{UserID: user.ID, NotificationType: models.NotificationAdminAnnouncement, Title: "Saved", Body: "Durable", Data: "{}"}, "api-provider-unavailable", clock.now(), clock.now().Add(time.Hour))
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if outbox.State != models.DeliveryPending {
		t.Fatalf("API provider failure discarded intent: %s", outbox.State)
	}
	if summary, err := apiService.ProcessNotificationWork(context.Background(), 25); !errors.Is(err, ErrPushUnavailable) || summary.Claimed != 0 {
		t.Fatalf("uninitialized worker claimed intent: %+v %v", summary, err)
	}
	var untouched models.NotificationOutbox
	database.DB.First(&untouched, "id = ?", outbox.ID)
	if untouched.State != models.DeliveryPending || untouched.LeaseGeneration != 0 {
		t.Fatal("unavailable worker changed intent")
	}
	processDelivery(t, d)
	if outboxState(t, outbox.ID) != models.DeliveryCompleted || provider.calls["active"] != 1 {
		t.Fatal("healthy independent worker could not deliver retained intent")
	}
}
func TestDeliveryUninitializedDispatcherDoesNotClaim(t *testing.T) {
	d, clock, _, user := deliveryFixture(t)
	addDeliveryDevice(t, user, "active")
	outbox := queueDelivery(t, user, "missing-worker-provider", clock.now())
	d.Provider = nil
	summary, err := d.Process(context.Background(), 25)
	if !errors.Is(err, ErrPushUnavailable) || summary.Claimed != 0 || outboxState(t, outbox.ID) != models.DeliveryPending {
		t.Fatalf("nil provider claimed event: %+v %v", summary, err)
	}
}
func TestRejectedRotatingAddressRetriesAreBoundedByExpiryAndCount(t *testing.T) {
	for _, mode := range []string{"exhaustion", "expiry", "owner_changed", "revoked", "accepted", "uncertain"} {
		t.Run(mode, func(t *testing.T) {
			d, clock, provider, user := deliveryFixture(t)
			device := addDeliveryDevice(t, user, "address-0")
			outbox := queueDelivery(t, user, "rotating-address-"+mode, clock.now())
			owner := newUser(t, "other")
			if mode == "expiry" {
				if err := database.DB.Model(outbox).Update("expires_at", clock.now().Add(30*time.Second)).Error; err != nil {
					t.Fatal(err)
				}
			}
			generation := 0
			provider.hook = func(token string) {
				generation++
				changes := map[string]any{"token": "address-" + strconv.Itoa(generation), "generation": gorm.Expr("generation + 1")}
				if mode == "owner_changed" {
					changes["user_id"] = owner.ID
				}
				if mode == "revoked" {
					changes["revoked_at"] = clock.now()
				}
				if err := database.DB.Model(&models.UserPushToken{}).Where("id = ?", device.ID).Updates(changes).Error; err != nil {
					t.Error(err)
				}
			}
			for i := 0; i < 5; i++ {
				provider.results["address-"+strconv.Itoa(i)] = []DeliveryProviderResult{{State: models.DeliveryFailed, Code: "unregistered", InvalidRegistration: true}}
			}
			if mode == "accepted" {
				provider.results["address-0"] = []DeliveryProviderResult{{State: models.DeliveryAccepted, InvalidRegistration: true}}
			}
			if mode == "uncertain" {
				provider.results["address-0"] = []DeliveryProviderResult{{State: models.DeliveryUncertain, InvalidRegistration: true}}
			}
			processDelivery(t, d)
			for _, delay := range deliveryRetryDelays {
				clock.advance(delay)
				processDelivery(t, d)
			}
			calls := 0
			for _, count := range provider.calls {
				calls += count
			}
			wantState, wantCalls := models.DeliveryFailed, 1
			switch mode {
			case "exhaustion":
				wantCalls = 4
			case "expiry":
				wantState = models.DeliveryExpired
			case "accepted":
				wantState = models.DeliveryCompleted
			case "uncertain":
				wantState = models.DeliveryUncertain
			}
			if calls != wantCalls || outboxState(t, outbox.ID) != wantState {
				t.Fatalf("mode %s calls %d state %s want %d %s", mode, calls, outboxState(t, outbox.ID), wantCalls, wantState)
			}
			if mode == "accepted" || mode == "uncertain" {
				var count int64
				database.DB.Model(&models.UserPushToken{}).Where("id = ? AND invalidated_at IS NULL", device.ID).Count(&count)
				if count != 1 {
					t.Fatal("terminal acceptance/uncertainty invalidated healthy rotation")
				}
			}
		})
	}
}
