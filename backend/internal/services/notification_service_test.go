package services

import (
	"context"
	"encoding/json"
	"errors"
	"firebase.google.com/go/v4/messaging"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"gorm.io/gorm"
)

func TestNotificationService_PreferencesAndTokens(t *testing.T) {
	requireDB(t)

	ns := NewNotificationService(NotificationConfig{
		FrontendURL: "https://rally.test",
	})
	user := newUser(t, "notifplayer")

	// Get default preferences
	prefs, err := ns.GetUserPreferences(user.ID)
	if err != nil {
		t.Fatalf("failed to get preferences: %v", err)
	}
	if !prefs.PushEnabled || prefs.EmailEnabled {
		t.Fatal("expected push enabled and email disabled")
	}

	// Update preferences
	updatedPrefs, err := ns.UpdateUserPreferences(user.ID, map[string]interface{}{
		"push_enabled":  false,
		"email_enabled": true,
	})
	if err != nil {
		t.Fatalf("failed to update preferences: %v", err)
	}
	if updatedPrefs.PushEnabled {
		t.Fatal("expected push_enabled to be false")
	}

	// Register Push Tokens
	token1 := "fcm_token_sample_1"
	if err := ns.RegisterPushToken(user.ID, token1, "iPhone 15"); err != nil {
		t.Fatalf("failed to register token 1: %v", err)
	}

	// Registering same token updates last used
	if err := ns.RegisterPushToken(user.ID, token1, "iPhone 15 Pro"); err != nil {
		t.Fatalf("failed to re-register token: %v", err)
	}

	// Unregister token
	if err := ns.UnregisterPushToken(user.ID, token1); err != nil {
		t.Fatalf("failed to unregister token: %v", err)
	}
}

func TestNotificationService_SendAndHistory(t *testing.T) {
	requireDB(t)

	ns := NewNotificationService(NotificationConfig{
		FrontendURL: "https://rally.test",
	})
	user := newUser(t, "historyplayer")

	// Send notification (stores record in database)
	err := ns.SendNotification(
		context.Background(),
		user.ID,
		models.NotificationWaitlistUpdate,
		"Spot Available!",
		"You have been promoted to confirmed player.",
		map[string]string{"session_id": uuid.NewString()},
	)
	if err != nil {
		t.Fatalf("failed to send notification: %v", err)
	}

	// Retrieve history
	history, err := ns.GetUserNotifications(user.ID, 10, 0)
	if err != nil {
		t.Fatalf("failed to get notification history: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 notification in history, got %d", len(history))
	}
	if history[0].Title != "Spot Available!" {
		t.Fatalf("unexpected title: %q", history[0].Title)
	}
	if history[0].ReadAt != nil {
		t.Fatal("new notification should be unread")
	}

	// Mark as read
	if err := ns.MarkNotificationRead(history[0].ID, user.ID); err != nil {
		t.Fatalf("failed to mark notification read: %v", err)
	}

	updatedHistory, err := ns.GetUserNotifications(user.ID, 10, 0)
	if err != nil || len(updatedHistory) == 0 || updatedHistory[0].ReadAt == nil {
		t.Fatal("expected notification to have read_at timestamp")
	}
}

func TestGetUserPreferencesPreservesConcurrentOptOut(t *testing.T) {
	requireDB(t)
	ns := NewNotificationService(NotificationConfig{})
	user := newUser(t, "preferences")
	concurrent := models.UserNotificationPreferences{UserID: user.ID}

	// Simulate another request saving an opt-out after the missing-row read,
	// but before this request can insert its defaults.
	var inserted atomic.Bool
	const callback = "test:concurrent_preference_opt_out"
	if err := database.DB.Callback().Create().Before("gorm:create").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Table != "user_notification_preferences" || inserted.Swap(true) {
			return
		}
		tx.AddError(database.DB.Transaction(func(other *gorm.DB) error {
			if err := other.Create(&concurrent).Error; err != nil {
				return err
			}
			return other.Model(&concurrent).Updates(map[string]interface{}{
				"push_enabled": false, "email_enabled": false,
			}).Error
		}))
	}); err != nil {
		t.Fatal(err)
	}
	defer database.DB.Callback().Create().Remove(callback)

	prefs, err := ns.GetUserPreferences(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	if prefs.ID != concurrent.ID || prefs.PushEnabled || prefs.EmailEnabled {
		t.Fatalf("did not return the saved opt-out: %+v", prefs)
	}
}

type stubPushClient func(context.Context, *messaging.MulticastMessage) (*messaging.BatchResponse, error)

