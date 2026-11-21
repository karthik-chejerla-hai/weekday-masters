package handlers

import (
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

// A lost HTTP reply must not let an admin record the same bank receipt twice.
// Each of the four create routes binds its request_id to the actor and the
// submitted contents, so these tests replay the same request after the first
// one has committed and assert on what the ledger holds afterwards.

type moneyCreateFlow struct {
	name string
	path string
	kind models.TransactionKind
	// body is the submission without its request_id.
	body func(player *models.User) map[string]any
	// changed is the same submission with different contents.
	changed func(player *models.User) map[string]any
	// position asserts the books after n identical submissions have posted.
	position func(t *testing.T, player *models.User, n int64)
}

func ledgerPosition(t *testing.T) *services.ClubPosition {
	t.Helper()
	position, err := services.NewLedgerService().Position()
	if err != nil {
		t.Fatal(err)
	}
	if !position.Balanced {
		t.Fatalf("club position does not balance: %+v", position)
	}
	return position
}

func playerBalance(t *testing.T, player *models.User) int64 {
	t.Helper()
	balance, err := services.NewLedgerService().BalanceOfUser(player.ID)
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

func moneyCreateFlows() []moneyCreateFlow {
	cash := func(cents int64) func(*models.User) map[string]any {
		return func(player *models.User) map[string]any {
			return map[string]any{"user_id": player.ID.String(), "amount_cents": cents, "description": "Bank transfer"}
		}
	}
	return []moneyCreateFlow{
		{
			name: "top-up", path: "/api/admin/transactions/topup", kind: models.TxnPlayerTopup,
			body: cash(5000), changed: cash(5001),
			position: func(t *testing.T, player *models.User, n int64) {
				if got := playerBalance(t, player); got != 5000*n {
					t.Errorf("player balance = %d, want %d", got, 5000*n)
				}
				if got := ledgerPosition(t).Assets.BankCents; got != 5000*n {
					t.Errorf("bank = %d, want %d", got, 5000*n)
				}
			},
		},
		{
			name: "withdrawal", path: "/api/admin/transactions/withdrawal", kind: models.TxnWithdrawal,
			body: cash(2000), changed: cash(2001),
			position: func(t *testing.T, player *models.User, n int64) {
				if got := playerBalance(t, player); got != -2000*n {
					t.Errorf("player balance = %d, want %d", got, -2000*n)
				}
				if got := ledgerPosition(t).Assets.BankCents; got != -2000*n {
					t.Errorf("bank = %d, want %d", got, -2000*n)
				}
			},
		},
		{
			name: "court-credit purchase", path: "/api/admin/transactions/court-credit", kind: models.TxnCourtCreditPurchase,
			body: func(*models.User) map[string]any {
				return map[string]any{"amount_cents": 10000, "description": "Venue account top-up"}
			},
			changed: func(*models.User) map[string]any {
				return map[string]any{"amount_cents": 10000, "description": "A different note"}
			},
			position: func(t *testing.T, player *models.User, n int64) {
				position := ledgerPosition(t)
				if position.Assets.CourtCreditCents != 10000*n || position.Assets.BankCents != -10000*n {
					t.Errorf("court credit / bank = %d / %d, want %d / %d",
						position.Assets.CourtCreditCents, position.Assets.BankCents, 10000*n, -10000*n)
				}
				if got := playerBalance(t, player); got != 0 {
					t.Errorf("player balance = %d, want 0", got)
				}
			},
		},
		{
			name: "shuttle purchase", path: "/api/admin/transactions/shuttle-purchase", kind: models.TxnShuttlePurchase,
			body: func(*models.User) map[string]any {
				return map[string]any{"amount_cents": 5000, "units": 12, "description": "Shuttle purchase"}
			},
			changed: func(*models.User) map[string]any {
				return map[string]any{"amount_cents": 5000, "units": 24, "description": "Shuttle purchase"}
			},
			position: func(t *testing.T, player *models.User, n int64) {
				position := ledgerPosition(t)
				if position.Assets.ShuttleStockCents != 5000*n || int64(position.Assets.ShuttleStockUnits) != 12*n ||
					position.Assets.BankCents != -5000*n {
					t.Errorf("stock value / units / bank = %d / %d / %d, want %d / %d / %d",
						position.Assets.ShuttleStockCents, position.Assets.ShuttleStockUnits, position.Assets.BankCents,
						5000*n, 12*n, -5000*n)
				}
			},
		},
	}
}

func countKind(t *testing.T, kind models.TransactionKind) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.Transaction{}).Where("kind = ?", kind).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func withRequestID(body map[string]any, id string) map[string]any {
	keyed := map[string]any{"request_id": id}
	for k, v := range body {
		keyed[k] = v
	}
	return keyed
}

type createdTransaction struct {
	ID         string `json:"id"`
	Kind       string `json:"kind"`
	OccurredAt string `json:"occurred_at"`
}

// The first reply is thrown away after the write commits, as a dropped
// connection would. The retry must return the original transaction.
func TestMoneyCreateRetryAfterLostReplyPostsOnce(t *testing.T) {
	for _, flow := range moneyCreateFlows() {
		t.Run(flow.name, func(t *testing.T) {
			h := newHarness(t)
			admin, player := makeAdmin(t), makePlayer(t)
			body := withRequestID(flow.body(player), uuid.NewString())

			var first, retry createdTransaction
			h.as(admin).post(flow.path, body).expect(http.StatusCreated).decode(&first)
			if got := countKind(t, flow.kind); got != 1 {
				t.Fatalf("first submission posted %d transactions", got)
			}

			h.as(admin).post(flow.path, body).expect(http.StatusCreated).decode(&retry)
			if retry.ID != first.ID || retry.OccurredAt != first.OccurredAt || retry.Kind != string(flow.kind) {
				t.Errorf("retry returned %+v, want the original %+v", retry, first)
			}
			if got := countKind(t, flow.kind); got != 1 {
				t.Errorf("%d transactions after a retry, want 1", got)
			}
			flow.position(t, player, 1)
		})
	}
}

func TestMoneyCreateRejectsReusedKeyWithDifferentContents(t *testing.T) {
	for _, flow := range moneyCreateFlows() {
		t.Run(flow.name, func(t *testing.T) {
			h := newHarness(t)
			admin, player := makeAdmin(t), makePlayer(t)
			id := uuid.NewString()
			h.as(admin).post(flow.path, withRequestID(flow.body(player), id)).expect(http.StatusCreated)

			failure := h.as(admin).post(flow.path, withRequestID(flow.changed(player), id)).
				expect(http.StatusConflict).ledgerError()
			if failure.Code != "request_conflict" {
				t.Errorf("code = %q, want request_conflict", failure.Code)
			}

			// The key is bound to the admin who used it, not only to the contents.
			other := makeAdmin(t)
			failure = h.as(other).post(flow.path, withRequestID(flow.body(player), id)).
				expect(http.StatusConflict).ledgerError()
			if failure.Code != "request_conflict" {
				t.Errorf("another admin reusing the key: code = %q, want request_conflict", failure.Code)
			}

			if got := countKind(t, flow.kind); got != 1 {
				t.Errorf("%d transactions after refused submissions, want 1", got)
			}
			flow.position(t, player, 1)
		})
	}
}

// Two receipts for the same amount on the same day are ordinary. Only the key
// says whether a second request is a retry or a second receipt.
func TestMoneyCreateSeparateSubmissionsEachPostOnce(t *testing.T) {
	for _, flow := range moneyCreateFlows() {
		t.Run(flow.name, func(t *testing.T) {
			h := newHarness(t)
			admin, player := makeAdmin(t), makePlayer(t)

			var first, second createdTransaction
			h.as(admin).post(flow.path, withRequestID(flow.body(player), uuid.NewString())).
				expect(http.StatusCreated).decode(&first)
			h.as(admin).post(flow.path, withRequestID(flow.body(player), uuid.NewString())).
				expect(http.StatusCreated).decode(&second)
			if first.ID == second.ID {
				t.Fatal("two separate submissions returned one transaction")
			}
			if got := countKind(t, flow.kind); got != 2 {
				t.Errorf("%d transactions, want 2", got)
			}
			flow.position(t, player, 2)
		})
	}
}

func TestMoneyCreateRequiresARequestID(t *testing.T) {
	for _, flow := range moneyCreateFlows() {
		t.Run(flow.name, func(t *testing.T) {
			h := newHarness(t)
			admin, player := makeAdmin(t), makePlayer(t)

			for name, body := range map[string]map[string]any{
				"missing":   flow.body(player),
				"empty":     withRequestID(flow.body(player), ""),
				"malformed": withRequestID(flow.body(player), "not-a-uuid"),
				"nil":       withRequestID(flow.body(player), uuid.Nil.String()),
			} {
				failure := h.as(admin).post(flow.path, body).expect(http.StatusBadRequest).ledgerError()
				if failure.Code != "invalid_input" {
					t.Errorf("%s request_id: code = %q, want invalid_input", name, failure.Code)
				}
			}
			if got := countKind(t, flow.kind); got != 0 {
				t.Errorf("%d transactions posted without a request ID", got)
			}
		})
	}
}

// A refused submission must leave its key free: the reservation rolls back with
// the posting, so the admin can correct the form and send it again.
func TestMoneyCreateFailureDoesNotConsumeTheKey(t *testing.T) {
	h := newHarness(t)
	admin := makeAdmin(t)
	id := uuid.NewString()

	h.as(admin).post("/api/admin/transactions/shuttle-purchase",
		map[string]any{"request_id": id, "amount_cents": 5000, "units": 0}).expect(http.StatusBadRequest)
	h.as(admin).post("/api/admin/transactions/shuttle-purchase",
		map[string]any{"request_id": id, "amount_cents": 5000, "units": 12}).expect(http.StatusCreated)
	if got := countKind(t, models.TxnShuttlePurchase); got != 1 {
		t.Errorf("%d transactions, want 1", got)
	}
}
