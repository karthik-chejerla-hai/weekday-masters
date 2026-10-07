package services

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
)

func TestAustralianMobile(t *testing.T) {
	for _, input := range []string{"0412 345 678", "+61 412 345 678", "61412345678"} {
		if got, err := australianMobile(input); err != nil || got != "61412345678" {
			t.Fatalf("%q: %q %v", input, got, err)
		}
	}
	for _, input := range []string{"", "+1 555 123 4567", "0291234567", "04123456789", "+61+412345678"} {
		if _, err := australianMobile(input); err == nil {
			t.Fatalf("accepted %q", input)
		}
	}
}

func whatsAppFixture(t *testing.T) (*NotificationService, models.User, *atomic.Int32) {
	t.Helper()
	requireDB(t)
	count := &atomic.Int32{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count.Add(1)
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing auth")
		}
		var body struct {
			To       string `json:"to"`
			Type     string `json:"type"`
			Template struct {
				Name string `json:"name"`
			} `json:"template"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.To != "61412345678" || body.Type != "template" || body.Template.Name != "rally_balance_low" {
			t.Errorf("wrong payload: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.test"}]}`))
	}))
	t.Cleanup(server.Close)
	sender := NewWhatsAppSender(WhatsAppConfig{Enabled: true, AccessToken: "test-token", PhoneNumberID: "123", GraphVersion: "v23.0", LowTemplate: "rally_balance_low", NegativeTemplate: "rally_balance_negative", Language: "en", ReservedCents: 2})
	sender.endpoint = server.URL
	ns := &NotificationService{whatsapp: sender, frontendURL: "https://rally.test"}
	user := newUser(t, "whatsapp")
	database.DB.Model(&user).Update("phone_number", "0412345678")
	if _, err := ns.UpdateUserPreferences(user.ID, map[string]interface{}{"whatsapp_balance_alerts": true}); err != nil {
		t.Fatal(err)
	}
	return ns, user, count
}

