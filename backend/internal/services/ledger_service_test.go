package services

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// newLedger returns the service with an approved member and their account ready.
func newLedger(t *testing.T) (*LedgerService, models.User, uuid.UUID) {
	t.Helper()
	requireDB(t)

	ls := NewLedgerService()
	user := newUser(t, "ledger")
	accountID, err := ls.EnsurePlayerAccount(user.ID, user.Name)
	if err != nil {
		t.Fatalf("failed to create player account: %v", err)
	}
	return ls, user, accountID
}

func clubAccount(t *testing.T, ls *LedgerService, kind models.AccountKind) uuid.UUID {
	t.Helper()
	id, err := ls.ClubAccountID(nil, kind)
	if err != nil {
		t.Fatalf("club account %s: %v", kind, err)
	}
	return id
}

// countEntries reports how many ledger rows exist, for asserting that a refused
// post wrote nothing at all.
func countEntries(t *testing.T) (entries, transactions int64) {
	t.Helper()
	database.DB.Model(&models.LedgerEntry{}).Count(&entries)
	database.DB.Model(&models.Transaction{}).Count(&transactions)
	return entries, transactions
}

// The gate that everything else rests on: movements that do not balance are
// refused, and the refusal leaves no trace.
func TestPostRefusesUnbalancedMovements(t *testing.T) {
	ls, _, playerAccount := newLedger(t)
	bank := clubAccount(t, ls, models.AccountKindBank)

	entriesBefore, txnsBefore := countEntries(t)

	// Bank up $50 but the player up $60 — sixty dollars of credit conjured from
	// fifty dollars of cash.
	_, err := ls.Post(PostInput{
		Kind:        models.TxnPlayerTopup,
		Description: "unbalanced",
		Movements: []Movement{
			{AccountID: bank, AmountCents: 5000},
			{AccountID: playerAccount, AmountCents: 6000},
		},
	})

	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) {
		t.Fatalf("got %v, want a LedgerError", err)
	}
	if ledgerErr.Code != CodeInvariantViolated {
		t.Errorf("code = %s, want %s", ledgerErr.Code, CodeInvariantViolated)
	}

	entriesAfter, txnsAfter := countEntries(t)
	if entriesAfter != entriesBefore || txnsAfter != txnsBefore {
		t.Errorf("a refused post left rows behind: entries %d→%d, transactions %d→%d",
			entriesBefore, entriesAfter, txnsBefore, txnsAfter)
	}
}

// A balanced post writes, and the identity still holds afterwards.
func TestPostAcceptsBalancedMovements(t *testing.T) {
	ls, _, playerAccount := newLedger(t)
	bank := clubAccount(t, ls, models.AccountKindBank)

	txn, err := ls.Post(PostInput{
		Kind:        models.TxnPlayerTopup,
		Description: "Bank transfer",
		Movements: []Movement{
			{AccountID: bank, AmountCents: 5000},
			{AccountID: playerAccount, AmountCents: 5000},
		},
	})
	if err != nil {
		t.Fatalf("balanced post rejected: %v", err)
	}
	if txn.ID == uuid.Nil {
		t.Error("transaction has no id")
	}

	balance, err := ls.BalanceOf(playerAccount)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 5000 {
		t.Errorf("player balance = %d, want 5000", balance)
	}

	bankBalance, err := ls.BalanceOfKind(nil, models.AccountKindBank)
	if err != nil {
		t.Fatal(err)
	}
	if bankBalance != 5000 {
		t.Errorf("bank = %d, want 5000", bankBalance)
	}

	assertBalanced(t)
}

// assertBalanced checks the club-position identity independently of the code
// that maintains it, by recomputing straight from the entries.
func assertBalanced(t *testing.T) {
	t.Helper()

	var residual int64
	err := database.DB.Raw(`
		SELECT COALESCE(SUM(
			CASE WHEN a.kind IN ('bank','court_credit','shuttle_stock')
			     THEN e.amount_cents ELSE -e.amount_cents END), 0)
		FROM ledger_entries e JOIN accounts a ON a.id = e.account_id
	`).Scan(&residual).Error
	if err != nil {
		t.Fatalf("failed to evaluate the club position: %v", err)
	}
	if residual != 0 {
		t.Errorf("club position is out by %d cents; assets should equal player balances plus surplus", residual)
	}
}

