package services

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

type sharedExpenseFixture struct {
	service  *SharedExpenseService
	ledger   *LedgerService
	notifier *recordingNotifier
	admin    models.User
	members  []models.User
}

func newSharedExpenseFixture(t *testing.T) *sharedExpenseFixture {
	t.Helper()
	requireDB(t)
	ledger := NewLedgerService()
	notifier := &recordingNotifier{}
	admin := newUser(t, "admin")
	admin.Role = models.RoleAdmin
	if err := database.DB.Save(&admin).Error; err != nil {
		t.Fatal(err)
	}
	return &sharedExpenseFixture{
		service: NewSharedExpenseService(ledger, notifier), ledger: ledger, notifier: notifier,
		admin: admin, members: newUsers(t, 4),
	}
}

func today() string { return utils.NowInSydney().Format("2006-01-02") }

func equalExpense(payer *uuid.UUID, total int64, members ...models.User) SharedExpenseInput {
	in := SharedExpenseInput{Description: "Dinner", OccurredOn: today(), TotalCents: total, PayerUserID: payer, SplitMethod: SplitEqual}
	for _, m := range members {
		in.Participants = append(in.Participants, SharedExpenseParticipant{UserID: m.ID})
	}
	return in
}

func (f *sharedExpenseFixture) confirm(t *testing.T, in SharedExpenseInput) *models.Transaction {
	t.Helper()
	preview, err := f.service.Preview(in, &f.admin)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	in.ExpectedPreview = preview.Fingerprint
	txn, err := f.service.Confirm(in, &f.admin)
	if err != nil {
		t.Fatalf("confirm: %v", err)
	}
	return txn
}

