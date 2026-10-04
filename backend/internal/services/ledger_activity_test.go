package services

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/splitwise"
)

func TestGameActivityGroupsSourceBeforePagingAndKeepsGrossShares(t *testing.T) {
	requireDB(t)
	text := strings.ReplaceAll(strings.ReplaceAll(importFixture, "Deposit", "Top-Up"), "Former deposit", "Former Topup")
	source, err := splitwise.Read(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	_, mapping := testImportSource(t)
	if _, err := ImportSplitwise(source, mapping, nil); err != nil {
		t.Fatal(err)
	}
	var alice, bob models.User
	database.DB.Where("email = ?", "alice@example.test").First(&alice)
	database.DB.Where("email = ?", "bob@example.test").First(&bob)
	beforeEntries, beforeTransactions := countEntries(t)
	ls := NewLedgerService()
	activities, total, err := ls.Activity(LedgerHistoryFilter{Limit: 200})
	if err != nil || total != 4 {
		t.Fatalf("activities %d: %v", total, err)
	}
	games := map[string]*LedgerGameView{}
	for _, a := range activities {
		if a.Game != nil {
			games[a.Game.PlayedDate] = a.Game
		}
	}
	if len(games) != 2 {
		t.Fatalf("games %+v", games)
	}
	combined := games["2024-04-02"]
	if combined == nil || combined.TotalChargedCents != 1900 || combined.SourceCount != 2 || len(combined.Shares) != 3 {
		t.Fatalf("combined %+v", combined)
	}
	for _, s := range combined.Shares {
		switch s.MemberName {
		case "Alice":
			if s.ChargeCents != 700 || s.BalanceAfterCents != 9300 {
				t.Fatal(s)
			}
		case "Bob":
			if s.ChargeCents != 700 || s.BalanceAfterCents != -700 {
				t.Fatal(s)
			}
		case "Former":
			if !s.Inactive || s.UserID != nil || s.ChargeCents != 500 || s.BalanceAfterCents != 0 {
				t.Fatal(s)
			}
		default:
			t.Fatal(s)
		}
	}
	funded := games["2024-04-04"]
	if funded.TotalChargedCents != 900 {
		t.Fatal(funded)
	}
	for _, s := range funded.Shares {
		if s.MemberName == "Alice" && (s.ChargeCents != 300 || s.PaidCents != 900 || s.AmountCents != 600 || s.BalanceAfterCents != 9900) {
			t.Fatalf("payer %+v", s)
		}
	}
	var paged []LedgerActivityView
	for offset := 0; offset < 4; offset++ {
		page, count, err := ls.Activity(LedgerHistoryFilter{Limit: 1, Offset: offset})
		if err != nil || count != 4 || len(page) != 1 {
			t.Fatalf("page %+v/%d: %v", page, count, err)
		}
		paged = append(paged, page...)
	}
	if !reflect.DeepEqual(activities, paged) {
		t.Fatal("paging split or repeated a game")
	}
	mine, count, err := ls.Activity(LedgerHistoryFilter{UserID: &bob.ID, Limit: 200})
	if err != nil || count != 2 || len(mine[1].Game.Shares) != 3 {
		t.Fatalf("mine %+v/%d: %v", mine, count, err)
	}
	topups, count, err := ls.Activity(LedgerHistoryFilter{TopupsOnly: true, Limit: 200})
	if err != nil || count != 2 {
		t.Fatalf("topups %+v/%d: %v", topups, count, err)
	}
	for _, a := range topups {
		if a.Game != nil || a.Entry.Category != "topup" {
			t.Fatal(a)
		}
	}
	empty, count, err := ls.Activity(LedgerHistoryFilter{UserID: &bob.ID, TopupsOnly: true})
	if err != nil || count != 0 || len(empty) != 0 {
		t.Fatalf("empty %+v/%d: %v", empty, count, err)
	}
	afterEntries, afterTransactions := countEntries(t)
	if beforeEntries != afterEntries || beforeTransactions != afterTransactions {
		t.Fatal("activity read changed the ledger")
	}
}

func TestGameActivityNativeGuestCompedAndSameDaySessions(t *testing.T) {
	f := newSettlementFixture(t, 48, 20000)
	host := f.member(t, "host")
	comped := f.member(t, "comped")
	lines := []LineInput{{UserID: host.ID, InBase: true}, {UserID: host.ID, GuestName: "Guest One", InBase: true}, {UserID: comped.ID, InBase: true, Comped: true}}
	first, preview, err := f.settlement.Settle(SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID, Lines: lines})
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.sessions.CreateSession(CreateSessionInput{Title: "Second game", SessionDate: f.session.SessionDate, StartTime: "16:00", EndTime: "18:00", Courts: 1, CreatedBy: f.admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.settlement.Settle(SettleInput{SessionID: second.ID, SettledBy: f.admin.ID, Lines: []LineInput{{UserID: comped.ID, InBase: true, Comped: true}}}); err != nil {
		t.Fatal(err)
	}
	activities, total, err := f.ledger.Activity(LedgerHistoryFilter{UserID: &comped.ID, Limit: 200})
	if err != nil || total != 2 {
		t.Fatalf("comped history %+v/%d: %v", activities, total, err)
	}
	found := false
	for _, a := range activities {
		if a.Game.SessionID != f.session.ID {
			continue
		}
		found = true
		if a.Game.TotalChargedCents != preview.Totals.ChargedCents || len(a.Game.Shares) != 2 {
			t.Fatal(a.Game)
		}
		for _, s := range a.Game.Shares {
			if *s.UserID == host.ID && (s.ChargeCents != preview.Totals.ChargedCents || s.BalanceAfterCents != -preview.Totals.ChargedCents || len(s.GuestNames) != 1 || s.GuestNames[0] != "Guest One") {
				t.Fatal(s)
			}
			if *s.UserID == comped.ID && (s.ChargeCents != 0 || s.BalanceAfterCents != 0) {
				t.Fatal(s)
			}
		}
	}
	if !found {
		t.Fatal("first game missing")
	}
	if _, err := f.settlement.ReverseSettlement(first.ID, "test correction", f.admin.ID); err != nil {
		t.Fatal(err)
	}
	activities, total, err = f.ledger.Activity(LedgerHistoryFilter{Limit: 200})
	if err != nil || total != 3 {
		t.Fatalf("correction %+v/%d: %v", activities, total, err)
	}
	reversals := 0
	for _, a := range activities {
		if a.Entry != nil && a.Entry.Kind == models.TxnReversal {
			reversals++
		}
		if a.Game != nil && a.Game.SessionID == first.SessionID && !a.Game.Reversed {
			t.Fatal("missing reversal label")
		}
	}
	if reversals != 1 {
		t.Fatalf("reversals %d", reversals)
	}
}

func TestGameActivityOverFiftyKeepsEverySplitOnOnePage(t *testing.T) {
	ls, alice, aAccount := newLedger(t)
	bob := newUser(t, "group-bob")
	bAccount, err := ls.EnsurePlayerAccount(bob.ID, bob.Name)
	if err != nil {
		t.Fatal(err)
	}
	surplus := clubAccount(t, ls, models.AccountKindSurplus)
	for i := 0; i < 53; i++ {
		session := models.Session{Courts: 1, Title: fmt.Sprintf("Game %d", i), SessionDate: time.Date(2024, 1, 1+i, 0, 0, 0, 0, time.UTC), CreatedBy: alice.ID}
		if err := database.DB.Create(&session).Error; err != nil {
			t.Fatal(err)
		}
		txn, err := ls.Post(PostInput{Kind: models.TxnSessionSettlement, SessionID: &session.ID, OccurredAt: session.SessionDate, CreatedBy: alice.ID, Movements: []Movement{{AccountID: aAccount, AmountCents: -100}, {AccountID: bAccount, AmountCents: -200}, {AccountID: surplus, AmountCents: 300}}})
		if err != nil {
			t.Fatal(err)
		}
		settlement := models.Settlement{SessionID: session.ID, TransactionID: txn.ID, SettledBy: alice.ID}
		if err := database.DB.Create(&settlement).Error; err != nil {
			t.Fatal(err)
		}
		for _, line := range []models.ChargeLine{{SettlementID: settlement.ID, UserID: alice.ID, InBase: true, AmountCents: 100}, {SettlementID: settlement.ID, UserID: bob.ID, InBase: true, AmountCents: 200}} {
			if err := database.DB.Create(&line).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	first, total, err := ls.Activity(LedgerHistoryFilter{Limit: 50})
	if err != nil || total != 53 || len(first) != 50 {
		t.Fatalf("first %d/%d: %v", len(first), total, err)
	}
	last, total, err := ls.Activity(LedgerHistoryFilter{Limit: 50, Offset: 50})
	if err != nil || total != 53 || len(last) != 3 {
		t.Fatalf("last %d/%d: %v", len(last), total, err)
	}
	seen := map[uuid.UUID]bool{}
	for _, a := range append(first, last...) {
		if seen[a.ID] || a.Game == nil || len(a.Game.Shares) != 2 || a.Game.TotalChargedCents != 300 {
			t.Fatalf("group %+v", a)
		}
		seen[a.ID] = true
	}
	// A request beyond the final page still returns the complete group count.
	empty, total, err := ls.Activity(LedgerHistoryFilter{Limit: 50, Offset: 1000})
	if err != nil || total != 53 || len(empty) != 0 {
		t.Fatalf("beyond %+v/%d: %v", empty, total, err)
	}
}
