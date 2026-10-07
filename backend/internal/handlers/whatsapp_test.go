package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

func TestPushReceiptCapability(t *testing.T) {
	h := newHarness(t)
	user := makePlayer(t)
	token := strings.Repeat("a", 64)
	sum := sha256.Sum256([]byte(token))
	n := models.Notification{UserID: user.ID, NotificationType: models.NotificationBalanceLow, Data: "{}", PushReceiptHash: hex.EncodeToString(sum[:]), WhatsAppStatus: "pending"}
	if err := database.DB.Create(&n).Error; err != nil {
		t.Fatal(err)
	}
	handler := NewNotificationHandler(services.NewNotificationService(services.NotificationConfig{}))
	h.router.POST("/api/notifications/:id/push-receipt", handler.PushReceipt)
	// No Auth0 session is required, but the random push-only capability is.
	h.as(nil).post("/api/notifications/"+n.ID.String()+"/push-receipt", map[string]string{"token": strings.Repeat("b", 64)}).expect(http.StatusNotFound)
	for i := 0; i < 2; i++ {
		h.as(nil).post("/api/notifications/"+n.ID.String()+"/push-receipt", map[string]string{"token": token}).expect(http.StatusNoContent)
	}
	database.DB.First(&n, "id = ?", n.ID)
	if n.PushReceivedAt == nil || n.WhatsAppStatus != "cancelled" {
		t.Fatal("receipt did not cancel fallback")
	}
	database.DB.Model(&n).Update("created_at", time.Now().Add(-25*time.Hour))
	h.as(nil).post("/api/notifications/"+n.ID.String()+"/push-receipt", map[string]string{"token": token}).expect(http.StatusNotFound)
	h.as(nil).post("/api/notifications/"+n.ID.String()+"/push-receipt", map[string]string{"token": "short"}).expect(http.StatusBadRequest)
}

func TestWhatsAppConsentRejectsMissingPhone(t *testing.T) {
	h := newHarness(t)
	user := makePlayer(t)
	h.as(user).put("/api/users/me/notifications", map[string]bool{"whatsapp_balance_alerts": true}).expect(http.StatusBadRequest)
	database.DB.Model(user).Update("phone_number", "0412345678")
	var prefs models.UserNotificationPreferences
	h.as(user).put("/api/users/me/notifications", map[string]bool{"whatsapp_balance_alerts": true}).expect(http.StatusOK).decode(&prefs)
	if !prefs.WhatsAppBalanceAlerts {
		t.Fatal("consent not saved")
	}
}

func TestWhatsAppDispatchRequiresDedicatedSecret(t *testing.T) {
	for _, tc := range []struct {
		configured, supplied string
		want                 int
	}{
		{"", "", http.StatusUnauthorized}, {"short", "short", http.StatusUnauthorized},
		{strings.Repeat("a", 32), "wrong", http.StatusUnauthorized},
		{strings.Repeat("a", 32), strings.Repeat("a", 32), http.StatusNoContent},
	} {
		r := gin.New()
		handler := NewNotificationHandler(services.NewNotificationService(services.NotificationConfig{})).WithWorkerToken(tc.configured)
		r.POST("/dispatch", handler.DispatchWhatsApp)
		req := httptest.NewRequest(http.MethodPost, "/dispatch", nil)
		req.Header.Set("X-Notification-Worker-Token", tc.supplied)
		result := httptest.NewRecorder()
		r.ServeHTTP(result, req)
		if result.Code != tc.want {
			t.Fatalf("got %d want %d", result.Code, tc.want)
		}
	}
}