func (f *sharedExpenseFixture) balance(t *testing.T, user models.User) int64 {
	t.Helper()
	balance, err := f.ledger.BalanceOfUser(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	return balance
}

func (f *sharedExpenseFixture) bank(t *testing.T) int64 {
	t.Helper()
	position, err := f.ledger.Position()
	if err != nil {
		t.Fatal(err)
	}
	if !position.Balanced {
		t.Fatal("club position is not balanced")
	}
	return position.Assets.BankCents
}

func transactionCount(t *testing.T) int64 {
	t.Helper()
	var count int64
	if err := database.DB.Model(&models.Transaction{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	return count
}

func lineFor(t *testing.T, p *SharedExpensePreview, user models.User) SharedExpenseLine {
	t.Helper()
	for _, line := range p.Lines {
		if line.UserID == user.ID {
			return line
		}
	}
	t.Fatalf("no line for %s", user.Name)
	return SharedExpenseLine{}
}

func TestSharedExpensePreviewEqualMemberPays(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	p, err := f.service.Preview(equalExpense(&m[0].ID, 8000, m...), &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Lines) != 4 || len(p.Fingerprint) != 64 || p.PayerName != m[0].DisplayName() {
		t.Fatalf("unexpected preview: %+v", p)
	}
	if payer := lineFor(t, p, m[0]); payer.ShareCents != 2000 || payer.NetCents != 6000 {
		t.Fatalf("payer line: %+v", payer)
	}
	for _, other := range m[1:] {
		if line := lineFor(t, p, other); line.ShareCents != 2000 || line.NetCents != -2000 {
			t.Fatalf("participant line: %+v", line)
		}
	}
}

func TestSharedExpensePreviewBankPays(t *testing.T) {
	f := newSharedExpenseFixture(t)
	p, err := f.service.Preview(equalExpense(nil, 1000, f.members[:3]...), &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if p.PayerName != "Club bank" || len(p.Lines) != 3 {
		t.Fatalf("unexpected preview: %+v", p)
	}
	var total int64
	for _, line := range p.Lines {
		if line.NetCents != -line.ShareCents || (line.ShareCents != 333 && line.ShareCents != 334) {
			t.Fatalf("line: %+v", line)
		}
		total += line.ShareCents
	}
	if total != 1000 {
		t.Fatalf("shares sum to %d", total)
	}
}

func TestSharedExpensePreviewPayerNotParticipating(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	p, err := f.service.Preview(equalExpense(&m[0].ID, 9000, m[1:]...), &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if payer := lineFor(t, p, m[0]); payer.ShareCents != 0 || payer.NetCents != 9000 {
		t.Fatalf("payer line: %+v", payer)
	}
}

func TestSharedExpensePreviewExactAndShares(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	five, three, two, one := int64(5000), int64(3000), 2, 1

	exact := equalExpense(nil, 8000)
	exact.SplitMethod = SplitExact
	exact.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, AmountCents: &five}, {UserID: m[1].ID, AmountCents: &three}}
	p, err := f.service.Preview(exact, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if lineFor(t, p, m[0]).ShareCents != 5000 || lineFor(t, p, m[1]).ShareCents != 3000 {
		t.Fatalf("exact lines: %+v", p.Lines)
	}

	weighted := equalExpense(nil, 9000)
	weighted.SplitMethod = SplitShares
	weighted.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, Weight: &two}, {UserID: m[1].ID, Weight: &one}}
	p, err = f.service.Preview(weighted, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if lineFor(t, p, m[0]).ShareCents != 6000 || lineFor(t, p, m[1]).ShareCents != 3000 {
		t.Fatalf("weighted lines: %+v", p.Lines)
	}
}

func TestSharedExpensePreviewIsStable(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	in := equalExpense(nil, 1000, m[:3]...)
	a, _ := f.service.Preview(in, &f.admin)
	// The order the form sends participants in must not matter.
	in.Participants[0], in.Participants[2] = in.Participants[2], in.Participants[0]
	in.Description = "  Dinner "
	b, err := f.service.Preview(in, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if a.Fingerprint != b.Fingerprint {
		t.Fatal("the same expense produced two fingerprints")
	}
	in.TotalCents++
	c, _ := f.service.Preview(in, &f.admin)
	if c.Fingerprint == a.Fingerprint {
		t.Fatal("a different total kept the fingerprint")
	}
}

func TestSharedExpenseRefusals(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	pending := newUser(t, "pending")
	removed := newUser(t, "removed")
	database.DB.Model(&pending).Update("membership_status", models.MembershipPending)
	database.DB.Model(&removed).Update("membership_status", models.MembershipRemoved)
	amount, zero, big := int64(4000), 0, 101
	short := int64(3999)
	one := 1

	many := equalExpense(nil, 10000)
	for i := 0; i < 101; i++ {
		many.Participants = append(many.Participants, SharedExpenseParticipant{UserID: uuid.New()})
	}

	cases := map[string]func(*SharedExpenseInput){
		"blank description": func(in *SharedExpenseInput) { in.Description = "   " },
		"long description":  func(in *SharedExpenseInput) { in.Description = strings.Repeat("a", 201) },
		"zero total":        func(in *SharedExpenseInput) { in.TotalCents = 0 },
		"negative total":    func(in *SharedExpenseInput) { in.TotalCents = -100 },
		"huge total":        func(in *SharedExpenseInput) { in.TotalCents = maxSharedExpenseCents + 1 },
		"future date": func(in *SharedExpenseInput) {
			in.OccurredOn = utils.NowInSydney().AddDate(0, 0, 1).Format("2006-01-02")
		},
		"bad date":            func(in *SharedExpenseInput) { in.OccurredOn = "yesterday" },
		"no participants":     func(in *SharedExpenseInput) { in.Participants = nil },
		"too many":            func(in *SharedExpenseInput) { *in = many },
		"duplicate":           func(in *SharedExpenseInput) { in.Participants = append(in.Participants, in.Participants[0]) },
		"pending participant": func(in *SharedExpenseInput) { in.Participants[0].UserID = pending.ID },
		"removed participant": func(in *SharedExpenseInput) { in.Participants[0].UserID = removed.ID },
		"removed payer":       func(in *SharedExpenseInput) { in.PayerUserID = &removed.ID },
		"unknown method":      func(in *SharedExpenseInput) { in.SplitMethod = "percent" },
		"equal with amount":   func(in *SharedExpenseInput) { in.Participants[0].AmountCents = &amount },
		"equal with weight":   func(in *SharedExpenseInput) { in.Participants[0].Weight = &one },
		"exact short": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitExact
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, AmountCents: &amount}, {UserID: m[1].ID, AmountCents: &short}}
		},
		"exact missing amount": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitExact
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, AmountCents: &amount}, {UserID: m[1].ID}}
		},
		"exact with weight": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitExact
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, AmountCents: &amount}, {UserID: m[1].ID, AmountCents: &amount, Weight: &one}}
		},
		"shares zero weight": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitShares
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, Weight: &one}, {UserID: m[1].ID, Weight: &zero}}
		},
		"shares huge weight": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitShares
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, Weight: &one}, {UserID: m[1].ID, Weight: &big}}
		},
		"shares missing weight": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitShares
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, Weight: &one}, {UserID: m[1].ID}}
		},
		"shares with amount": func(in *SharedExpenseInput) {
			in.SplitMethod = SplitShares
			in.Participants = []SharedExpenseParticipant{{UserID: m[0].ID, Weight: &one, AmountCents: &amount}, {UserID: m[1].ID, Weight: &one}}
		},
		"payer is the only participant": func(in *SharedExpenseInput) {
			in.PayerUserID = &m[0].ID
			in.Participants = in.Participants[:1]
		},
		"too small to split": func(in *SharedExpenseInput) { in.TotalCents = 1 },
	}
	before := transactionCount(t)
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := equalExpense(nil, 8000, m[0], m[1])
			mutate(&in)
			if _, err := f.service.Preview(in, &f.admin); err == nil {
				t.Fatal("preview accepted")
			} else if _, ok := AsLedgerError(err); !ok {
				t.Fatalf("not a ledger error: %v", err)
			}
			in.ExpectedPreview = strings.Repeat("0", 64)
			if _, err := f.service.Confirm(in, &f.admin); err == nil {
				t.Fatal("confirm accepted")
			}
		})
	}
	if transactionCount(t) != before {
		t.Fatal("a refused expense posted a transaction")
	}
}

