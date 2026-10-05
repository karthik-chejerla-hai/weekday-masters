package handlers

import (
	"fmt"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
	"net/http"
	"testing"
)

func mountExpenses(h *harness) {
	ledger := services.NewLedgerService()
	expenses := services.NewExpenseService(services.NewSettlementService(ledger), ledger)
	NewExpenseHandler(expenses).RegisterRoutes(h.router.Group("/api"))
	NewAssistantHandler(services.NewAssistantService(nil, nil, expenses, ledger)).RegisterRoutes(h.router.Group("/api"))
}
func TestExpenseRoutePermissions(t *testing.T) {
	h := newHarness(t)
	mountExpenses(h)
	admin := makeAdmin(t)
	player := makePlayer(t)
	pending := makePending(t)
	session := pastSession(t, h, admin)
	path := fmt.Sprintf("/api/admin/sessions/%s/expense/preview", session.ID)
	for _, user := range []*models.User{nil, pending, player} {
		status := 403
		if user == nil {
			status = 401
		}
		h.as(user).post(path, map[string]any{}).expect(status)
	}
	h.as(player).get("/api/sessions/unsettled").expect(200)
	h.as(pending).get("/api/sessions/unsettled").expect(403)
	h.as(nil).get("/api/assistant/status").expect(401)
	h.as(pending).post("/api/assistant/messages", map[string]any{}).expect(403)
	h.as(player).get("/api/assistant/status").expect(200)
	h.as(player).post("/api/assistant/messages", map[string]any{"messages": []map[string]string{{"role": "user", "content": "Hello"}}}).expect(503)
	h.as(player).post(fmt.Sprintf("/api/admin/sessions/%s/expense", session.ID), map[string]any{}).expect(403)
}
func TestExpenseHTTPPreviewConfirm(t *testing.T) {
	h := newHarness(t)
	mountExpenses(h)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)
	path := fmt.Sprintf("/api/admin/sessions/%s/expense", session.ID)
	body := map[string]any{"total_hours": 3, "shuttles_used": 8, "participant_ids": []string{player.ID.String()}}
	var preview services.ExpensePreview
	h.as(admin).post(path+"/preview", body).expect(200).decode(&preview)
	if preview.Fingerprint == "" || preview.Settlement.Totals.ShuttleUnits != 8 {
		t.Fatal("invalid preview")
	}
	body["expected_preview"] = preview.Fingerprint
	h.as(admin).post(path, body).expect(http.StatusCreated)
	h.as(admin).post(path, body).expect(409)
}
func TestExpenseRejectsBadJSON(t *testing.T) {
	h := newHarness(t)
	mountExpenses(h)
	admin := makeAdmin(t)
	session := pastSession(t, h, admin)
	path := fmt.Sprintf("/api/admin/sessions/%s/expense/preview", session.ID)
	for _, body := range []map[string]any{{"shuttles_used": 2.5, "total_hours": 2}, {"shuttles_used": 8, "total_hours": 2, "extra_field": true}, {"participant_ids": []string{"invalid"}}} {
		h.as(admin).post(path, body).expect(400)
	}
}

func TestExpenseHTTPHonoursExtraHourGroup(t *testing.T) {
	h := newHarness(t)
	mountExpenses(h)
	admin := makeAdmin(t)
	stayed, left := makePlayer(t), makePlayer(t)
	session := pastSession(t, h, admin)
	path := fmt.Sprintf("/api/admin/sessions/%s/expense", session.ID)
	body := map[string]any{"total_hours": 3, "shuttles_used": 8, "participant_ids": []string{stayed.ID.String(), left.ID.String()}, "extra_participant_ids": []string{stayed.ID.String()}}
	var preview services.ExpensePreview
	h.as(admin).post(path+"/preview", body).expect(200).decode(&preview)
	if preview.Settlement.Bands["extra"].Heads != 1 {
		t.Fatal("extra-hour group was ignored")
	}
	body["expected_preview"] = preview.Fingerprint
	h.as(admin).post(path, body).expect(201)
	var view services.SettlementView
	h.as(stayed).get(fmt.Sprintf("/api/sessions/%s/settlement", session.ID)).expect(200).decode(&view)
	for _, line := range view.Lines {
		if line.UserID == left.ID && line.InExtra {
			t.Fatal("history charged the early leaver for the extra hour")
		}
	}
}