func queueWhatsApp(t *testing.T, ns *NotificationService, user models.User) models.Notification {
	t.Helper()
	if err := ns.SendNotification(context.Background(), user.ID, models.NotificationBalanceLow, "Low balance", "Please top up", nil); err != nil {
		t.Fatal(err)
	}
	var n models.Notification
	if err := database.DB.Where("user_id = ?", user.ID).Order("created_at DESC").First(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n.WhatsAppStatus != "pending" {
		t.Fatalf("not queued: %+v", n)
	}
	return n
}

func TestWhatsAppPushReceiptSuppressesFallback(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	var token string
	ns.fcmEnabled = true
	ns.fcmClient = stubPushClient(func(_ context.Context, m *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		token = m.Data["receipt_token"]
		return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
	})
	if err := ns.RegisterPushToken(user.ID, "device", "test"); err != nil {
		t.Fatal(err)
	}
	n := queueWhatsApp(t, ns, user)
	if n.WhatsAppDueAt.Sub(n.CreatedAt) < 14*time.Minute || !n.PushSent || n.PushReceivedAt != nil {
		t.Fatal("acceptance confused with receipt or missing delay")
	}
	if err := ns.RecordPushReceipt(n.ID, strings.Repeat("0", 64)); !errors.Is(err, ErrPushReceipt) {
		t.Fatal("bad capability accepted")
	}
	if err := ns.RecordPushReceipt(uuid.New(), token); !errors.Is(err, ErrPushReceipt) {
		t.Fatal("capability accepted for another notification")
	}
	for i := 0; i < 2; i++ {
		if err := ns.RecordPushReceipt(n.ID, token); err != nil {
			t.Fatal(err)
		}
	}
	if err := ns.dispatchWhatsApp(context.Background(), n.ID, time.Now().Add(16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	database.DB.First(&n, "id = ?", n.ID)
	if count.Load() != 0 || n.PushReceivedAt == nil || n.WhatsAppStatus != "cancelled" {
		t.Fatalf("receipt failed: %+v", n)
	}
	bytes, _ := json.Marshal(n)
	if strings.Contains(string(bytes), token) || strings.Contains(string(bytes), n.PushReceiptHash) || strings.Contains(string(bytes), n.WhatsAppPhone) {
		t.Fatal("private delivery data leaked in history")
	}
}

func TestWhatsAppAcceptanceAloneDoesNotSuppressFallback(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	ns.fcmEnabled = true
	ns.fcmClient = stubPushClient(func(_ context.Context, _ *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
		return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
	})
	ns.RegisterPushToken(user.ID, "device", "test")
	n := queueWhatsApp(t, ns, user)
	if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 0 {
		t.Fatal("sent before 15 minutes")
	}
	if err := ns.dispatchWhatsApp(context.Background(), n.ID, time.Now().Add(16*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatal("accepted push suppressed fallback")
	}
}

func TestWhatsAppNoDeviceConcurrentWorkersSendOnce(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	n := queueWhatsApp(t, ns, user)
	if n.WhatsAppDueAt.After(time.Now()) {
		t.Fatal("no-device alert was delayed")
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("got %d requests", count.Load())
	}
	database.DB.First(&n, "id = ?", n.ID)
	if n.WhatsAppStatus != "accepted" || n.WhatsAppReservedCents != 2 || n.WhatsAppMessageID != "wamid.test" {
		t.Fatalf("wrong delivery state: %+v", n)
	}
}

func TestWhatsAppRechecksEligibility(t *testing.T) {
	for _, mode := range []string{"optout", "number", "removed", "recovered", "paused", "disabled", "expired", "read", "negative"} {
		t.Run(mode, func(t *testing.T) {
			ns, user, count := whatsAppFixture(t)
			n := queueWhatsApp(t, ns, user)
			switch mode {
			case "optout":
				ns.UpdateUserPreferences(user.ID, map[string]interface{}{"whatsapp_balance_alerts": false})
			case "number":
				database.DB.Model(&user).Update("phone_number", "0499999999")
			case "removed":
				database.DB.Model(&user).Update("membership_status", models.MembershipRemoved)
			case "paused":
				database.DB.Model(&models.Club{}).Where("1=1").Update("notifications_paused", true)
			case "disabled":
				ns.disabled = true
			case "expired":
				database.DB.Model(&n).Update("created_at", time.Now().Add(-25*time.Hour))
			case "read":
				ns.MarkNotificationRead(n.ID, user.ID)
			case "negative":
				database.DB.Model(&n).Update("notification_type", models.NotificationBalanceNegative)
			case "recovered":
				ledger := NewLedgerService()
				if _, err := ledger.EnsurePlayerAccount(user.ID, user.Name); err != nil {
					t.Fatal(err)
				}
				if _, err := ledger.RecordTopup(CashInput{UserID: user.ID, AmountCents: 10000, CreatedBy: user.ID}); err != nil {
					t.Fatal(err)
				}
			}
			if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
				t.Fatal(err)
			}
			if count.Load() != 0 {
				t.Fatalf("sent despite %s", mode)
			}
		})
	}
}

func TestWhatsAppBudgetSerializesConcurrentReservations(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	now := time.Now()
	used := models.Notification{UserID: user.ID, NotificationType: models.NotificationBalanceLow, Data: "{}", WhatsAppReservedAt: &now, WhatsAppReservedCents: 498, WhatsAppStatus: "uncertain"}
	if err := database.DB.Create(&used).Error; err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		queueWhatsApp(t, ns, user)
	}
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if count.Load() != 1 {
		t.Fatalf("budget allowed %d requests", count.Load())
	}
	var total int64
	database.DB.Model(&models.Notification{}).Select("SUM(whats_app_reserved_cents)").Scan(&total)
	if total != 500 {
		t.Fatalf("reserved %d cents", total)
	}
}

func TestWhatsAppMonthUsesSydneyBoundary(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	local := time.Now().In(utils.SydneyLocation)
	start := time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, utils.SydneyLocation)
	previous := start.Add(-time.Second)
	used := models.Notification{UserID: user.ID, NotificationType: models.NotificationBalanceLow, Data: "{}", WhatsAppReservedAt: &previous, WhatsAppReservedCents: 500, WhatsAppStatus: "uncertain"}
	database.DB.Create(&used)
	queueWhatsApp(t, ns, user)
	if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 1 {
		t.Fatal("previous Sydney month consumed current budget")
	}
}

func TestWhatsAppFailedRequestIsNotRetried(t *testing.T) {
	ns, user, _ := whatsAppFixture(t)
	var count atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { count.Add(1); w.WriteHeader(503) }))
	defer server.Close()
	ns.whatsapp.endpoint = server.URL
	n := queueWhatsApp(t, ns, user)
	for i := 0; i < 2; i++ {
		if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	database.DB.First(&n, "id = ?", n.ID)
	if count.Load() != 1 || n.WhatsAppStatus != "uncertain" || n.WhatsAppReservedCents != 2 {
		t.Fatalf("unsafe retry/state: %+v", n)
	}
}