func (f stubPushClient) SendEachForMulticast(ctx context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
	return f(ctx, msg)
}

func TestPushDeliveryRequiresAcceptedDevice(t *testing.T) {
	for _, mode := range []string{"no_devices", "all_failed", "partial", "accepted"} {
		t.Run(mode, func(t *testing.T) {
			requireDB(t)
			user := newUser(t, "pushplayer")
			calls := 0
			sessionID := uuid.NewString()
			ns := &NotificationService{fcmEnabled: true, frontendURL: "https://rally.test", fcmClient: stubPushClient(func(_ context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
				calls++
				if msg.Webpush.FCMOptions.Link != "https://rally.test/sessions/"+sessionID {
					t.Fatal("wrong notification link")
				}
				if mode == "all_failed" {
					return &messaging.BatchResponse{FailureCount: 1, Responses: []*messaging.SendResponse{{Error: errors.New("rejected")}}}, nil
				}
				if mode == "partial" {
					return &messaging.BatchResponse{SuccessCount: 1, FailureCount: 1, Responses: []*messaging.SendResponse{{Success: true}, {Error: errors.New("rejected")}}}, nil
				}
				return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
			})}
			if mode != "no_devices" {
				if err := ns.RegisterPushToken(user.ID, "token-1", "test"); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "partial" {
				if err := ns.RegisterPushToken(user.ID, "token-2", "test"); err != nil {
					t.Fatal(err)
				}
			}
			if err := ns.SendNotification(context.Background(), user.ID, models.NotificationSessionReminder, "Reminder", "Play", map[string]string{"session_id": sessionID}); err != nil {
				t.Fatal(err)
			}
			notices, err := ns.GetUserNotifications(user.ID, 10, 0)
			if err != nil || len(notices) != 1 {
				t.Fatalf("history: %v, %v", notices, err)
			}
			wantSent := mode == "partial" || mode == "accepted"
			if notices[0].PushSent != wantSent || (notices[0].PushSentAt != nil) != wantSent || notices[0].EmailSent {
				t.Fatalf("wrong delivery state: %+v", notices[0])
			}
			if mode == "no_devices" && calls != 0 {
				t.Fatal("called FCM without devices")
			}
		})
	}
}

func TestEmailCredentialsDoNotEnableAlerts(t *testing.T) {
	ns := NewNotificationService(NotificationConfig{SendGridAPIKey: "unused"})
	if ns.IsEnabled() {
		t.Fatal("email credentials enabled automatic notifications")
	}
}

func TestFCMUsesApplicationDefaultCredentials(t *testing.T) {
	// Authorized-user ADC is enough to initialize; this test never contacts Google.
	path := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(path, []byte(`{"type":"authorized_user","client_id":"test","client_secret":"test","refresh_token":"test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	ns := NewNotificationService(NotificationConfig{FirebaseProjectID: "test-project"})
	if !ns.IsEnabled() || ns.fcmClient == nil {
		t.Fatal("project configuration did not enable FCM using ADC")
	}
}

func TestDisabledDeliveryStillInitializesFCMForExplicitTests(t *testing.T) {
	path := filepath.Join(t.TempDir(), "adc.json")
	if err := os.WriteFile(path, []byte(`{"type":"authorized_user","client_id":"test","client_secret":"test","refresh_token":"test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", path)
	ns := NewNotificationService(NotificationConfig{Disabled: true, FirebaseProjectID: "test-project"})
	if ns.IsEnabled() {
		t.Fatal("deployment-wide delivery became enabled")
	}
	if !ns.fcmEnabled || ns.fcmClient == nil {
		t.Fatal("FCM was not initialized for an explicit self-test")
	}
}

func TestSendTestPushTargetsOnlyTheCallerWhileNotificationsAreDisabled(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "pushadmin")
	other := newUser(t, "othermember")
	var sentTokens []string
	ns := &NotificationService{
		disabled: true, fcmEnabled: true, frontendURL: "https://rally.test",
		fcmClient: stubPushClient(func(_ context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
			sentTokens = append(sentTokens, msg.Tokens...)
			if msg.Data["type"] != "push_test" || msg.Webpush.FCMOptions.Link != "https://rally.test/dashboard" {
				t.Fatalf("wrong test payload: %+v", msg)
			}
			return &messaging.BatchResponse{SuccessCount: len(msg.Tokens), Responses: []*messaging.SendResponse{{Success: true}}}, nil
		}),
	}
	if err := ns.RegisterPushToken(admin.ID, "admin-token", "Admin browser"); err != nil {
		t.Fatal(err)
	}
	if err := ns.RegisterPushToken(other.ID, "other-token", "Other browser"); err != nil {
		t.Fatal(err)
	}

	result, err := ns.SendTestPush(context.Background(), admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.AcceptedDevices != 1 || result.AttemptedDevices != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(sentTokens) != 1 || sentTokens[0] != "admin-token" {
		t.Fatalf("test escaped the caller's devices: %v", sentTokens)
	}
	var historyCount int64
	if err := database.DB.Model(&models.Notification{}).Count(&historyCount).Error; err != nil || historyCount != 0 {
		t.Fatalf("test push created history: count=%d err=%v", historyCount, err)
	}
}

func TestSendTestPushHonoursAccountOptOutAndRequiresADevice(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "pushadmin")
	calls := 0
	ns := &NotificationService{fcmEnabled: true, fcmClient: stubPushClient(func(_ context.Context, _ *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		calls++
		return &messaging.BatchResponse{}, nil
	})}

	if _, err := ns.SendTestPush(context.Background(), admin.ID); !errors.Is(err, ErrNoPushDevices) {
		t.Fatalf("expected no-device error, got %v", err)
	}
	if _, err := ns.UpdateUserPreferences(admin.ID, map[string]interface{}{"push_enabled": false}); err != nil {
		t.Fatal(err)
	}
	if _, err := ns.SendTestPush(context.Background(), admin.ID); !errors.Is(err, ErrPushDisabledForUser) {
		t.Fatalf("expected opt-out error, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("called FCM %d times despite setup errors", calls)
	}
}

func TestDisabledDeliveryAllowlistTargetsOnlyTheConfiguredEmail(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "previewadmin")
	other := newUser(t, "previewmember")
	var sentTokens []string
	ns := &NotificationService{
		disabled: true, disabledAllowEmail: strings.ToUpper(admin.Email), fcmEnabled: true, frontendURL: "*",
		fcmClient: stubPushClient(func(_ context.Context, msg *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
			sentTokens = append(sentTokens, msg.Tokens...)
			if msg.Webpush.FCMOptions != nil {
				t.Fatalf("invalid preview frontend URL was sent to FCM: %+v", msg.Webpush.FCMOptions)
			}
			return &messaging.BatchResponse{SuccessCount: len(msg.Tokens), Responses: []*messaging.SendResponse{{Success: true}}}, nil
		}),
	}
	if err := ns.RegisterPushToken(admin.ID, "admin-token", "Admin browser"); err != nil {
		t.Fatal(err)
	}
	if err := ns.RegisterPushToken(other.ID, "other-token", "Other browser"); err != nil {
		t.Fatal(err)
	}

	for _, user := range []models.User{admin, other} {
		if err := ns.SendNotification(context.Background(), user.ID, models.NotificationAdminAnnouncement, "Preview", "Selective", nil); err != nil {
			t.Fatal(err)
		}
	}
	if len(sentTokens) != 1 || sentTokens[0] != "admin-token" {
		t.Fatalf("disabled allowlist delivered to the wrong devices: %v", sentTokens)
	}
	var notices []models.Notification
	if err := database.DB.Find(&notices).Error; err != nil {
		t.Fatal(err)
	}
	if len(notices) != 1 || notices[0].UserID != admin.ID || !notices[0].PushSent {
		t.Fatalf("wrong selective notification history: %+v", notices)
	}
}

func TestConcurrentPushTokenRegistration(t *testing.T) {
	requireDB(t)
	user := newUser(t, "pushplayer")
	ns := NewNotificationService(NotificationConfig{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ns.RegisterPushToken(user.ID, "shared-token", "browser"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	var count int64
	if err := database.DB.Model(&models.UserPushToken{}).Where("token = ?", "shared-token").Count(&count).Error; err != nil || count != 1 {
		t.Fatalf("tokens=%d, error=%v", count, err)
	}
}

func TestSettlementBalancePushRoutesToMoney(t *testing.T) {
	for _, balanceType := range []models.NotificationType{models.NotificationBalanceLow, models.NotificationBalanceNegative} {
		t.Run(string(balanceType), func(t *testing.T) {
			f := newSettlementFixture(t, 24, 10000)
			player := f.member(t, "player")
			if balanceType == models.NotificationBalanceLow {
				if _, err := f.ledger.RecordTopup(CashInput{UserID: player.ID, AmountCents: 11000, CreatedBy: f.admin.ID}); err != nil {
					t.Fatal(err)
				}
			}
			calls := 0
			notifier := &NotificationService{fcmEnabled: true, frontendURL: "https://rally.test", fcmClient: stubPushClient(func(_ context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
				calls++
				if message.Data["type"] != string(balanceType) || message.Data["balance_cents"] == "" {
					t.Errorf("incomplete balance payload: %+v", message.Data)
				}
				if message.Webpush.FCMOptions.Link != "https://rally.test/money" {
					t.Errorf("wrong balance link: %s", message.Webpush.FCMOptions.Link)
				}
				return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
			})}
			if err := notifier.RegisterPushToken(player.ID, "balance-token", "test"); err != nil {
				t.Fatal(err)
			}
			f.settlement.WithNotifier(notifier)
			if _, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, Lines: []LineInput{{UserID: player.ID, InBase: true}}, SettledBy: f.admin.ID}); err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("push calls=%d, want 1", calls)
			}
		})
	}
}

func TestBalanceNudgeCreatesHistoryAndEnforcesCooldown(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "nudge-admin")
	admin.Role = models.RoleAdmin
	if err := database.DB.Save(&admin).Error; err != nil {
		t.Fatal(err)
	}
	player := newUser(t, "nudge-player")
	ns := NewNotificationService(NotificationConfig{FrontendURL: "https://rally.test"})

	result, err := ns.SendBalanceNudge(context.Background(), player.ID, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.PushSent || result.BalanceCents != 0 || result.NotificationID == uuid.Nil {
		t.Fatalf("unexpected result: %+v", result)
	}

	var notification models.Notification
	if err := database.DB.First(&notification, "id = ?", result.NotificationID).Error; err != nil {
		t.Fatal(err)
	}
	if notification.UserID != player.ID || notification.NotificationType != models.NotificationBalanceLow {
		t.Fatalf("wrong notification: %+v", notification)
	}
	var data map[string]string
	if err := json.Unmarshal([]byte(notification.Data), &data); err != nil {
		t.Fatal(err)
	}
	if data["source"] != "admin_balance_nudge" || data["nudged_by"] != admin.ID.String() {
		t.Fatalf("missing nudge metadata: %+v", data)
	}

	_, err = ns.SendBalanceNudge(context.Background(), player.ID, admin.ID)
	var cooldown *BalanceNudgeCooldownError
	if !errors.As(err, &cooldown) || cooldown.NextAllowedAt.IsZero() {
		t.Fatalf("expected cooldown error, got %v", err)
	}
	var count int64
	database.DB.Model(&models.Notification{}).Where("user_id = ?", player.ID).Count(&count)
	if count != 1 {
		t.Fatalf("cooldown allowed %d notifications", count)
	}
}

func TestBalanceNudgeRequiresALowApprovedMember(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "nudge-admin")
	player := newUser(t, "healthy-player")
	ns := NewNotificationService(NotificationConfig{})

	if _, err := NewLedgerService().RecordTopup(CashInput{
		UserID: player.ID, AmountCents: 5000, CreatedBy: admin.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := ns.SendBalanceNudge(context.Background(), player.ID, admin.ID); !errors.Is(err, ErrBalanceNudgeNotNeeded) {
		t.Fatalf("expected healthy balance refusal, got %v", err)
	}

	player.MembershipStatus = models.MembershipRemoved
	if err := database.DB.Save(&player).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := ns.SendBalanceNudge(context.Background(), player.ID, admin.ID); !errors.Is(err, ErrBalanceNudgeNotAllowed) {
		t.Fatalf("expected membership refusal, got %v", err)
	}
	if _, err := ns.SendBalanceNudge(context.Background(), admin.ID, admin.ID); !errors.Is(err, ErrBalanceNudgeSelf) {
		t.Fatalf("expected self-nudge refusal, got %v", err)
	}
}

func TestBalanceNudgeReportsAcceptedBackgroundPush(t *testing.T) {
	requireDB(t)
	admin := newUser(t, "nudge-admin")
	player := newUser(t, "push-nudge-player")
	var delivered *messaging.MulticastMessage
	ns := &NotificationService{
		fcmEnabled:  true,
		frontendURL: "https://rally.test",
		fcmClient: stubPushClient(func(_ context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
			delivered = message
			return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
		}),
	}
	if err := ns.RegisterPushToken(player.ID, "nudge-token", "phone"); err != nil {
		t.Fatal(err)
	}
	result, err := ns.SendBalanceNudge(context.Background(), player.ID, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !result.PushSent || delivered == nil {
		t.Fatalf("push was not accepted: %+v", result)
	}
	if delivered.Data["type"] != string(models.NotificationBalanceLow) || delivered.Webpush.FCMOptions.Link != "https://rally.test/money" {
		t.Fatalf("wrong push payload: %+v", delivered)
	}
}