// Principle VI is only true if there is genuinely no way to edit an entry. This
// asserts the absence of the method rather than trusting the review that added
// it — a future refactor that adds UpdateEntry should fail here.
func TestLedgerServiceExposesNoEntryMutation(t *testing.T) {
	forbidden := []string{"update", "delete", "remove", "edit", "destroy", "set"}

	serviceType := reflect.TypeOf(NewLedgerService())
	for i := 0; i < serviceType.NumMethod(); i++ {
		name := strings.ToLower(serviceType.Method(i).Name)
		for _, word := range forbidden {
			if strings.HasPrefix(name, word) {
				t.Errorf("LedgerService.%s looks like it mutates the ledger; corrections must be reversals (Principle VI)",
					serviceType.Method(i).Name)
			}
		}
	}
}

// An account that does not exist must stop the post rather than silently
// writing an entry that points at nothing.
func TestPostRejectsUnknownAccount(t *testing.T) {
	ls, _, playerAccount := newLedger(t)

	_, err := ls.Post(PostInput{
		Kind:        models.TxnPlayerTopup,
		Description: "ghost account",
		Movements: []Movement{
			{AccountID: playerAccount, AmountCents: 5000},
			{AccountID: uuid.New(), AmountCents: 5000},
		},
	})
	if err == nil {
		t.Fatal("expected a post against a non-existent account to fail")
	}

	entries, _ := countEntries(t)
	if entries != 0 {
		t.Errorf("%d entries written despite the failure", entries)
	}
}

// A transaction that moves nothing is a bug in the caller, not a no-op.
func TestPostRejectsEmptyMovements(t *testing.T) {
	ls, _, _ := newLedger(t)

	if _, err := ls.Post(PostInput{Kind: models.TxnPlayerTopup}); err == nil {
		t.Error("expected an empty transaction to be refused")
	}
}

// Every approved member appears in the balance list, including those who have
// never transacted.
func TestAllPlayerBalancesIncludesUntouchedMembers(t *testing.T) {
	ls, user, _ := newLedger(t)
	quiet := newUser(t, "quiet")
	if _, err := ls.EnsurePlayerAccount(quiet.ID, quiet.Name); err != nil {
		t.Fatal(err)
	}

	balances, err := ls.AllPlayerBalances()
	if err != nil {
		t.Fatal(err)
	}
	if len(balances) != 2 {
		t.Fatalf("got %d balances, want 2", len(balances))
	}

	byUser := map[uuid.UUID]int64{}
	for _, b := range balances {
		byUser[b.UserID] = b.BalanceCents
	}
	if byUser[user.ID] != 0 || byUser[quiet.ID] != 0 {
		t.Errorf("expected both members at zero, got %v", byUser)
	}
}

// Creating an account twice must not create two.
func TestEnsurePlayerAccountIsIdempotent(t *testing.T) {
	ls, user, first := newLedger(t)

	second, err := ls.EnsurePlayerAccount(user.ID, user.Name)
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Errorf("second call created a different account: %s then %s", first, second)
	}

	var count int64
	database.DB.Model(&models.Account{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Errorf("%d accounts for one member, want 1", count)
	}
}

// Approval is the moment a member becomes chargeable, so it is the moment their
// account must appear.
func TestApprovalCreatesLedgerAccount(t *testing.T) {
	requireDB(t)

	user := models.User{
		Auth0ID:          "auth0|pending-" + uuid.NewString(),
		Email:            uuid.NewString() + "@example.com",
		Name:             "Pending Member",
		Role:             models.RolePending,
		MembershipStatus: models.MembershipPending,
	}
	if err := database.DB.Create(&user).Error; err != nil {
		t.Fatal(err)
	}

	if _, err := NewUserService("admin@example.com").ApproveJoinRequest(user.ID); err != nil {
		t.Fatalf("approval failed: %v", err)
	}

	var count int64
	database.DB.Model(&models.Account{}).Where("user_id = ?", user.ID).Count(&count)
	if count != 1 {
		t.Errorf("approved member has %d ledger accounts, want 1", count)
	}
}

// The four club accounts are schema, not data — they must survive a reset.
func TestClubAccountsExist(t *testing.T) {
	ls, _, _ := newLedger(t)

	for _, kind := range models.ClubAccountKinds {
		if _, err := ls.ClubAccountID(nil, kind); err != nil {
			t.Errorf("club account %s missing: %v", kind, err)
		}
	}
}
