package handlers

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
)

// The services own the arithmetic and have their own tests. What these assert is
// the HTTP surface: status codes, error shapes, and the structured codes the
// frontend keys off — the settlement form cannot tell "you are short on
// shuttles" from "the server fell over" unless this layer says so.

// errorBody is the structured failure shape from contracts/README.md.
type errorBody struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details"`
}

func (r *response) ledgerError() errorBody {
	r.t.Helper()
	var body errorBody
	r.decode(&body)
	return body
}

func topup(t *testing.T, h *harness, admin, user *models.User, cents int64) string {
	t.Helper()
	var txn struct {
		ID string `json:"id"`
	}
	h.as(admin).post("/api/admin/transactions/topup", map[string]any{
		"user_id":      user.ID.String(),
		"amount_cents": cents,
		"description":  "Bank transfer",
	}).expect(http.StatusCreated).decode(&txn)
	return txn.ID
}

func TestListBalancesReturnsEveryMember(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	topup(t, h, admin, player, 5000)

	var body struct {
		Items []struct {
			UserID       string `json:"user_id"`
			Name         string `json:"name"`
			BalanceCents int64  `json:"balance_cents"`
		} `json:"items"`
	}
	h.as(player).get("/api/accounts").expect(http.StatusOK).decode(&body)

	if len(body.Items) != 2 {
		t.Fatalf("got %d balances, want 2", len(body.Items))
	}

	byUser := map[string]int64{}
	for _, item := range body.Items {
		byUser[item.UserID] = item.BalanceCents
	}
	if byUser[player.ID.String()] != 5000 {
		t.Errorf("player balance = %d, want 5000", byUser[player.ID.String()])
	}
}

// The header chip asks for this on nearly every screen, so it returns the state
// as well as the number — the threshold rule lives on the server so there is one
// definition of "running low" rather than two.
func TestGetMyBalanceReportsState(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	var body struct {
		BalanceCents int64  `json:"balance_cents"`
		State        string `json:"state"`
	}

	player := makePlayer(t)
	h.as(player).get("/api/accounts/me").expect(http.StatusOK).decode(&body)
	if body.State != "low" {
		t.Errorf("a member at zero reads %q, want low", body.State)
	}

	topup(t, h, admin, player, 5000)
	h.as(player).get("/api/accounts/me").expect(http.StatusOK).decode(&body)
	if body.BalanceCents != 5000 || body.State != "ok" {
		t.Errorf("got %d / %q, want 5000 / ok", body.BalanceCents, body.State)
	}
}

func TestGetMyEntriesReturnsARunningBalance(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	topup(t, h, admin, player, 5000)
	topup(t, h, admin, player, 2500)

	var body struct {
		Items []struct {
			Kind              string `json:"kind"`
			AmountCents       int64  `json:"amount_cents"`
			BalanceAfterCents int64  `json:"balance_after_cents"`
		} `json:"items"`
		Total int64 `json:"total"`
	}
	h.as(player).get("/api/accounts/me/entries").expect(http.StatusOK).decode(&body)

	if body.Total != 2 {
		t.Fatalf("total = %d, want 2", body.Total)
	}
	if body.Items[0].BalanceAfterCents != 7500 {
		t.Errorf("newest running balance = %d, want 7500", body.Items[0].BalanceAfterCents)
	}
}

func TestRecordTopupRejectsBadInput(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	h.as(admin).post("/api/admin/transactions/topup", map[string]any{
		"user_id":      "not-a-uuid",
		"amount_cents": 5000,
	}).expect(http.StatusBadRequest)

	h.as(admin).post("/api/admin/transactions/topup", map[string]any{
		"user_id":      player.ID.String(),
		"amount_cents": 5000,
		"occurred_at":  "last Tuesday",
	}).expect(http.StatusBadRequest)
}

