package handlers

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

func TestMemberPushStatus(t *testing.T) {
	h := newHarness(t)
	admin, player := makeAdmin(t), makePlayer(t)
	other := makePlayer(t)
	path := "/api/admin/users/" + player.ID.String() + "/push-notifications"
	h.as(player).get(path).expect(http.StatusForbidden)
	h.as(makePending(t)).get(path).expect(http.StatusForbidden)
	removed := makeUser(t, models.RoleAdmin, models.MembershipRemoved)
	h.as(removed).get(path).expect(http.StatusForbidden)
	h.as(nil).get(path).expect(http.StatusUnauthorized)
	h.as(admin).get("/api/admin/users/bad-id/push-notifications").expect(http.StatusBadRequest)
	h.as(admin).get("/api/admin/users/" + uuid.NewString() + "/push-notifications").expect(http.StatusNotFound)

	var empty services.MemberPushStatus
	h.as(admin).get(path).expect(http.StatusOK).decode(&empty)
	if empty.Preferences != nil || empty.Devices == nil || len(empty.Devices) != 0 {
		t.Fatalf("unexpected empty status: %+v", empty)
	}
	var count int64
	database.DB.Model(&models.UserNotificationPreferences{}).Where("user_id = ?", player.ID).Count(&count)
	if count != 0 {
		t.Fatal("read created preferences")
	}

	prefs := models.UserNotificationPreferences{UserID: player.ID}
	if err := database.DB.Create(&prefs).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&prefs).Updates(map[string]interface{}{"push_enabled": false, "push_balance_alerts": false}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	for _, token := range []models.UserPushToken{
		{UserID: player.ID, Token: "private-token-one", DeviceName: "Phone", LastUsedAt: now.Add(-time.Hour)},
		{UserID: player.ID, Token: "private-token-two", DeviceName: "", LastUsedAt: now},
		{UserID: other.ID, Token: "other-private-token", DeviceName: "Other member"},
	} {
		if err := database.DB.Create(&token).Error; err != nil {
			t.Fatal(err)
		}
	}
	response := h.as(admin).get(path).expect(http.StatusOK)
	var result services.MemberPushStatus
	response.decode(&result)
	if result.Preferences == nil || result.Preferences.PushEnabled || result.Preferences.PushBalanceAlerts || !result.Preferences.PushSessionReminders {
		t.Fatalf("incorrect saved settings: %+v", result.Preferences)
	}
	if len(result.Devices) != 2 || result.Devices[0].DeviceName != "" || !result.Devices[0].LastRegisteredAt.Equal(now) {
		t.Fatalf("incorrect devices: %+v", result.Devices)
	}
	// Inspect the full JSON response for sensitive data.
	raw := string(response.Body)
	if strings.Contains(raw, "token") || strings.Contains(raw, "Other member") {
		t.Fatalf("private token or another member's device exposed: %s", raw)
	}
}

func TestAdminMemberPushSummary(t *testing.T) {
	h := newHarness(t)
	admin, empty, registered := makeAdmin(t), makePlayer(t), makePlayer(t)
	// Default flags are true but do not establish registration.
	if err := database.DB.Create(&models.UserNotificationPreferences{UserID: empty.ID}).Error; err != nil {
		t.Fatal(err)
	}
	prefs := models.UserNotificationPreferences{UserID: registered.ID}
	if err := database.DB.Create(&prefs).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&prefs).Update("push_enabled", false).Error; err != nil {
		t.Fatal(err)
	}
	for _, token := range []string{"private-a", "private-b"} {
		if err := database.DB.Create(&models.UserPushToken{UserID: registered.ID, Token: token}).Error; err != nil {
			t.Fatal(err)
		}
	}
	response := h.as(admin).get("/api/admin/users").expect(http.StatusOK)
	var rows []services.AdminMember
	response.decode(&rows)
	counts := map[uuid.UUID]int64{}
	for _, row := range rows {
		counts[row.ID] = row.PushDeviceCount
	}
	if counts[empty.ID] != 0 || counts[registered.ID] != 2 {
		t.Fatalf("incorrect registration counts: %v", counts)
	}
	if strings.Contains(string(response.Body), "private-") || strings.Contains(string(response.Body), "preferences") {
		t.Fatal("summary exposed private registration data")
	}
}
