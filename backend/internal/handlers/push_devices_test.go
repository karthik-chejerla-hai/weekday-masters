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
