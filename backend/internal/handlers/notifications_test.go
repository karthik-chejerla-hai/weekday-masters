package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// --- preferences ----------------------------------------------------------

func TestGetPreferences_CreatesDefaultsOnFirstRead(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var prefs models.UserNotificationPreferences
	h.as(player).get("/api/users/me/notifications").expect(http.StatusOK).decode(&prefs)

	if prefs.UserID != player.ID {
		t.Fatalf("expected preferences for %s, got %s", player.ID, prefs.UserID)
	}
	// A new member should be opted in, otherwise reminders silently never arrive.
	if !prefs.PushEnabled || !prefs.EmailEnabled {
		t.Fatalf("expected push and email on by default, got push=%v email=%v",
			prefs.PushEnabled, prefs.EmailEnabled)
	}
}

func TestGetPreferences_RequiresAuthentication(t *testing.T) {
	h := newHarness(t)

	h.as(nil).get("/api/users/me/notifications").expect(http.StatusUnauthorized)
}

func TestUpdatePreferences_TogglesOnlyTheFieldsProvided(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var before models.UserNotificationPreferences
	h.as(player).get("/api/users/me/notifications").expect(http.StatusOK).decode(&before)

	var after models.UserNotificationPreferences
	h.as(player).put("/api/users/me/notifications",
		map[string]any{"push_enabled": false}).expect(http.StatusOK).decode(&after)

	if after.PushEnabled {
		t.Fatal("expected push to be switched off")
	}
	// Omitted fields are pointers on the request, so they must be left alone.
	if after.EmailEnabled != before.EmailEnabled {
		t.Fatal("expected an omitted preference to keep its previous value")
	}
}

func TestUpdatePreferences_RejectsAMalformedBody(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).put("/api/users/me/notifications",
		map[string]any{"push_enabled": "yes please"}).expect(http.StatusBadRequest)
}

// --- push tokens ----------------------------------------------------------

func TestRegisterPushToken_StoresTheToken(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).post("/api/users/me/push-tokens", map[string]string{
		"token":       "fcm-token-abc",
		"device_name": "Pixel 8",
	}).expect(http.StatusOK)

	var count int64
	database.DB.Model(&models.UserPushToken{}).
		Where("user_id = ? AND token = ?", player.ID, "fcm-token-abc").Count(&count)
	if count != 1 {
		t.Fatalf("expected the token to be stored once, found %d", count)
	}
}

func TestRegisterPushToken_RequiresAToken(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).post("/api/users/me/push-tokens",
		map[string]string{"device_name": "Pixel 8"}).expect(http.StatusBadRequest)
}

func TestUnregisterPushToken_RemovesTheNamedToken(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).post("/api/users/me/push-tokens",
		map[string]string{"token": "fcm-token-abc"}).expect(http.StatusOK)
	h.as(player).post("/api/users/me/push-tokens",
		map[string]string{"token": "fcm-token-def"}).expect(http.StatusOK)

	h.as(player).request(http.MethodDelete, "/api/users/me/push-tokens",
		map[string]string{"token": "fcm-token-abc"}).expect(http.StatusOK)

	var remaining []models.UserPushToken
	database.DB.Where("user_id = ?", player.ID).Find(&remaining)
	if len(remaining) != 1 || remaining[0].Token != "fcm-token-def" {
		t.Fatalf("expected only the other token to survive, got %+v", remaining)
	}
}

// --- history --------------------------------------------------------------

func TestGetNotificationHistory_IsEmptyForANewMember(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	var history []models.Notification
	h.as(player).get("/api/users/me/notifications/history").
		expect(http.StatusOK).decode(&history)

	if len(history) != 0 {
		t.Fatalf("expected no notifications, got %d", len(history))
	}
}

func TestGetNotificationHistory_HonoursLimitAndIgnoresNonsense(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	for i := 0; i < 3; i++ {
		notification := models.Notification{
			UserID:           player.ID,
			NotificationType: models.NotificationAdminAnnouncement,
			Title:            "Announcement",
			Body:             "Body",
			Data:             "{}",
		}
		if err := database.DB.Create(&notification).Error; err != nil {
			t.Fatalf("failed to seed notification: %v", err)
		}
	}

	var limited []models.Notification
	h.as(player).get("/api/users/me/notifications/history?limit=2").
		expect(http.StatusOK).decode(&limited)
	if len(limited) != 2 {
		t.Fatalf("expected the limit to be applied, got %d", len(limited))
	}

	// An unparseable limit falls back to the default rather than erroring.
	var all []models.Notification
	h.as(player).get("/api/users/me/notifications/history?limit=banana&offset=-4").
		expect(http.StatusOK).decode(&all)
	if len(all) != 3 {
		t.Fatalf("expected the default limit to return all 3, got %d", len(all))
	}
}

func TestMarkNotificationRead_FlagsTheCallersOwnNotification(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	notification := models.Notification{
		UserID:           player.ID,
		NotificationType: models.NotificationAdminAnnouncement,
		Title:            "Announcement",
		Body:             "Body",
		Data:             "{}",
	}
	if err := database.DB.Create(&notification).Error; err != nil {
		t.Fatalf("failed to seed notification: %v", err)
	}

	h.as(player).post("/api/notifications/"+notification.ID.String()+"/read", nil).
		expect(http.StatusOK)

	var stored models.Notification
	database.DB.First(&stored, "id = ?", notification.ID)
	if stored.ReadAt == nil {
		t.Fatal("expected the notification to be stamped as read")
	}
}

func TestMarkNotificationRead_RejectsAnInvalidID(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)

	h.as(player).post("/api/notifications/not-a-uuid/read", nil).expect(http.StatusBadRequest)
}

// --- announcements --------------------------------------------------------

func TestSendAnnouncement_RecordsItAndNotifiesApprovedMembers(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	makePending(t)

	var announcement models.Announcement
	h.as(admin).post("/api/admin/announcements", map[string]string{
		"title": "Courts closed Friday",
		"body":  "The venue is resurfacing the floor.",
	}).expect(http.StatusCreated).decode(&announcement)

	if announcement.CreatedBy != admin.ID {
		t.Fatalf("expected the announcement to be attributed to the admin, got %s", announcement.CreatedBy)
	}

	// SendBulkNotification fans out to one goroutine per member and returns
	// without waiting, so the rows appear shortly after the response.
	got := eventually(t, func() (map[uuid.UUID]bool, bool) {
		var notified []models.Notification
		database.DB.Where("notification_type = ?", models.NotificationAdminAnnouncement).Find(&notified)

		seen := map[uuid.UUID]bool{}
		for _, n := range notified {
			seen[n.UserID] = true
		}
		return seen, len(seen) >= 2
	})

	if !got[admin.ID] || !got[player.ID] {
		t.Fatalf("expected both approved members to be notified, got %v", got)
	}
	// The pending user must not be on the distribution list.
	if len(got) != 2 {
		t.Fatalf("expected exactly the 2 approved members to be notified, got %d", len(got))
	}
}

func TestSendAnnouncement_RequiresATitleAndBody(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).post("/api/admin/announcements",
		map[string]string{"title": "Only a title"}).expect(http.StatusBadRequest)
}