// A correction is a reversal, and a second reversal of the same transaction is
// refused with a code the frontend can act on.
func TestReverseTransactionIsAcceptedOnceOnly(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	id := topup(t, h, admin, player, 5000)
	path := fmt.Sprintf("/api/admin/transactions/%s/reverse", id)

	h.as(admin).post(path, map[string]any{"description": "wrong player"}).expect(http.StatusCreated)

	body := h.as(admin).post(path, nil).expect(http.StatusConflict).ledgerError()
	if body.Code != "transaction_already_reversed" {
		t.Errorf("code = %q, want transaction_already_reversed", body.Code)
	}
}

func TestReverseTransactionRejectsAnUnknownID(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)

	h.as(admin).post(fmt.Sprintf("/api/admin/transactions/%s/reverse", uuid.New()), nil).
		expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/transactions/not-a-uuid/reverse", nil).
		expect(http.StatusBadRequest)
}

func TestOpeningBalancesAcceptedOnce(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)

	payload := map[string]any{
		"players": []map[string]any{
			{"user_id": player.ID.String(), "balance_cents": 4250},
		},
		"bank_cents":         4250,
		"court_credit_cents": 0,
		"shuttle_stock":      map[string]any{"units": 0, "amount_cents": 0},
	}

	h.as(admin).post("/api/admin/transactions/opening-balances", payload).expect(http.StatusCreated)

	body := h.as(admin).post("/api/admin/transactions/opening-balances", payload).
		expect(http.StatusConflict).ledgerError()
	if body.Code != "opening_balances_already_recorded" {
		t.Errorf("code = %q, want opening_balances_already_recorded", body.Code)
	}
}

func TestAssetPurchasesMoveNoPlayerBalance(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	topup(t, h, admin, player, 20000)

	h.as(admin).post("/api/admin/transactions/court-credit", map[string]any{
		"amount_cents": 10000, "description": "venue top-up",
	}).expect(http.StatusCreated)

	h.as(admin).post("/api/admin/transactions/shuttle-purchase", map[string]any{
		"units": 12, "amount_cents": 5000, "description": "1 tube",
	}).expect(http.StatusCreated)

	var position struct {
		Assets struct {
			BankCents         int64 `json:"bank_cents"`
			CourtCreditCents  int64 `json:"court_credit_cents"`
			ShuttleStockCents int64 `json:"shuttle_stock_cents"`
			ShuttleStockUnits int   `json:"shuttle_stock_units"`
			TotalCents        int64 `json:"total_cents"`
		} `json:"assets"`
		Liabilities struct {
			PlayerBalancesCents int64 `json:"player_balances_cents"`
		} `json:"liabilities"`
		Balanced bool `json:"balanced"`
	}
	h.as(admin).get("/api/admin/position").expect(http.StatusOK).decode(&position)

	if position.Assets.TotalCents != 20000 {
		t.Errorf("total assets = %d, want 20000 — buying things does not change what the club holds",
			position.Assets.TotalCents)
	}
	if position.Liabilities.PlayerBalancesCents != 20000 {
		t.Errorf("player balances = %d, want 20000", position.Liabilities.PlayerBalancesCents)
	}
	if position.Assets.ShuttleStockUnits != 12 {
		t.Errorf("stock units = %d, want 12", position.Assets.ShuttleStockUnits)
	}
	if !position.Balanced {
		t.Error("position reports unbalanced")
	}
}

func TestIntegrityReportsBalanced(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	player := makePlayer(t)
	topup(t, h, admin, player, 5000)

	var report struct {
		Balanced      bool  `json:"balanced"`
		ResidualCents int64 `json:"residual_cents"`
		Entries       int64 `json:"entries_checked"`
	}
	h.as(admin).get("/api/admin/position/integrity").expect(http.StatusOK).decode(&report)

	if !report.Balanced || report.ResidualCents != 0 {
		t.Errorf("integrity: balanced=%v residual=%d", report.Balanced, report.ResidualCents)
	}
	if report.Entries == 0 {
		t.Error("integrity checked zero entries")
	}
}
