package services

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

func importedHistoryFixture(t *testing.T) (*SettlementService, models.User) {
	t.Helper()
	requireDB(t)
	source, mapping := testImportSource(t)
	assets := &ImportAssets{BankCents: 4000, CourtCreditCents: 50000, ShuttleUnits: 36, ShuttleCents: 15000}
	if _, err := ImportSplitwise(source, mapping, assets); err != nil {
		t.Fatal(err)
	}
	var alice models.User
	if err := database.DB.Where("email = ?", "alice@example.test").First(&alice).Error; err != nil {
		t.Fatal(err)
	}
	return NewSettlementService(NewLedgerService()), alice
}

func historicalSchedule(t *testing.T, creator uuid.UUID, date, start string) *models.Session {
	t.Helper()
	day, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	session, err := NewSessionService().CreateSession(CreateSessionInput{
		Title: "Scheduled game", SessionDate: day, StartTime: start,
		EndTime: "22:00", Courts: 1, CreatedBy: creator,
	})
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func TestImportedHistoryReplacesDuplicateSchedulesBeforePaging(t *testing.T) {
	service, alice := importedHistoryFixture(t)
	// Sydney 00:30 falls on the previous UTC day. The stored play date must win.
	early := historicalSchedule(t, alice.ID, "2024-04-02", "00:30")
	duplicate := historicalSchedule(t, alice.ID, "2024-04-02", "20:00")
	cancelled := historicalSchedule(t, alice.ID, "2024-04-04", "20:00")
	cancelled.Status = models.SessionStatusCancelled
	if err := database.DB.Save(cancelled).Error; err != nil {
		t.Fatal(err)
	}
	// April 3 is the recorded date of the April 2 game, not its play date.
	unmatched := historicalSchedule(t, alice.ID, "2024-04-03", "20:00")
	seen := map[uuid.UUID]bool{}
	var importedID uuid.UUID
	for offset := 0; offset < 3; offset++ {
		page, total, err := service.ListPastSessions(1, offset)
		if err != nil || total != 3 || len(page) != 1 {
			t.Fatalf("page %d: %+v, total %d, error %v", offset, page, total, err)
		}
		h := page[0]
		if seen[h.SessionID] || h.SessionID == early.ID || h.SessionID == duplicate.ID || h.SessionID == cancelled.ID {
			t.Fatalf("duplicate schedule in history: %+v", h)
		}
		seen[h.SessionID] = true
		if h.SessionID == unmatched.ID {
			if h.Settled || h.ImportedDate != "" || offset != 1 {
				t.Fatalf("unmatched schedule changed: %+v", h)
			}
		} else if !h.Settled || h.ImportedDate == "" {
			t.Fatalf("imported game is not settled: %+v", h)
		}
		if h.ImportedDate == "2024-04-02" {
			importedID = h.SessionID
			if offset != 2 || h.TotalCents != 1900 || h.PlayerCount != 3 {
				t.Fatalf("regular and extra-hour charges were lost: %+v", h)
			}
		}
	}
	if !seen[unmatched.ID] || importedID == uuid.Nil {
		t.Fatal("missing native schedule or imported game")
	}
	page, total, err := service.ListPastSessions(1, 3)
	if err != nil || total != 3 || len(page) != 0 {
		t.Fatalf("end of history: %+v, %d, %v", page, total, err)
	}
	canonical, err := service.SettlementForSession(importedID)
	if err != nil || canonical == nil || canonical.Imported == nil || len(canonical.Imported.Sources) != 2 {
		t.Fatalf("canonical breakdown: %+v, %v", canonical, err)
	}
	for _, id := range []uuid.UUID{early.ID, duplicate.ID} {
		view, err := service.SettlementForSession(id)
		if err != nil || !reflect.DeepEqual(view, canonical) {
			t.Fatalf("old schedule link lost its imported split: %+v, %v", view, err)
		}
	}
	view, err := service.SettlementForSession(cancelled.ID)
	if err != nil || view == nil || view.Imported == nil || view.Imported.TotalCents != 900 {
		t.Fatalf("cancelled schedule import: %+v, %v", view, err)
	}
}

func TestImportedScheduleCannotChargeMembersAgain(t *testing.T) {
	service, alice := importedHistoryFixture(t)
	session := historicalSchedule(t, alice.ID, "2024-04-02", "20:00")
	rsvp := models.RSVP{SessionID: session.ID, UserID: alice.ID, Status: models.RSVPStatusIn}
	if err := database.DB.Create(&rsvp).Error; err != nil {
		t.Fatal(err)
	}
	before, err := service.ledger.Position()
	if err != nil {
		t.Fatal(err)
	}
	balanceBefore, err := service.ledger.BalanceOfUser(alice.ID)
	if err != nil {
		t.Fatal(err)
	}
	var transactionsBefore int64
	if err := database.DB.Model(&models.Transaction{}).Count(&transactionsBefore).Error; err != nil {
		t.Fatal(err)
	}
	in := SettleInput{SessionID: session.ID, SettledBy: alice.ID, Lines: []LineInput{{UserID: alice.ID, InBase: true}}}
	assertImportedError := func(err error) {
		t.Helper()
		domainErr, ok := AsLedgerError(err)
		if !ok || domainErr.Code != CodeNotSettleable || !strings.Contains(domainErr.Message, "settled in Splitwise") {
			t.Fatalf("expected an imported settlement refusal, got %v", err)
		}
	}
	_, err = service.Preview(in)
	assertImportedError(err)
	_, _, err = service.Settle(in)
	assertImportedError(err)
	view, err := service.SettlementForSession(session.ID)
	if err != nil || view == nil {
		t.Fatalf("breakdown: %+v, %v", view, err)
	}
	in.SessionID = view.Session.ID
	_, err = service.Preview(in)
	assertImportedError(err)
	if _, _, err := service.Settle(in); err == nil {
		t.Fatal("canonical imported ID accepted for native settlement")
	}
	after, err := service.ledger.Position()
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("club position changed: before %+v, after %+v, error %v", before, after, err)
	}
	balanceAfter, err := service.ledger.BalanceOfUser(alice.ID)
	if err != nil || balanceAfter != balanceBefore {
		t.Fatalf("member balance changed: %d to %d, %v", balanceBefore, balanceAfter, err)
	}
	for _, check := range []struct {
		model any
		want  int64
	}{{&models.Transaction{}, transactionsBefore}, {&models.Settlement{}, 0}, {&models.Session{}, 1}, {&models.RSVP{}, 1}} {
		var count int64
		if err := database.DB.Model(check.model).Count(&count).Error; err != nil || count != check.want {
			t.Fatalf("%T count %d, want %d: %v", check.model, count, check.want, err)
		}
	}
}

