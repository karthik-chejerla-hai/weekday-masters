package handlers

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"gorm.io/gorm"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAdminDeliveryStatusAccessAndTerminalOutcome(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	pending := makePending(t)
	n := models.Notification{UserID: player.ID, NotificationType: models.NotificationBalanceLow, Title: "Private title", Body: "Private balance", Data: `{}`}
	var outbox *models.NotificationOutbox
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		outbox, err = services.QueueNotificationWithin(tx, &n, "", time.Now(), time.Now().Add(time.Hour))
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	target := "/api/admin/notifications/" + n.ID.String() + "/delivery"
	h.as(nil).get(target).expect(http.StatusUnauthorized)
	h.as(player).get(target).expect(http.StatusForbidden)
	h.as(pending).get(target).expect(http.StatusForbidden)
	h.as(admin).get("/api/admin/notifications/bad/delivery").expect(http.StatusBadRequest)
	h.as(admin).get("/api/admin/notifications/" + uuid.NewString() + "/delivery").expect(http.StatusNotFound)
	for _, state := range []models.DeliveryState{models.DeliveryPending, models.DeliveryFailed, models.DeliveryUncertain, models.DeliveryCompleted, models.DeliveryPartial, models.DeliverySuppressed} {
		if err := database.DB.Model(outbox).Update("state", state).Error; err != nil {
			t.Fatal(err)
		}
		var result map[string]any
		h.as(admin).get(target).expect(http.StatusOK).decode(&result)
		if result["state"] != string(state) {
			t.Fatalf("state not refreshed: %v", result)
		}
		raw, _ := json.Marshal(result)
		if strings.Contains(string(raw), "Private") || len(result) != 3 {
			t.Fatalf("private metadata leaked: %s", raw)
		}
	}
}
