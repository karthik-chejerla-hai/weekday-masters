package handlers

import (
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"net/http"
	"strings"
	"testing"
)

func TestPushDeviceAPIApprovalOwnershipAndProofCleanup(t *testing.T) {
	h := newHarness(t)
	owner := makePlayer(t)
	other := makePlayer(t)
	installation := uuid.New()
	secret := strings.Repeat("ab", 32)
	input := map[string]any{"installation_id": installation, "token": "private-token", "revocation_secret": secret, "generation": 0}
	var device services.PushEnrollment
	h.as(owner).post("/api/users/me/push-devices/enroll", input).expect(http.StatusOK).decode(&device)
	var raw []map[string]any
	h.as(owner).get("/api/users/me/push-devices").expect(http.StatusOK).decode(&raw)
	for _, row := range raw {
		if _, ok := row["token"]; ok {
			t.Fatal("token leaked")
		}
		if _, ok := row["revocation_secret"]; ok {
			t.Fatal("secret leaked")
		}
	}
	h.as(other).request(http.MethodDelete, "/api/users/me/push-devices/"+device.ID.String(), map[string]any{"generation": 1}).expect(http.StatusNotFound)
	h.as(other).post("/api/push-devices/revoke-proof", map[string]any{"installation_id": installation, "revocation_secret": strings.Repeat("00", 32), "generation": 1}).expect(http.StatusNotFound)
	other.MembershipStatus = models.MembershipPending
	database.DB.Model(other).Update("membership_status", other.MembershipStatus)
	h.as(other).get("/api/users/me/push-devices").expect(http.StatusForbidden)
	h.as(other).post("/api/push-devices/revoke-proof", map[string]any{"installation_id": installation, "revocation_secret": secret, "generation": 1}).expect(http.StatusNoContent)
	h.as(owner).request(http.MethodPut, "/api/users/me/push-devices/"+device.ID.String()+"/registration", map[string]any{"installation_id": installation, "token": "late", "generation": 1}).expect(http.StatusConflict)
	h.as(nil).post("/api/push-devices/revoke-proof", map[string]any{"installation_id": installation, "revocation_secret": secret, "generation": 2}).expect(http.StatusUnauthorized)
}

func TestLegacyRevokeAllowsAuthenticatedPendingAccountWithoutOwnerDisclosure(t *testing.T) {
	for _, status := range []models.MembershipStatus{models.MembershipApproved, models.MembershipPending} {
		t.Run(string(status), func(t *testing.T) {
			h := newHarness(t)
			owner := makePlayer(t)
			other := makePlayer(t)
			other.MembershipStatus = status
			database.DB.Model(other).Update("membership_status", status)
			row := models.UserPushToken{UserID: owner.ID, Token: "legacy-private"}
			if err := database.DB.Create(&row).Error; err != nil {
				t.Fatal(err)
			}
			var retained map[string]any
			h.as(owner).post("/api/push-devices/legacy-revoke", map[string]any{"token": row.Token}).expect(http.StatusOK).decode(&retained)
			if retained["cleanup_required"] != false || len(retained) != 1 {
				t.Fatal(retained)
			}
			var retired map[string]any
			h.as(other).post("/api/push-devices/legacy-revoke", map[string]any{"token": row.Token}).expect(http.StatusOK).decode(&retired)
			if retired["cleanup_required"] != true || len(retired) != 1 {
				t.Fatal(retired)
			}
			var stored models.UserPushToken
			database.DB.First(&stored, "id = ?", row.ID)
			if stored.UserID != owner.ID || stored.RevokedAt == nil {
				t.Fatal("legacy cleanup transferred row")
			}
			h.as(nil).post("/api/push-devices/legacy-revoke", map[string]any{"token": row.Token}).expect(http.StatusUnauthorized)
		})
	}
}
func TestRevokeProofWrongSecretCannotExposeCurrentGeneration(t *testing.T) {
	h := newHarness(t)
	owner := makePlayer(t)
	installation := uuid.New()
	secret := strings.Repeat("ab", 32)
	h.as(owner).post("/api/users/me/push-devices/enroll", map[string]any{"installation_id": installation, "token": "private", "revocation_secret": secret}).expect(http.StatusOK)
	var failed map[string]any
	h.as(owner).post("/api/push-devices/revoke-proof", map[string]any{"installation_id": installation, "revocation_secret": strings.Repeat("00", 32), "generation": 0}).expect(http.StatusNotFound).decode(&failed)
	if _, ok := failed["details"]; ok {
		t.Fatal("bad proof exposed metadata", failed)
	}
	var valid map[string]any
	h.as(owner).post("/api/push-devices/revoke-proof", map[string]any{"installation_id": installation, "revocation_secret": secret, "generation": 0}).expect(http.StatusConflict).decode(&valid)
	if valid["code"] != "device_generation_changed" {
		t.Fatal(valid)
	}
}