func TestImportedHistoryKeepsNativeSettlementsAndReversals(t *testing.T) {
	service, alice := importedHistoryFixture(t)
	var nativeIDs []uuid.UUID
	for _, reverse := range []bool{false, true} {
		session := historicalSchedule(t, alice.ID, "2024-04-03", "20:00")
		in := SettleInput{SessionID: session.ID, SettledBy: alice.ID, Lines: []LineInput{{UserID: alice.ID, InBase: true}}}
		settlement, _, err := service.Settle(in)
		if err != nil {
			t.Fatal(err)
		}
		if reverse {
			if _, err := service.ReverseSettlement(settlement.ID, "Correct native game", alice.ID); err != nil {
				t.Fatal(err)
			}
		}
		// A schedule date edit must not let an import take over a native posting.
		session.SessionDate = time.Date(2024, 4, 2, 0, 0, 0, 0, time.UTC)
		if err := database.DB.Save(session).Error; err != nil {
			t.Fatal(err)
		}
		nativeIDs = append(nativeIDs, session.ID)
		view, err := service.SettlementForSession(session.ID)
		if err != nil || (reverse && view != nil) || (!reverse && (view == nil || view.Imported != nil)) {
			t.Fatalf("native breakdown replaced by import: %+v, %v", view, err)
		}
		if reverse {
			if _, err := service.Preview(in); err != nil {
				t.Fatalf("native reversal cannot be corrected: %v", err)
			}
		}
	}
	history, total, err := service.ListPastSessions(50, 0)
	if err != nil || total != 4 || len(history) != 4 {
		t.Fatalf("history lost native records: %+v, %d, %v", history, total, err)
	}
	for i, id := range nativeIDs {
		found := false
		for _, h := range history {
			if h.SessionID == id {
				found = true
				if h.ImportedDate != "" || h.Settled != (i == 0) {
					t.Fatalf("native settlement state changed: %+v", h)
				}
			}
		}
		if !found {
			t.Fatalf("native session %s hidden", id)
		}
	}
}
