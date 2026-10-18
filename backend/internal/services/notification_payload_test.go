package services

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"firebase.google.com/go/v4/messaging"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// Use the actual single-device FCM request shape. The SDK expands multicast
// calls into these requests, not a JSON array of device tokens.
func assertPushRequestBudget(t *testing.T, message *messaging.MulticastMessage) {
	t.Helper()
	request := map[string]any{"message": &messaging.Message{
		Token: message.Tokens[0], Notification: message.Notification,
		Data: message.Data, Webpush: message.Webpush,
	}}
	encoded, err := json.Marshal(request)
	if err != nil || len(encoded) > 4096 {
		t.Fatalf("FCM request bytes = %d, limit 4096: %v", len(encoded), err)
	}
	if !utf8.ValidString(message.Notification.Title) || !utf8.ValidString(message.Notification.Body) {
		t.Fatal("push preview split a UTF-8 sequence")
	}
}

func TestAnnouncementPushPreviewPreservesFullHistory(t *testing.T) {
	for _, tc := range []struct {
		name, title, body string
		short             bool
	}{
		{name: "ascii5000", title: "Club news", body: strings.Repeat("a", 5000)},
		{name: "multibyte5000", title: "Club news", body: strings.Repeat("🏸", 5000)},
		{name: "maximum_title", title: strings.Repeat("🏸", 200), body: strings.Repeat("🏸", 5000)},
		{name: "json_escaping", title: strings.Repeat("\"", 200), body: strings.Repeat("<>&\"\\", 1000)},
		{name: "short_unchanged", title: "Club news", body: "Courts open at 8 PM.", short: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			requireDB(t)
			actor := publicationAdmin(t)
			token := strings.Repeat("device-address", 12)
			addDeliveryDevice(t, actor, token)
			calls := 0
			ns := &NotificationService{fcmEnabled: true, frontendURL: "https://rally.test", fcmClient: stubPushClient(func(_ context.Context, message *messaging.MulticastMessage) (*messaging.BatchResponse, error) {
				calls++
				assertPushRequestBudget(t, message)
				if len(message.Tokens) != 1 || message.Tokens[0] != token || message.Notification.Title != tc.title {
					t.Fatal("preview changed the recipient or title")
				}
				if tc.short {
					if message.Notification.Body != tc.body {
						t.Fatal("ordinary short message changed")
					}
				} else if message.Notification.Body == tc.body || !strings.HasSuffix(message.Notification.Body, "…") || !strings.HasPrefix(tc.body, strings.TrimSuffix(message.Notification.Body, "…")) {
					t.Fatal("long body must be a bounded preview of the original")
				}
				id, err := uuid.Parse(message.Data["notification_id"])
				if err != nil || message.Data["type"] != string(models.NotificationAdminAnnouncement) || message.Data["announcement_id"] == "" {
					t.Fatal("push lost its navigation metadata")
				}
				if message.Webpush.FCMOptions.Link != "https://rally.test/notifications/"+id.String() || message.Webpush.Notification.Tag != id.String() {
					t.Fatal("preview no longer links to its complete history item")
				}
				return &messaging.BatchResponse{SuccessCount: 1, Responses: []*messaging.SendResponse{{Success: true}}}, nil
			})}
			publication, err := NewAnnouncementService(ns).Publish(actor.ID, uuid.New(), tc.title, tc.body)
			if err != nil || publication.QueuedCount != 1 {
				t.Fatalf("publish: %+v, %v", publication, err)
			}
			if summary, err := ns.ProcessNotificationWork(context.Background(), 10); err != nil || summary.Completed != 1 || calls != 1 {
				t.Fatalf("dispatch: %+v, calls=%d, %v", summary, calls, err)
			}
			var stored models.Announcement
			if err := database.DB.First(&stored, "id = ?", publication.ID).Error; err != nil {
				t.Fatal(err)
			}
			history, err := ns.GetUserNotifications(actor.ID, 10, 0)
			if err != nil || len(history) != 1 {
				t.Fatalf("history: %+v, %v", history, err)
			}
			if stored.Title != tc.title || stored.Body != tc.body || history[0].Title != tc.title || history[0].Body != tc.body || !history[0].PushSent {
				t.Fatal("provider preview changed full publication or member history")
			}
		})
	}
}

func TestFCMPreviewPreservesSettlementNavigation(t *testing.T) {
	client := &ttlPushClient{}
	service := &NotificationService{fcmClient: client, fcmEnabled: true, frontendURL: "https://rally.test"}
	sessionID, settlementID := uuid.NewString(), uuid.NewString()
	data := map[string]string{"type": string(models.NotificationSessionCharge), "session_id": sessionID, "settlement_id": settlementID, "balance_after_cents": "-1000"}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	notification := models.Notification{ID: uuid.New(), NotificationType: models.NotificationSessionCharge, Title: "Session charges", Body: strings.Repeat("🏸", 5000), Data: string(encoded)}
	result := (notificationFCMProvider{service}).Send(context.Background(), notification, "device", time.Hour)
	if result.State != models.DeliveryAccepted {
		t.Fatal(result)
	}
	assertPushRequestBudget(t, client.message)
	for key, expected := range data {
		if client.message.Data[key] != expected {
			t.Fatalf("preview lost %s", key)
		}
	}
	if client.message.Webpush.FCMOptions.Link != "https://rally.test/sessions/"+sessionID+"/settlement?settlement_id="+settlementID {
		t.Fatal("preview changed the historical settlement link")
	}
}

func TestFCMPreviewRejectsOversizeMetadataWithoutSubmission(t *testing.T) {
	client := &ttlPushClient{}
	service := &NotificationService{fcmClient: client, fcmEnabled: true}
	encoded, err := json.Marshal(map[string]string{"metadata": strings.Repeat("x", 5000)})
	if err != nil {
		t.Fatal(err)
	}
	notification := models.Notification{ID: uuid.New(), NotificationType: models.NotificationAdminAnnouncement, Title: "Hello", Body: "Hello", Data: string(encoded)}
	result := (notificationFCMProvider{service}).Send(context.Background(), notification, "device", time.Hour)
	if result.State != models.DeliveryFailed || result.Code != "push_payload_too_large" || client.message != nil {
		t.Fatal("oversize routing data was sent or silently discarded", result)
	}
}
