package handlers

import (
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"net/http"
	"testing"
)

func TestAssetPurchaseCorrectionHTTPAndAccess(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	var original models.Transaction
	h.as(admin).post("/api/admin/transactions/shuttle-purchase", map[string]any{"amount_cents": 5000, "units": 12}).expect(http.StatusCreated).decode(&original)
	path := "/api/admin/asset-purchases/" + original.ID.String() + "/correct"
	req := map[string]any{"amount_cents": 6000, "units": 24, "reason": "Receipt verified"}
	h.as(player).get("/api/admin/asset-purchases").expect(http.StatusForbidden)
	h.as(player).post(path, req).expect(http.StatusForbidden)
	pending := makeAdmin(t)
	database.DB.Model(&pending).Update("membership_status", models.MembershipPending)
	pending.MembershipStatus = models.MembershipPending
	h.as(pending).post(path, req).expect(http.StatusForbidden)
	h.as(pending).get("/api/admin/asset-purchases").expect(http.StatusForbidden)
	h.as(admin).get("/api/admin/asset-purchases?limit=1").expect(http.StatusOK)
	var corrected models.Transaction
	h.as(admin).post(path, req).expect(http.StatusOK).decode(&corrected)
	h.as(admin).post(path, req).expect(http.StatusOK)
	req["amount_cents"] = 7000
	h.as(admin).post(path, req).expect(http.StatusConflict)
	delete(req, "reason")
	h.as(admin).post(path, req).expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/asset-purchases/bad/correct", req).expect(http.StatusBadRequest)
}
