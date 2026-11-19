package handlers

import (
	"testing"

	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"github.com/weekday-masters/backend/internal/utils"
)

func mountSharedExpenses(h *harness) *services.LedgerService {
	ledger := services.NewLedgerService()
	NewSharedExpenseHandler(services.NewSharedExpenseService(ledger, nil)).RegisterRoutes(h.router.Group("/api"))
	return ledger
}

func sharedExpenseBody(payer *models.User, members ...*models.User) map[string]any {
	participants := []map[string]any{}
	for _, m := range members {
		participants = append(participants, map[string]any{"user_id": m.ID})
	}
	body := map[string]any{
		"description": "Team dinner", "occurred_on": utils.NowInSydney().Format("2006-01-02"),
		"total_cents": 9000, "payer_user_id": nil, "split_method": "equal", "participants": participants,
	}
	if payer != nil {
		body["payer_user_id"] = payer.ID
	}
	return body
}

func TestSharedExpenseRoutePermissions(t *testing.T) {
	h := newHarness(t)
	mountSharedExpenses(h)
	player, pending := makePlayer(t), makePending(t)
	for _, path := range []string{"/api/admin/expenses/preview", "/api/admin/expenses"} {
		h.as(nil).post(path, sharedExpenseBody(nil, player)).expect(401)
		h.as(pending).post(path, sharedExpenseBody(nil, player)).expect(403)
		h.as(player).post(path, sharedExpenseBody(nil, player)).expect(403)
	}
}

func TestSharedExpenseHTTPPreviewConfirm(t *testing.T) {
	h := newHarness(t)
	ledger := mountSharedExpenses(h)
	admin, a, b, c := makeAdmin(t), makePlayer(t), makePlayer(t), makePlayer(t)
	body := sharedExpenseBody(a, a, b, c)

	// Confirming without a reviewed preview is refused.
	h.as(admin).post("/api/admin/expenses", body).expect(409)

	var preview services.SharedExpensePreview
	h.as(admin).post("/api/admin/expenses/preview", body).expect(200).decode(&preview)
	if len(preview.Fingerprint) != 64 || len(preview.Lines) != 3 {
		t.Fatalf("unexpected preview: %+v", preview)
	}

	body["expected_preview"] = preview.Fingerprint
	var txn models.Transaction
	h.as(admin).post("/api/admin/expenses", body).expect(201).decode(&txn)
	if txn.Kind != models.TxnSharedExpense {
		t.Fatalf("kind %q", txn.Kind)
	}
	for user, want := range map[*models.User]int64{a: 6000, b: -3000, c: -3000} {
		if got, err := ledger.BalanceOfUser(user.ID); err != nil || got != want {
			t.Fatalf("balance %d, want %d (%v)", got, want, err)
		}
	}
}

func TestSharedExpenseHTTPRejectsBadBodies(t *testing.T) {
	h := newHarness(t)
	mountSharedExpenses(h)
	admin, player := makeAdmin(t), makePlayer(t)

	unknown := sharedExpenseBody(nil, player)
	unknown["tip_cents"] = 500
	h.as(admin).post("/api/admin/expenses/preview", unknown).expect(400)

	invalid := sharedExpenseBody(nil, player)
	invalid["total_cents"] = 0
	h.as(admin).post("/api/admin/expenses/preview", invalid).expect(422)
}
