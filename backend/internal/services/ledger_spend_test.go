package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

func TestPersonalSpendUsesImportedGrossSharesAndPlayDates(t *testing.T) {
	requireDB(t)
	source, mapping := testImportSource(t)
	if _, err := ImportSplitwise(source, mapping, nil); err != nil {
		t.Fatal(err)
	}
	var alice models.User
	if err := database.DB.Where("email = ?", "alice@example.test").First(&alice).Error; err != nil {
		t.Fatal(err)
	}
	ls := NewLedgerService()
	beforeEntries, beforeTransactions := countEntries(t)
	// The two April 2 charges were recorded on April 3. Both belong to April 2.
	spend, err := ls.MySpend(alice.ID, time.Date(2024, 4, 2, 23, 0, 0, 0, utils.SydneyLocation))
	if err != nil || spend.YTDCents != 700 || spend.AllTimeCents != 700 || spend.Months[3].AmountCents != 700 {
		t.Fatalf("play date spend %+v: %v", spend, err)
	}
	// Alice paid $9 for the group on April 4 and received a $6 net credit.
	// Her own $3 charge must still count. Her $100 deposit must not count.
	spend, err = ls.MySpend(alice.ID, time.Date(2024, 4, 5, 0, 0, 0, 0, utils.SydneyLocation))
	if err != nil || spend.YTDCents != 1000 || spend.AllTimeCents != 1000 || spend.Months[3].AmountCents != 1000 {
		t.Fatalf("gross spend %+v: %v", spend, err)
	}
	if spend.RecordedFrom == nil || *spend.RecordedFrom != "2024-04-02" {
		t.Fatalf("recorded from %+v", spend)
	}
	spend, err = ls.MySpend(alice.ID, time.Date(2026, 1, 1, 0, 0, 0, 0, utils.SydneyLocation))
	if err != nil || spend.YTDCents != 0 || spend.AllTimeCents != 1000 || len(spend.Months) != 1 {
		t.Fatalf("later year %+v: %v", spend, err)
	}
	afterEntries, afterTransactions := countEntries(t)
	if beforeEntries != afterEntries || beforeTransactions != afterTransactions {
		t.Fatal("spend read changed the ledger")
	}
}

// Deterministic native records let the calendar tests run at any time of year.
func recordSpendSession(t *testing.T, ls *LedgerService, member models.User, date time.Time, cents int64) *models.Settlement {
	t.Helper()
	session := models.Session{Title: "Spend test", SessionDate: date, StartTime: "00:00", EndTime: "02:00", Courts: 1, CreatedBy: member.ID}
	if err := database.DB.Create(&session).Error; err != nil {
		t.Fatal(err)
	}
	account, err := ls.PlayerAccountID(member.ID)
	if err != nil {
		t.Fatal(err)
	}
	txn, err := ls.Post(PostInput{
		Kind: models.TxnSessionSettlement, SessionID: &session.ID, CreatedBy: member.ID,
		// Posting time intentionally differs from play date.
		OccurredAt: date.AddDate(0, 1, 0),
		Movements:  []Movement{{AccountID: account, AmountCents: -cents}, {AccountID: clubAccount(t, ls, models.AccountKindSurplus), AmountCents: cents}},
	})
	if err != nil {
		t.Fatal(err)
	}
	settlement := models.Settlement{SessionID: session.ID, TransactionID: txn.ID, SettledBy: member.ID,
		Lines: []models.ChargeLine{{UserID: member.ID, AmountCents: cents}},
	}
	if err := database.DB.Create(&settlement).Error; err != nil {
		t.Fatal(err)
	}
	return &settlement
}

