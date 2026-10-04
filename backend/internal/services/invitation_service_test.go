package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sendgrid/sendgrid-go"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func TestInvitationEmailProviderContract(t *testing.T) {
	var code atomic.Int32
	code.Store(202)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Error("incorrect provider request")
		}
		var body struct {
			Personalizations []struct {
				To []struct {
					Email string `json:"email"`
				} `json:"to"`
			} `json:"personalizations"`
			Content  []struct{ Type, Value string } `json:"content"`
			Tracking struct {
				Click struct {
					Enable bool `json:"enable"`
				} `json:"click_tracking"`
			} `json:"tracking_settings"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body.Personalizations) != 1 || len(body.Personalizations[0].To) != 1 || body.Personalizations[0].To[0].Email != "admin@example.com" || len(body.Content) != 2 || body.Tracking.Click.Enable {
			t.Error("recipient, content, or link tracking changed")
		}
		w.WriteHeader(int(code.Load()))
		if code.Load() == 401 {
			_, _ = w.Write([]byte(`{"errors":[{"message":"Maximum credits exceeded"}]}`))
		}
	}))
	defer server.Close()
	s := NewInvitationService(NotificationConfig{SendGridAPIKey: "test-key", SendGridFromEmail: "club@example.com"}, true)
	s.newClient = func() *sendgrid.Client {
		client := sendgrid.NewSendClient("test-key")
		client.BaseURL = server.URL
		return client
	}
	email := InvitationEmail{Subject: "Welcome", HTML: "<p>Welcome</p>", Text: "Welcome"}
	status, _ := s.sendEmail(context.Background(), "admin@example.com", email)
	if status != "accepted" {
		t.Fatal(status)
	}
	code.Store(401)
	status, message := s.sendEmail(context.Background(), "admin@example.com", email)
	if status != "failed" || !strings.Contains(message, "sending limit") {
		t.Fatalf("%s %s", status, message)
	}
	code.Store(500)
	status, _ = s.sendEmail(context.Background(), "admin@example.com", email)
	if status != "unknown" {
		t.Fatal("unexpected server response must remain uncertain")
	}
}

func invitationFixture(t *testing.T) (*InvitationService, models.User, *models.User) {
	t.Helper()
	requireDB(t)
	admin := newUser(t, "admin")
	admin.Role = models.RoleAdmin
	if err := database.DB.Save(&admin).Error; err != nil {
		t.Fatal(err)
	}
	member := invite(t, NewUserService(""), "invited@example.com", "Invited Player")
	s := NewInvitationService(NotificationConfig{FrontendURL: "https://rally.example", SendGridAPIKey: "fake-for-tests", SendGridFromEmail: "club@example.com", SendGridFromName: "Rally"}, true)
	s.send = func(context.Context, string, InvitationEmail) (string, string) {
		return "accepted", "Accepted for delivery, not inbox confirmation."
	}
	return s, admin, member
}

func TestInvitationPreviewAndPausedTestCopy(t *testing.T) {
	s, admin, member := invitationFixture(t)
	if err := database.DB.Model(&models.Club{}).Where("true").Update("notifications_paused", true).Error; err != nil {
		t.Fatal(err)
	}
	s.cfg.Disabled = true
	member.Nickname = `<script>alert("x")</script>`
	if err := database.DB.Save(member).Error; err != nil {
		t.Fatal(err)
	}
	p, err := s.Preview(admin.ID, member.ID)
	if err != nil {
		t.Fatal(err)
	}
	if p.CanSend || !p.CanTest || p.TestRecipient != admin.Email || p.LastInvitation != nil || p.LoginURL != "https://rally.example/welcome" {
		t.Fatalf("preview %+v", p)
	}
	if strings.Contains(p.HTML, "<script>") || !strings.Contains(p.HTML, "&lt;script&gt;") || !strings.Contains(p.Text, member.Email) || !strings.Contains(p.HTML, p.LoginURL) {
		t.Fatal("unsafe or incomplete preview")
	}
	var attempts int64
	database.DB.Model(&models.InvitationDelivery{}).Count(&attempts)
	if attempts != 0 {
		t.Fatal("preview must not send or record an attempt")
	}
	if _, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), false, member.Email); err == nil {
		t.Fatal("member sends must remain paused")
	}
	s.send = func(ctx context.Context, to string, email InvitationEmail) (string, string) {
		if to != admin.Email || email.HTML != p.HTML || email.Text != p.Text || email.Subject != "[TEST] "+p.Subject {
			t.Fatal("test must only change destination and subject prefix")
		}
		return "accepted", "Accepted by provider."
	}
	delivery, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), true, member.Email)
	if err != nil || delivery.Status != "accepted" || delivery.AcceptedAt == nil || !delivery.IsTest {
		t.Fatalf("%+v %v", delivery, err)
	}
	p, err = s.Preview(admin.ID, member.ID)
	if err != nil || p.LastInvitation != nil || p.LastTest == nil {
		t.Fatal("test marked member invitation sent", err)
	}
	var notifications int64
	database.DB.Model(&models.Notification{}).Count(&notifications)
	if notifications != 0 {
		t.Fatal("test must not create routine notifications")
	}
	var club models.Club
	database.DB.First(&club)
	if !club.NotificationsPaused {
		t.Fatal("test cleared notification pause")
	}
}

func TestInvitationSendIdempotencyCooldownAndResults(t *testing.T) {
	s, admin, member := invitationFixture(t)
	var calls atomic.Int32
	s.send = func(context.Context, string, InvitationEmail) (string, string) {
		calls.Add(1)
		return "accepted", "Accepted by provider."
	}
	key := uuid.New()
	first, err := s.Send(context.Background(), admin.ID, member.ID, key, false, member.Email)
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.Send(context.Background(), admin.ID, member.ID, key, false, member.Email)
	if err != nil || first.ID != second.ID || calls.Load() != 1 {
		t.Fatalf("duplicate sent: %v, %d", err, calls.Load())
	}
	if _, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), false, member.Email); err == nil {
		t.Fatal("missing cooldown")
	}
	for _, outcome := range []string{"failed", "unknown"} {
		database.DB.Model(&models.InvitationDelivery{}).Where("true").Update("created_at", time.Now().Add(-2*time.Minute))
		s.send = func(context.Context, string, InvitationEmail) (string, string) {
			calls.Add(1)
			return outcome, "Check before resending."
		}
		result, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), false, member.Email)
		if err != nil || result.Status != outcome || result.AcceptedAt != nil {
			t.Fatalf("untruthful result %+v %v", result, err)
		}
		p, err := s.Preview(admin.ID, member.ID)
		if err != nil || p.LastInvitation.Status != outcome {
			t.Fatalf("missing durable result %+v %v", p, err)
		}
	}
}

func TestInvitationConcurrentSameRequestSendsOnce(t *testing.T) {
	s, admin, member := invitationFixture(t)
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s.send = func(context.Context, string, InvitationEmail) (string, string) {
		calls.Add(1)
		close(started)
		<-release
		return "accepted", "Accepted by provider."
	}
	key := uuid.New()
	done := make(chan error, 1)
	go func() {
		_, err := s.Send(context.Background(), admin.ID, member.ID, key, false, member.Email)
		done <- err
	}()
	<-started
	pending, err := s.Send(context.Background(), admin.ID, member.ID, key, false, member.Email)
	close(release)
	if err != nil || pending.Status != "pending" {
		t.Fatalf("%+v %v", pending, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Fatal("concurrent duplicate email")
	}
}

func TestInvitationGuards(t *testing.T) {
	s, admin, member := invitationFixture(t)
	if _, err := s.Preview(member.ID, member.ID); err == nil {
		t.Fatal("member may not preview private invitation")
	}
	for _, test := range []bool{false, true} {
		if _, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), test, "old@example.com"); err == nil {
			t.Fatal("stale email preview accepted")
		}
	}
	s.testEnabled = false
	if _, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), true, member.Email); err == nil {
		t.Fatal("disabled test sent")
	}
	s.cfg.SendGridAPIKey = ""
	if _, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), false, member.Email); err == nil {
		t.Fatal("unconfigured send accepted")
	}
	for _, address := range []string{"javascript:alert(1)", "https://user:pass@example.com", "https://example.com/?secret=one", "http://public.example"} {
		s.cfg.FrontendURL = address
		if _, err := s.Preview(admin.ID, member.ID); err == nil {
			t.Fatalf("unsafe link %s", address)
		}
	}
	s.cfg.FrontendURL = "http://localhost:5173"
	if _, err := s.Preview(admin.ID, member.ID); err != nil {
		t.Fatal(err)
	}
	member.MembershipStatus = models.MembershipRemoved
	database.DB.Save(member)
	if _, err := s.Preview(admin.ID, member.ID); err == nil {
		t.Fatal("removed member invite")
	}
	member.MembershipStatus, member.Auth0ID = models.MembershipApproved, "google-oauth2|claimed"
	database.DB.Save(member)
	if _, err := s.Preview(admin.ID, member.ID); err == nil {
		t.Fatal("already claimed invitation")
	}
}

func TestInvitedSignInKeepsBalanceHistoryAndRSVP(t *testing.T) {
	s, admin, member := invitationFixture(t)
	ledger := NewLedgerService()
	account, err := ledger.EnsurePlayerAccount(member.ID, member.DisplayName())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ledger.Post(PostInput{Kind: models.TxnPlayerTopup, Description: "Existing imported balance", Movements: []Movement{{AccountID: clubAccount(t, ledger, models.AccountKindBank), AmountCents: 12345}, {AccountID: account, AmountCents: 12345}}}); err != nil {
		t.Fatal(err)
	}
	session := newSession(t, NewSessionService(), admin.ID, 1)
	rsvp, err := NewRSVPService(nil).CreateOrUpdateRSVP(RSVPInput{SessionID: session.ID, UserID: member.ID, Status: models.RSVPStatusIn}, true)
	if err != nil {
		t.Fatal(err)
	}
	beforeEntries, beforeTxns := countEntries(t)
	if _, err := s.Send(context.Background(), admin.ID, member.ID, uuid.New(), false, member.Email); err != nil {
		t.Fatal(err)
	}
	users := NewUserService("")
	if _, _, err := users.RegisterUser(&Auth0Profile{Sub: "google-oauth2|unverified", Email: member.Email, EmailVerified: false}); err == nil {
		t.Fatal("unverified email claimed account")
	}
	other, _, err := users.RegisterUser(&Auth0Profile{Sub: "google-oauth2|different", Email: "different@example.com", EmailVerified: true})
	if err != nil || other.ID == member.ID || other.IsApproved() {
		t.Fatal("wrong Google email gained invited access", err)
	}
	claimed, isNew, err := users.RegisterUser(&Auth0Profile{Sub: "google-oauth2|confirmed", Email: strings.ToUpper(member.Email), EmailVerified: true})
	if err != nil || isNew || claimed.ID != member.ID || !claimed.IsApproved() {
		t.Fatalf("claim %+v %v", claimed, err)
	}
	var stored models.RSVP
	database.DB.First(&stored, "id = ?", rsvp.ID)
	if stored.UserID != claimed.ID || stored.Status != models.RSVPStatusIn {
		t.Fatal("RSVP changed on claim")
	}
	afterEntries, afterTxns := countEntries(t)
	if afterEntries != beforeEntries || afterTxns != beforeTxns {
		t.Fatal("sign-in changed ledger history")
	}
	var balance int64
	database.DB.Model(&models.LedgerEntry{}).Select("sum(amount_cents)").Where("account_id = ?", account).Scan(&balance)
	if balance != 12345 {
		t.Fatal("balance changed on sign-in")
	}
}