func TestSharedExpenseIsAdminOnly(t *testing.T) {
	f := newSharedExpenseFixture(t)
	in := equalExpense(nil, 8000, f.members...)
	for _, actor := range []*models.User{nil, &f.members[0]} {
		_, err := f.service.Preview(in, actor)
		ledgerErr, ok := AsLedgerError(err)
		if !ok || (ledgerErr.Status != 401 && ledgerErr.Status != 403) {
			t.Fatalf("preview: %v", err)
		}
		in.ExpectedPreview = strings.Repeat("0", 64)
		if _, err = f.service.Confirm(in, actor); err == nil {
			t.Fatal("confirm accepted")
		}
	}
}

func TestSharedExpenseRefusedWhileAssetsPending(t *testing.T) {
	f := newSharedExpenseFixture(t)
	pending := models.SplitwiseImport{ID: uuid.New(), SourceHash: strings.Repeat("a", 64), MappingHash: strings.Repeat("b", 64), Cutoff: utils.NowInSydney()}
	if err := database.DB.Create(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.service.Preview(equalExpense(nil, 8000, f.members...), &f.admin); err == nil {
		t.Fatal("accepted while club assets await confirmation")
	}
}

func TestSharedExpenseConfirmMemberPays(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	bankBefore := f.bank(t)
	txn := f.confirm(t, equalExpense(&m[0].ID, 8000, m...))

	if got := f.balance(t, m[0]); got != 6000 {
		t.Fatalf("payer balance %d", got)
	}
	for _, other := range m[1:] {
		if got := f.balance(t, other); got != -2000 {
			t.Fatalf("participant balance %d", got)
		}
	}
	var stored models.Transaction
	if err := database.DB.Preload("Entries").First(&stored, "id = ?", txn.ID).Error; err != nil {
		t.Fatal(err)
	}
	// One net entry for the payer, not a credit and a separate charge.
	if stored.Kind != models.TxnSharedExpense || stored.Description != "Dinner" || len(stored.Entries) != 4 {
		t.Fatalf("stored transaction: kind=%s entries=%d", stored.Kind, len(stored.Entries))
	}
	if f.bank(t) != bankBefore {
		t.Fatal("a member-paid expense moved the bank")
	}
}

func TestSharedExpenseConfirmBankPays(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	if _, err := f.ledger.RecordTopup(CashInput{UserID: m[0].ID, AmountCents: 10000, CreatedBy: f.admin.ID}); err != nil {
		t.Fatal(err)
	}
	bankBefore := f.bank(t)
	f.confirm(t, equalExpense(nil, 8000, m...))

	if got := f.bank(t); got != bankBefore-8000 {
		t.Fatalf("bank moved by %d", got-bankBefore)
	}
	if got := f.balance(t, m[0]); got != 8000 {
		t.Fatalf("topped-up member balance %d", got)
	}
	// A share may take a member below zero, as a session charge may.
	if got := f.balance(t, m[1]); got != -2000 {
		t.Fatalf("participant balance %d", got)
	}
}

func TestSharedExpenseConfirmNeedsMatchingPreview(t *testing.T) {
	f := newSharedExpenseFixture(t)
	in := equalExpense(nil, 8000, f.members...)
	before := transactionCount(t)

	_, err := f.service.Confirm(in, &f.admin)
	if ledgerErr, ok := AsLedgerError(err); !ok || ledgerErr.Code != "preview_required" {
		t.Fatalf("missing fingerprint: %v", err)
	}
	preview, _ := f.service.Preview(in, &f.admin)
	in.ExpectedPreview = preview.Fingerprint
	in.TotalCents = 8400
	_, err = f.service.Confirm(in, &f.admin)
	if ledgerErr, ok := AsLedgerError(err); !ok || ledgerErr.Code != "preview_stale" || ledgerErr.Status != 409 {
		t.Fatalf("stale fingerprint: %v", err)
	}
	if transactionCount(t) != before {
		t.Fatal("an unreviewed expense posted a transaction")
	}
}

func TestSharedExpenseRefusesMemberRemovedAfterPreview(t *testing.T) {
	f := newSharedExpenseFixture(t)
	in := equalExpense(nil, 8000, f.members...)
	preview, _ := f.service.Preview(in, &f.admin)
	in.ExpectedPreview = preview.Fingerprint
	database.DB.Model(&f.members[1]).Update("membership_status", models.MembershipRemoved)
	before := transactionCount(t)
	if _, err := f.service.Confirm(in, &f.admin); err == nil {
		t.Fatal("charged a removed member")
	}
	if transactionCount(t) != before {
		t.Fatal("a refused expense posted a transaction")
	}
}

func TestSharedExpenseReversal(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	txn := f.confirm(t, equalExpense(&m[0].ID, 8000, m...))
	if _, err := f.ledger.ReverseTransaction(txn.ID, "", f.admin.ID); err != nil {
		t.Fatal(err)
	}
	for _, member := range m {
		if got := f.balance(t, member); got != 0 {
			t.Fatalf("balance after reversal %d", got)
		}
	}
	f.bank(t) // asserts the position still balances
	_, err := f.ledger.ReverseTransaction(txn.ID, "", f.admin.ID)
	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) || ledgerErr.Code != CodeTransactionAlreadyReverse {
		t.Fatalf("second reversal: %v", err)
	}
}

