package handlers

import (
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"net/http"
	"testing"
)

func TestBalanceCorrectionHTTPAndAccess(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	req := map[string]any{"request_id": uuid.NewString(), "account_kind": "court_credit", "expected_cents": 0, "target_cents": 11300, "reason": "Verified venue credit"}
	h.as(player).post("/api/admin/balance-corrections", req).expect(http.StatusForbidden)
	h.as(player).get("/api/admin/balance-corrections").expect(http.StatusForbidden)
	var result models.BalanceCorrection
	h.as(admin).post("/api/admin/balance-corrections", req).expect(http.StatusOK).decode(&result)
	if result.BeforeCents != 0 || result.AfterCents != 11300 || result.CreatedBy != admin.ID {
		t.Fatalf("wrong audit: %+v", result)
	}
	var retried models.BalanceCorrection
	h.as(admin).post("/api/admin/balance-corrections", req).expect(http.StatusOK).decode(&retried)
	if result.TransactionID != retried.TransactionID {
		t.Fatal("retry duplicated correction")
	}
	req["request_id"] = uuid.NewString()
	h.as(admin).post("/api/admin/balance-corrections", req).expect(http.StatusConflict)
	req["expected_cents"] = 11300
	req["target_cents"] = 0
	h.as(admin).post("/api/admin/balance-corrections", req).expect(http.StatusOK)
	h.as(admin).get("/api/admin/balance-corrections?limit=1").expect(http.StatusOK)
	delete(req, "expected_cents")
	h.as(admin).post("/api/admin/balance-corrections", req).expect(http.StatusBadRequest)
}
