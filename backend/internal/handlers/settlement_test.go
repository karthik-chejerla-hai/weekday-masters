package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

// pastSession creates a session that has already finished, stocked and funded so
// it can actually be settled.
func pastSession(t *testing.T, h *harness, admin *models.User) *models.Session {
	t.Helper()

	session := models.Session{
		Title:        "Tuesday Social",
		SessionDate:  utils.NowInSydney().AddDate(0, 0, -1),
		StartTime:    "20:00",
		EndTime:      "22:00",
		Courts:       1,
		RSVPDeadline: utils.NowInSydney().AddDate(0, 0, -4),
		Status:       models.SessionStatusOpen,
		CreatedBy:    admin.ID,
	}
	if err := database.DB.Create(&session).Error; err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Fund the club so settlement is not the thing that runs it dry.
	h.as(admin).post("/api/admin/transactions/topup", map[string]any{
		"user_id": admin.ID.String(), "amount_cents": 100000,
	}).expect(http.StatusCreated)
	h.as(admin).post("/api/admin/transactions/court-credit", map[string]any{
		"amount_cents": 50000,
	}).expect(http.StatusCreated)
	h.as(admin).post("/api/admin/transactions/shuttle-purchase", map[string]any{
		"units": 24, "amount_cents": 10000,
	}).expect(http.StatusCreated)

	return &session
}

type previewBody struct {
	Bands map[string]struct {
		Hours        float64 `json:"hours"`
		CourtCents   int64   `json:"court_cents"`
		ShuttleUnits int     `json:"shuttle_units"`
		ShuttleCents int64   `json:"shuttle_cents"`
		TotalCents   int64   `json:"total_cents"`
		Heads        int     `json:"heads"`
	} `json:"bands"`
	Totals struct {
		CourtCents   int64 `json:"court_cents"`
		ShuttleCents int64 `json:"shuttle_cents"`
		ChargedCents int64 `json:"charged_cents"`
		SurplusCents int64 `json:"surplus_cents"`
	} `json:"totals"`
	Lines []struct {
		UserID      string `json:"user_id"`
		Name        string `json:"name"`
		GuestName   string `json:"guest_name"`
		AmountCents int64  `json:"amount_cents"`
	} `json:"lines"`
	StockAfter struct {
		Units       int   `json:"units"`
		AmountCents int64 `json:"amount_cents"`
	} `json:"stock_after"`
}

// Preview must write nothing, so the form can call it on every keystroke.
func TestPreviewSettlementWritesNothing(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	body := map[string]any{
		"lines": []map[string]any{
			{"user_id": player.ID.String(), "in_base": true},
		},
	}

	var preview previewBody
	h.as(admin).post(fmt.Sprintf("/api/admin/sessions/%s/settlement/preview", session.ID), body).
		expect(http.StatusOK).decode(&preview)

	if preview.Totals.ChargedCents != 10167 {
		t.Errorf("charged = %d, want 10167", preview.Totals.ChargedCents)
	}

	var settlements int64
	database.DB.Model(&models.Settlement{}).Count(&settlements)
	if settlements != 0 {
		t.Errorf("preview wrote %d settlements", settlements)
	}
}

func TestSettleSessionChargesTheParticipants(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	body := map[string]any{
		"lines": []map[string]any{
			{"user_id": player.ID.String(), "in_base": true},
		},
	}
	path := fmt.Sprintf("/api/admin/sessions/%s/settle", session.ID)

	h.as(admin).post(path, body).expect(http.StatusCreated)

	var balance struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	h.as(player).get("/api/accounts/me").expect(http.StatusOK).decode(&balance)
	if balance.BalanceCents != -10167 {
		t.Errorf("player balance = %d, want -10167", balance.BalanceCents)
	}

	// Settling twice is refused with a code the frontend can act on.
	conflict := h.as(admin).post(path, body).expect(http.StatusConflict).ledgerError()
	if conflict.Code != "session_already_settled" {
		t.Errorf("code = %q, want session_already_settled", conflict.Code)
	}
}

// The shortfall must arrive as structured detail, not prose — the form needs the
// numbers to offer recording the missing purchase.
func TestSettleReportsAShuttleShortfallWithNumbers(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	session := models.Session{
		Title:        "Underprovisioned",
		SessionDate:  utils.NowInSydney().AddDate(0, 0, -1),
		StartTime:    "20:00",
		EndTime:      "22:00",
		Courts:       1,
		RSVPDeadline: utils.NowInSydney().AddDate(0, 0, -4),
		Status:       models.SessionStatusOpen,
		CreatedBy:    admin.ID,
	}
	if err := database.DB.Create(&session).Error; err != nil {
		t.Fatal(err)
	}

	body := map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}
	failure := h.as(admin).
		post(fmt.Sprintf("/api/admin/sessions/%s/settle", session.ID), body).
		expect(http.StatusUnprocessableEntity).ledgerError()

	if failure.Code != "shuttle_stock_short" {
		t.Fatalf("code = %q, want shuttle_stock_short", failure.Code)
	}
	if failure.Details["required_units"] != float64(10) {
		t.Errorf("required_units = %v, want 10", failure.Details["required_units"])
	}
	if failure.Details["available_units"] != float64(0) {
		t.Errorf("available_units = %v, want 0", failure.Details["available_units"])
	}
}