func TestWhatsAppConsentRequiresMobileAndRevokesOnEdit(t *testing.T) {
	ns, user, _ := whatsAppFixture(t)
	for _, admin := range []bool{false, true} {
		phone := "0499999999"
		if admin {
			phone = "0412345678"
		}
		us := NewUserService("")
		var err error
		if admin {
			_, err = us.UpdateMemberDetails(user.ID, UpdateMemberInput{PhoneNumber: &phone})
		} else {
			_, err = us.UpdateProfile(user.ID, UpdateProfileInput{PhoneNumber: &phone})
		}
		if err != nil {
			t.Fatal(err)
		}
		prefs, err := ns.GetUserPreferences(user.ID)
		if err != nil {
			t.Fatal(err)
		}
		if prefs.WhatsAppBalanceAlerts || prefs.WhatsAppConsentPhone != "" {
			t.Fatal("number change retained consent")
		}
		if _, err = ns.UpdateUserPreferences(user.ID, map[string]interface{}{"whatsapp_balance_alerts": true}); err != nil {
			t.Fatal(err)
		}
	}
	database.DB.Model(&user).Update("phone_number", "")
	if _, err := ns.UpdateUserPreferences(user.ID, map[string]interface{}{"whatsapp_balance_alerts": true}); !errors.Is(err, ErrWhatsAppPhone) {
		t.Fatal("accepted absent number")
	}
}

func TestBalanceCrossingsOnly(t *testing.T) {
	id := uuid.New()
	notifier := &recordingNotifier{}
	s := NewSettlementService(NewLedgerService()).WithNotifier(notifier)
	for _, tc := range []struct {
		before, after int64
		want          models.NotificationType
	}{
		{2500, 1500, models.NotificationBalanceLow}, {1500, 1000, ""}, {1000, -100, models.NotificationBalanceNegative}, {-100, -500, ""}, {0, -100, models.NotificationBalanceNegative}, {2000, 1999, models.NotificationBalanceLow}, {3000, -100, models.NotificationBalanceNegative},
	} {
		notifier.sent = nil
		p := &SettlementPreview{Lines: []ChargeLineView{{UserID: id, AmountCents: tc.before - tc.after}}}
		s.notifyLowBalances(context.Background(), p, "Session", map[uuid.UUID]int64{id: tc.after}, 2000)
		if got := notifier.typeFor(id); got != tc.want {
			t.Errorf("%d -> %d got %q want %q", tc.before, tc.after, got, tc.want)
		}
	}
}

func TestWhatsAppRecoveryThenNewDropCancelsOldAlert(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	queueWhatsApp(t, ns, user)
	ledger := NewLedgerService()
	if _, err := ledger.EnsurePlayerAccount(user.ID, user.Name); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.RecordTopup(CashInput{UserID: user.ID, AmountCents: 10000, CreatedBy: user.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.RecordWithdrawal(CashInput{UserID: user.ID, AmountCents: 9500, CreatedBy: user.ID}); err != nil {
		t.Fatal(err)
	}
	if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if count.Load() != 0 {
		t.Fatal("old alert survived recovery")
	}
}

func TestWhatsAppMonthlyMessageLimit(t *testing.T) {
	ns, user, count := whatsAppFixture(t)
	now := time.Now()
	for i := 0; i < 200; i++ {
		n := models.Notification{UserID: user.ID, NotificationType: models.NotificationBalanceLow, Data: "{}", WhatsAppStatus: "accepted", WhatsAppReservedAt: &now, WhatsAppReservedCents: 2}
		if err := database.DB.Create(&n).Error; err != nil {
			t.Fatal(err)
		}
	}
	n := queueWhatsApp(t, ns, user)
	if err := ns.ProcessWhatsAppFallbacks(context.Background()); err != nil {
		t.Fatal(err)
	}
	database.DB.First(&n, "id = ?", n.ID)
	if count.Load() != 0 || n.WhatsAppStatus != "budget_blocked" {
		t.Fatal("200-message cap exceeded")
	}
}

func TestWhatsAppConfigurationFailsClosed(t *testing.T) {
	valid := WhatsAppConfig{Enabled: true, AccessToken: "token", PhoneNumberID: "123", GraphVersion: "v23.0", LowTemplate: "low", NegativeTemplate: "negative", Language: "en", ReservedCents: 2}
	for _, mutate := range []func(*WhatsAppConfig){
		func(c *WhatsAppConfig) { c.Enabled = false }, func(c *WhatsAppConfig) { c.AccessToken = "" }, func(c *WhatsAppConfig) { c.PhoneNumberID = "../x" },
		func(c *WhatsAppConfig) { c.GraphVersion = "" }, func(c *WhatsAppConfig) { c.LowTemplate = "" }, func(c *WhatsAppConfig) { c.NegativeTemplate = "" },
		func(c *WhatsAppConfig) { c.Language = "" }, func(c *WhatsAppConfig) { c.ReservedCents = 1 }, func(c *WhatsAppConfig) { c.ReservedCents = 501 },
	} {
		cfg := valid
		mutate(&cfg)
		if NewWhatsAppSender(cfg) != nil {
			t.Fatal("invalid config enabled sender")
		}
	}
}
