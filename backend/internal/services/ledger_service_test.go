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

// --- User Story 1 ---------------------------------------------------------

func TestRecordTopupMovesBankAndPlayerTogether(t *testing.T) {
	ls, user, playerAccount := newLedger(t)

	if _, err := ls.RecordTopup(CashInput{
		UserID:      user.ID,
		AmountCents: 5000,
		Description: "Bank transfer",
		CreatedBy:   user.ID,
	}); err != nil {
		t.Fatalf("topup failed: %v", err)
	}

	player, _ := ls.BalanceOf(playerAccount)
	bank, _ := ls.BalanceOfKind(nil, models.AccountKindBank)
	if player != 5000 {
		t.Errorf("player balance = %d, want 5000", player)
	}
	if bank != 5000 {
		t.Errorf("bank = %d, want 5000 — the club is holding their money", bank)
	}
	assertBalanced(t)
}

func TestRecordWithdrawalPaysAMemberBack(t *testing.T) {
	ls, user, playerAccount := newLedger(t)

	if _, err := ls.RecordTopup(CashInput{UserID: user.ID, AmountCents: 5000, CreatedBy: user.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := ls.RecordWithdrawal(CashInput{
		UserID:      user.ID,
		AmountCents: 2000,
		Description: "Settled up on leaving",
		CreatedBy:   user.ID,
	}); err != nil {
		t.Fatalf("withdrawal failed: %v", err)
	}

	player, _ := ls.BalanceOf(playerAccount)
	bank, _ := ls.BalanceOfKind(nil, models.AccountKindBank)
	if player != 3000 || bank != 3000 {
		t.Errorf("player = %d, bank = %d; want 3000 and 3000", player, bank)
	}
	assertBalanced(t)
}

func TestRecordTopupRejectsNonPositiveAmounts(t *testing.T) {
	ls, user, _ := newLedger(t)

	for _, amount := range []int64{0, -100} {
		if _, err := ls.RecordTopup(CashInput{UserID: user.ID, AmountCents: amount, CreatedBy: user.ID}); err == nil {
			t.Errorf("topup of %d was accepted; want an error", amount)
		}
	}
}

// A mistake is corrected by reversing it, and both halves stay on the record.
func TestReverseRestoresBalancesAndKeepsBothEntries(t *testing.T) {
	ls, user, playerAccount := newLedger(t)

	txn, err := ls.RecordTopup(CashInput{
		UserID:      user.ID,
		AmountCents: 5000,
		Description: "Recorded against the wrong player",
		CreatedBy:   user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := ls.ReverseTransaction(txn.ID, "Wrong player", user.ID); err != nil {
		t.Fatalf("reversal failed: %v", err)
	}

	balance, _ := ls.BalanceOf(playerAccount)
	if balance != 0 {
		t.Errorf("balance after reversal = %d, want 0", balance)
	}

	var transactions int64
	database.DB.Model(&models.Transaction{}).Count(&transactions)
	if transactions != 2 {
		t.Errorf("got %d transactions, want 2 — the error and its correction both stay visible", transactions)
	}

	assertBalanced(t)
}

func TestReverseRefusesASecondReversal(t *testing.T) {
	ls, user, _ := newLedger(t)

	txn, err := ls.RecordTopup(CashInput{UserID: user.ID, AmountCents: 5000, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ReverseTransaction(txn.ID, "", user.ID); err != nil {
		t.Fatal(err)
	}

	_, err = ls.ReverseTransaction(txn.ID, "", user.ID)
	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) || ledgerErr.Code != CodeTransactionAlreadyReverse {
		t.Fatalf("got %v, want %s", err, CodeTransactionAlreadyReverse)
	}
}

// Reversing must unwind the shuttle count as well as the value, or the bag and
// the books stop agreeing.
func TestReverseUnwindsShuttleUnits(t *testing.T) {
	ls, user, _ := newLedger(t)

	txn, err := ls.RecordShuttlePurchase(AssetPurchaseInput{
		AmountCents: 5000, Units: 12, Description: "1 tube", CreatedBy: user.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	stock, _ := ls.StockPosition(nil)
	if stock.Units != 12 || stock.ValueCents != 5000 {
		t.Fatalf("stock after purchase = %+v, want {5000 12}", stock)
	}

	if _, err := ls.ReverseTransaction(txn.ID, "Returned the tube", user.ID); err != nil {
		t.Fatal(err)
	}

	stock, _ = ls.StockPosition(nil)
	if stock.Units != 0 || stock.ValueCents != 0 {
		t.Errorf("stock after reversal = %+v, want empty", stock)
	}
	assertBalanced(t)
}

// Balances carried over from Splitwise, with surplus as the balancing figure.
func TestOpeningBalancesSeedTheClub(t *testing.T) {
	ls, user, playerAccount := newLedger(t)
	other := newUser(t, "other")
	if _, err := ls.EnsurePlayerAccount(other.ID, other.Name); err != nil {
		t.Fatal(err)
	}

	if _, err := ls.RecordOpeningBalances(OpeningBalancesInput{
		Players: []OpeningPlayerBalance{
			{UserID: user.ID, BalanceCents: 4250},
			{UserID: other.ID, BalanceCents: 3100},
		},
		BankCents:        1850,
		CourtCreditCents: 1700,
		ShuttleUnits:     9,
		ShuttleCents:     3750,
		CreatedBy:        user.ID,
	}); err != nil {
		t.Fatalf("opening balances failed: %v", err)
	}

	if balance, _ := ls.BalanceOf(playerAccount); balance != 4250 {
		t.Errorf("carried-over balance = %d, want 4250", balance)
	}

	stock, _ := ls.StockPosition(nil)
	if stock.Units != 9 || stock.ValueCents != 3750 {
		t.Errorf("opening stock = %+v, want {3750 9}", stock)
	}

	// Assets 7300, members hold 7350, so the club is 50 cents short — surplus
	// carries the difference rather than the books being wrong.
	surplus, _ := ls.BalanceOfKind(nil, models.AccountKindSurplus)
	if surplus != -50 {
		t.Errorf("surplus = %d, want -50 as the balancing figure", surplus)
	}

	assertBalanced(t)
}

func TestOpeningBalancesRefuseASecondRun(t *testing.T) {
	ls, user, _ := newLedger(t)

	in := OpeningBalancesInput{
		Players:   []OpeningPlayerBalance{{UserID: user.ID, BalanceCents: 4250}},
		BankCents: 4250,
		CreatedBy: user.ID,
	}
	if _, err := ls.RecordOpeningBalances(in); err != nil {
		t.Fatal(err)
	}

	_, err := ls.RecordOpeningBalances(in)
	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) || ledgerErr.Code != CodeOpeningBalancesRecorded {
		t.Fatalf("got %v, want %s", err, CodeOpeningBalancesRecorded)
	}
}

// A member must be able to follow their own arithmetic: every running balance
// in the list has to agree with the balance derived from the entries up to it.
func TestMyEntriesRunningBalanceReconciles(t *testing.T) {
	ls, user, playerAccount := newLedger(t)

	for _, amount := range []int64{5000, 2500, 1234, 99} {
		if _, err := ls.RecordTopup(CashInput{
			UserID: user.ID, AmountCents: amount, Description: "top-up", CreatedBy: user.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := ls.RecordWithdrawal(CashInput{
		UserID: user.ID, AmountCents: 1000, Description: "refund", CreatedBy: user.ID,
	}); err != nil {
		t.Fatal(err)
	}

	views, total, err := ls.MyEntries(user.ID, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}

	// Newest first, so the first row's running balance is the current balance.
	current, _ := ls.BalanceOf(playerAccount)
	if views[0].BalanceAfterCents != current {
		t.Errorf("newest running balance = %d, but the account holds %d",
			views[0].BalanceAfterCents, current)
	}

	// And walking backwards, each row differs from the next by its own amount.
	for i := 0; i < len(views)-1; i++ {
		if views[i].BalanceAfterCents-views[i].AmountCents != views[i+1].BalanceAfterCents {
			t.Errorf("row %d does not follow from row %d: %d − %d ≠ %d",
				i, i+1, views[i].BalanceAfterCents, views[i].AmountCents, views[i+1].BalanceAfterCents)
		}
	}
}

// Paging must not restart the arithmetic from zero on page two.
func TestMyEntriesRunningBalanceSurvivesPaging(t *testing.T) {
	ls, user, _ := newLedger(t)

	for i := 0; i < 6; i++ {
		if _, err := ls.RecordTopup(CashInput{
			UserID: user.ID, AmountCents: 1000, Description: "top-up", CreatedBy: user.ID,
		}); err != nil {
			t.Fatal(err)
		}
	}

	all, _, err := ls.MyEntries(user.ID, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	page2, _, err := ls.MyEntries(user.ID, 3, 3)
	if err != nil {
		t.Fatal(err)
	}

	for i, view := range page2 {
		if view.BalanceAfterCents != all[i+3].BalanceAfterCents {
			t.Errorf("paged row %d has balance %d, want %d", i, view.BalanceAfterCents, all[i+3].BalanceAfterCents)
		}
	}
}

// A reversed transaction stays in the list, flagged, rather than disappearing.
func TestMyEntriesFlagsReversedTransactions(t *testing.T) {
	ls, user, _ := newLedger(t)

	txn, err := ls.RecordTopup(CashInput{UserID: user.ID, AmountCents: 5000, CreatedBy: user.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ls.ReverseTransaction(txn.ID, "wrong player", user.ID); err != nil {
		t.Fatal(err)
	}

	views, total, err := ls.MyEntries(user.ID, 50, 0)
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Fatalf("total = %d, want 2", total)
	}

	var flagged int
	for _, v := range views {
		if v.Reversed {
			flagged++
		}
	}
	if flagged != 1 {
		t.Errorf("%d entries flagged as reversed, want exactly 1 (the original)", flagged)
	}
}
