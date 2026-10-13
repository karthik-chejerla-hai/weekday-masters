package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
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
	// Compatibility reads retain defaults; these flags no longer govern delivery.
	if !prefs.PushEnabled || prefs.EmailEnabled {
		t.Fatalf("expected push on and email off by default, got push=%v email=%v",
			prefs.PushEnabled, prefs.EmailEnabled)
	}
}

func TestGetPreferences_RequiresAuthentication(t *testing.T) {
	h := newHarness(t)

	h.as(nil).get("/api/users/me/notifications").expect(http.StatusUnauthorized)
}

func TestObsoleteNotificationMutationsRequireUpdateAndPreserveTokens(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)
	token := models.UserPushToken{UserID: player.ID, Token: "retained-token"}
	if err := database.DB.Create(&token).Error; err != nil {
		t.Fatal(err)
	}
	h.as(player).put("/api/users/me/notifications", map[string]any{"push_enabled": false}).expect(http.StatusConflict)
	h.as(player).post("/api/users/me/push-tokens", map[string]any{"token": "replacement"}).expect(http.StatusConflict)
	h.as(player).request(http.MethodDelete, "/api/users/me/push-tokens", nil).expect(http.StatusConflict)
	var stored models.UserPushToken
	if err := database.DB.First(&stored, "id = ?", token.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Token != token.Token || stored.UserID != player.ID || stored.RevokedAt != nil {
		t.Fatal("legacy mutation changed enrollment")
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
		"request_id": uuid.NewString(),
		"title":      "Courts closed Friday",
		"body":       "The venue is resurfacing the floor.",
	}).expect(http.StatusCreated).decode(&announcement)

	if announcement.CreatedBy != admin.ID {
		t.Fatalf("expected the announcement to be attributed to the admin, got %s", announcement.CreatedBy)
	}

	var notified []models.Notification
	if err := database.DB.Where("notification_type = ?", models.NotificationAdminAnnouncement).Find(&notified).Error; err != nil {
		t.Fatal(err)
	}
	got := map[uuid.UUID]bool{}
	for _, notice := range notified {
		got[notice.UserID] = true
	}

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