func TestPersonalSpendSydneyYearBoundaryAndEmptyHistory(t *testing.T) {
	ls, member, _ := newLedger(t)
	now := time.Date(2025, 12, 31, 13, 0, 0, 0, time.UTC) // Jan 1 in Sydney.
	empty, err := ls.MySpend(uuid.New(), now)
	if err != nil || empty.Year != 2026 || empty.AsOf != "2026-01-01" || empty.RecordedFrom != nil || empty.AllTimeCents != 0 || empty.YTDCents != 0 || len(empty.Months) != 1 {
		t.Fatalf("empty %+v: %v", empty, err)
	}
	date := func(year int, month time.Month, day int) time.Time {
		return time.Date(year, month, day, 0, 0, 0, 0, utils.SydneyLocation)
	}
	recordSpendSession(t, ls, member, date(2025, 12, 31), 399)
	recordSpendSession(t, ls, member, date(2026, 1, 1), 1001)
	recordSpendSession(t, ls, member, date(2026, 1, 2), 3000)
	recordSpendSession(t, ls, member, date(2027, 1, 1), 9999)
	spend, err := ls.MySpend(member.ID, now)
	if err != nil || spend.YTDCents != 1001 || spend.AllTimeCents != 1400 || spend.Months[0].AmountCents != 1001 {
		t.Fatalf("boundary spend %+v: %v", spend, err)
	}
	// Sydney switches off daylight savings in April and back on in October.
	recordSpendSession(t, ls, member, date(2026, 4, 5), 201)
	recordSpendSession(t, ls, member, date(2026, 10, 4), 301)
	spend, err = ls.MySpend(member.ID, date(2026, 10, 5))
	if err != nil || spend.YTDCents != 4503 || spend.AllTimeCents != 4902 || len(spend.Months) != 10 || spend.Months[3].AmountCents != 201 || spend.Months[9].AmountCents != 301 || spend.Months[1].AmountCents != 0 {
		t.Fatalf("DST spend %+v: %v", spend, err)
	}
}

func TestPersonalSpendNativeGuestsReversalsAndReplacement(t *testing.T) {
	f := newSettlementFixture(t, 48, 20000)
	host, other, comped := f.member(t, "host"), f.member(t, "other"), f.member(t, "comped")
	lines := []LineInput{{UserID: host.ID, InBase: true}, {UserID: host.ID, GuestName: "Guest", InBase: true}, {UserID: other.ID, InBase: true}, {UserID: comped.ID, InBase: true, Comped: true}}
	settle := func() *models.Settlement {
		t.Helper()
		record, _, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: lines})
		if err != nil {
			t.Fatal(err)
		}
		if err := database.DB.Preload("Lines").First(record, "id = ?", record.ID).Error; err != nil {
			t.Fatal(err)
		}
		return record
	}
	first := settle()
	var hostCharge int64
	for _, line := range first.Lines {
		if line.UserID == host.ID {
			hostCharge += line.AmountCents
		}
	}
	if hostCharge == 0 {
		t.Fatal("fixture has no host charges")
	}
	if _, err := f.ledger.RecordTopup(CashInput{UserID: host.ID, AmountCents: 50000, CreatedBy: f.admin.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.ledger.RecordWithdrawal(CashInput{UserID: host.ID, AmountCents: 1000, CreatedBy: f.admin.ID}); err != nil {
		t.Fatal(err)
	}
	check := func(userID uuid.UUID, want int64) {
		t.Helper()
		spend, err := f.ledger.MySpend(userID, utils.NowInSydney())
		if err != nil || spend.AllTimeCents != want {
			t.Fatalf("spend %+v want %d: %v", spend, want, err)
		}
	}
	check(host.ID, hostCharge)
	check(comped.ID, 0)
	if _, err := f.settlement.ReverseSettlement(first.ID, "correction", f.admin.ID); err != nil {
		t.Fatal(err)
	}
	check(host.ID, 0)
	replacement := settle()
	check(host.ID, hostCharge)
	// The generic transaction reversal path does not mark the settlement row.
	if _, err := f.ledger.ReverseTransaction(replacement.TransactionID, "correction", f.admin.ID); err != nil {
		t.Fatal(err)
	}
	check(host.ID, 0)
}

func TestPersonalSpendIncludesHistoryBeyondLedgerPageLimit(t *testing.T) {
	ls, member, _ := newLedger(t)
	for i := 0; i < 205; i++ {
		recordSpendSession(t, ls, member, time.Date(2025, 1, 1+i, 0, 0, 0, 0, utils.SydneyLocation), 101)
	}
	spend, err := ls.MySpend(member.ID, time.Date(2025, 12, 31, 0, 0, 0, 0, utils.SydneyLocation))
	if err != nil || spend.YTDCents != 20705 || spend.AllTimeCents != 20705 {
		t.Fatalf("full history %+v: %v", spend, err)
	}
}