func TestSettleRefusesACancelledSession(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	database.DB.Model(&models.Session{}).Where("id = ?", session.ID).
		Update("status", models.SessionStatusCancelled)

	failure := h.as(admin).post(
		fmt.Sprintf("/api/admin/sessions/%s/settle", session.ID),
		map[string]any{"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}}},
	).expect(http.StatusUnprocessableEntity).ledgerError()

	if failure.Code != "not_settleable" {
		t.Errorf("code = %q, want not_settleable", failure.Code)
	}
}

// History lists finished sessions, settled or not — an uncosted one is exactly
// what the admin needs to see.
func TestSessionHistoryIncludesUnsettledSessions(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	settled := pastSession(t, h, admin)

	h.as(admin).post(fmt.Sprintf("/api/admin/sessions/%s/settle", settled.ID), map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}).expect(http.StatusCreated)

	unsettled := models.Session{
		Title:        "Not costed yet",
		SessionDate:  utils.NowInSydney().AddDate(0, 0, -8),
		StartTime:    "20:00",
		EndTime:      "22:00",
		Courts:       1,
		RSVPDeadline: utils.NowInSydney().AddDate(0, 0, -11),
		Status:       models.SessionStatusOpen,
		CreatedBy:    admin.ID,
	}
	if err := database.DB.Create(&unsettled).Error; err != nil {
		t.Fatal(err)
	}

	var body struct {
		Items []struct {
			SessionID   string `json:"session_id"`
			Title       string `json:"title"`
			Settled     bool   `json:"settled"`
			TotalCents  int64  `json:"total_cents"`
			PlayerCount int    `json:"player_count"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	h.as(player).get("/api/sessions/history").expect(http.StatusOK).decode(&body)

	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}

	byID := map[string]bool{}
	for _, item := range body.Items {
		byID[item.SessionID] = item.Settled
	}
	if !byID[settled.ID.String()] {
		t.Error("the settled session is not marked settled")
	}
	if byID[unsettled.ID.String()] {
		t.Error("the uncosted session is marked settled")
	}
}

// A member can check the split they were part of.
func TestSessionSettlementIsReadableByAnyMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	other := makePlayer(t)
	session := pastSession(t, h, admin)

	h.as(admin).post(fmt.Sprintf("/api/admin/sessions/%s/settle", session.ID), map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}).expect(http.StatusCreated)

	var view struct {
		Session struct {
			Title string `json:"title"`
		} `json:"session"`
		Rates struct {
			BaseRateCents int64 `json:"base_rate_cents"`
		} `json:"rates"`
		Totals struct {
			ChargedCents int64 `json:"charged_cents"`
		} `json:"totals"`
		Lines []struct {
			Name        string `json:"name"`
			AmountCents int64  `json:"amount_cents"`
		} `json:"lines"`
	}
	// Read as someone who was not even in the session.
	h.as(other).get(fmt.Sprintf("/api/sessions/%s/settlement", session.ID)).
		expect(http.StatusOK).decode(&view)

	if view.Totals.ChargedCents != 10167 {
		t.Errorf("charged = %d, want 10167", view.Totals.ChargedCents)
	}
	if view.Rates.BaseRateCents != 3000 {
		t.Errorf("rate = %d, want the snapshotted 3000", view.Rates.BaseRateCents)
	}
	if len(view.Lines) != 1 {
		t.Errorf("got %d lines, want 1", len(view.Lines))
	}
}

func TestSessionSettlementIsNotFoundBeforeSettling(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	h.as(player).get(fmt.Sprintf("/api/sessions/%s/settlement", session.ID)).
		expect(http.StatusNotFound)
	h.as(player).get("/api/sessions/not-a-uuid/settlement").
		expect(http.StatusBadRequest)
}

// Reversing frees the session to be settled again.
func TestReverseSettlementAllowsResettling(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	session := pastSession(t, h, admin)

	var created struct {
		Settlement struct {
			ID string `json:"id"`
		} `json:"settlement"`
	}
	h.as(admin).post(fmt.Sprintf("/api/admin/sessions/%s/settle", session.ID), map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}).expect(http.StatusCreated).decode(&created)

	h.as(admin).post(fmt.Sprintf("/api/admin/settlements/%s/reverse", created.Settlement.ID),
		map[string]any{"description": "wrong player list"}).expect(http.StatusCreated)

	var balance struct {
		BalanceCents int64 `json:"balance_cents"`
	}
	h.as(player).get("/api/accounts/me").expect(http.StatusOK).decode(&balance)
	if balance.BalanceCents != 0 {
		t.Errorf("balance after reversal = %d, want 0", balance.BalanceCents)
	}

	h.as(admin).post(fmt.Sprintf("/api/admin/sessions/%s/settle", session.ID), map[string]any{
		"lines": []map[string]any{{"user_id": player.ID.String(), "in_base": true}},
	}).expect(http.StatusCreated)
}

func TestSettlementRejectsMalformedIDs(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).post("/api/admin/sessions/not-a-uuid/settle", nil).expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/sessions/not-a-uuid/settlement/preview", nil).expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/settlements/not-a-uuid/reverse", nil).expect(http.StatusBadRequest)
}