func TestSendBalanceNudge_TargetsOneLowBalanceMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	var result services.BalanceNudgeResult
	h.as(admin).post("/api/admin/users/"+player.ID.String()+"/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusCreated).decode(&result)
	if result.NotificationID == uuid.Nil || result.PushSent || result.BalanceCents != 0 {
		t.Fatalf("unexpected result: %+v", result)
	}

	var notice models.Notification
	if err := database.DB.First(&notice, "id = ?", result.NotificationID).Error; err != nil {
		t.Fatal(err)
	}
	if notice.UserID != player.ID || notice.NotificationType != models.NotificationBalanceLow {
		t.Fatalf("wrong targeted notification: %+v", notice)
	}

	h.as(admin).post("/api/admin/users/"+player.ID.String()+"/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusTooManyRequests)
}

func TestSendBalanceNudge_RequiresApprovedAdminAndValidTarget(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	pending := makePending(t)

	h.as(player).post("/api/admin/users/"+admin.ID.String()+"/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusForbidden)
	h.as(admin).post("/api/admin/users/not-a-uuid/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/users/"+uuid.NewString()+"/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusNotFound)
	h.as(admin).post("/api/admin/users/"+pending.ID.String()+"/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusConflict)
	h.as(admin).post("/api/admin/users/"+admin.ID.String()+"/balance-nudge", map[string]string{"request_id": uuid.NewString()}).
		expect(http.StatusConflict)
}

func TestSendTestPushIsAdminOnlyAndFailsClosedWithoutFCM(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	h.as(nil).post("/api/admin/notifications/test-push", nil).expect(http.StatusUnauthorized)
	h.as(player).post("/api/admin/notifications/test-push", nil).expect(http.StatusForbidden)
	h.as(admin).post("/api/admin/notifications/test-push", nil).expect(http.StatusServiceUnavailable)
}

func TestNotificationDetail_OwnershipAndApproval(t *testing.T) {
	h := newHarness(t)
	player := makePlayer(t)
	other := makePlayer(t)
	n := models.Notification{UserID: player.ID, NotificationType: models.NotificationAdminAnnouncement, Title: "Exact announcement", Body: "Private body", Data: "{}"}
	if err := database.DB.Create(&n).Error; err != nil {
		t.Fatal(err)
	}
	var got services.NotificationNavigation
	h.as(player).get("/api/notifications/" + n.ID.String()).expect(http.StatusOK).decode(&got)
	if got.DestinationPath != "/notifications/"+n.ID.String() || got.Body != n.Body {
		t.Fatal(got)
	}
	h.as(other).get("/api/notifications/" + n.ID.String()).expect(http.StatusNotFound)
	h.as(other).post("/api/notifications/"+n.ID.String()+"/read", nil).expect(http.StatusNotFound)
	h.as(player).get("/api/notifications/bad-id").expect(http.StatusBadRequest)
	h.as(nil).get("/api/notifications/" + n.ID.String()).expect(http.StatusUnauthorized)
	if err := database.DB.Model(other).Update("membership_status", models.MembershipPending).Error; err != nil {
		t.Fatal(err)
	}
	other.MembershipStatus = models.MembershipPending
	h.as(other).get("/api/users/me/notifications/history").expect(http.StatusForbidden)
	h.as(other).get("/api/notifications/" + n.ID.String()).expect(http.StatusForbidden)
}

func TestBalanceNudgeRequestIdentityAndConflict(t *testing.T) {
	h := newHarness(t)
	admin, player, other := makeAdmin(t), makePlayer(t), makePlayer(t)
	path := "/api/admin/users/" + player.ID.String() + "/balance-nudge"
	h.as(admin).post(path, nil).expect(http.StatusBadRequest)
	h.as(admin).post(path, map[string]string{"request_id": "not-a-uuid"}).expect(http.StatusBadRequest)
	h.as(admin).post(path, map[string]string{"request_id": uuid.Nil.String()}).expect(http.StatusBadRequest)
	request := map[string]string{"request_id": uuid.NewString()}
	var first, retry services.BalanceNudgeResult
	h.as(admin).post(path, request).expect(http.StatusCreated).decode(&first)
	h.as(admin).post(path, request).expect(http.StatusCreated).decode(&retry)
	if first.NotificationID != retry.NotificationID || first.RequestID != retry.RequestID {
		t.Fatal("retry did not preserve event identity")
	}
	var conflict struct {
		Code string `json:"code"`
	}
	h.as(admin).post("/api/admin/users/"+other.ID.String()+"/balance-nudge", request).expect(http.StatusConflict).decode(&conflict)
	if conflict.Code != "request_id_conflict" {
		t.Fatalf("wrong conflicting reuse result: %s", conflict.Code)
	}
}

func TestAnnouncementRequestRetryConflictAndApprovedAdminGate(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	request := uuid.NewString()
	input := map[string]string{"request_id": request, "title": "News", "body": "Message"}
	h.as(nil).post("/api/admin/announcements", input).expect(http.StatusUnauthorized)
	h.as(player).post("/api/admin/announcements", input).expect(http.StatusForbidden)
	var first, second services.AnnouncementPublication
	h.as(admin).post("/api/admin/announcements", input).expect(http.StatusCreated).decode(&first)
	h.as(admin).post("/api/admin/announcements", input).expect(http.StatusCreated).decode(&second)
	if first.ID != second.ID || first.AudienceCount != 2 || first.QueuedCount != 0 || first.SuppressedCount != 2 {
		t.Fatal(first, second)
	}
	input["body"] = "Changed"
	h.as(admin).post("/api/admin/announcements", input).expect(http.StatusConflict)
	admin.MembershipStatus = models.MembershipPending
	database.DB.Model(admin).Update("membership_status", models.MembershipPending)
	h.as(admin).post("/api/admin/announcements", input).expect(http.StatusForbidden)
}