func TestSharedExpenseNotifications(t *testing.T) {
	f := newSharedExpenseFixture(t)
	m := f.members
	f.confirm(t, equalExpense(&m[0].ID, 8000, m...))

	var sent []models.Notification
	if err := database.DB.Where("notification_type = ?", models.NotificationSharedExpense).Find(&sent).Error; err != nil {
		t.Fatal(err)
	}
	if len(sent) != 4 || len(f.notifier.notifiedUsers()) != 4 {
		t.Fatalf("queued %d notifications", len(sent))
	}
	for _, n := range sent {
		want := "Charged $20.00 for Dinner."
		if n.UserID == m[0].ID {
			want = "Credited $60.00 for paying Dinner."
		}
		if !strings.HasPrefix(n.Body, want) || !strings.Contains(n.Data, `"/money"`) {
			t.Fatalf("notification for %s: %q %s", n.UserID, n.Body, n.Data)
		}
	}

	// The bank is not a member: only the participants hear about it.
	f.notifier.reset()
	in := equalExpense(nil, 900, m[:3]...)
	in.Description = "Coffee"
	f.confirm(t, in)
	if got := len(f.notifier.notifiedUsers()); got != 3 {
		t.Fatalf("bank-paid expense notified %d members", got)
	}
}

func TestSharedExpenseHistoryCategory(t *testing.T) {
	f := newSharedExpenseFixture(t)
	want := map[string]string{
		"Coffee after training":      "coffee",
		"CAFE run":                   "coffee",
		"Coffee and lunch":           "coffee",
		"Team dinner at Chubby Buns": "food",
		"Friday at Joe's":            "other",
	}
	for description := range want {
		in := equalExpense(nil, 900, f.members[:3]...)
		in.Description = description
		f.confirm(t, in)
	}
	entries, total, err := f.ledger.Entries(LedgerHistoryFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 15 {
		t.Fatalf("history holds %d entries", total)
	}
	for _, entry := range entries {
		if entry.Kind != models.TxnSharedExpense || entry.Category != want[entry.Description] {
			t.Fatalf("%q is categorised %q", entry.Description, entry.Category)
		}
	}
}
