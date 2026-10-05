package services

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services/money"
	"github.com/weekday-masters/backend/internal/utils"
	"sync"
	"testing"
)

func TestExpenseExtraHourGroupValidation(t *testing.T) {
	f, s, in := expenseFixture(t)
	for _, tc := range []struct {
		name  string
		hours int
		extra []uuid.UUID
	}{
		{"empty", 3, []uuid.UUID{}},
		{"not in main group", 3, []uuid.UUID{f.admin.ID}},
		{"duplicate", 3, []uuid.UUID{in.ParticipantIDs[0], in.ParticipantIDs[0]}},
		{"two hours with extra players", 2, in.ParticipantIDs},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := in
			bad.TotalHours, bad.ExtraParticipantIDs = tc.hours, tc.extra
			if _, err := s.Preview(f.session.ID, bad, &f.admin); err == nil {
				t.Fatal("invalid extra-hour group accepted")
			}
		})
	}
}

func TestExpenseHourlyShuttleValueRounding(t *testing.T) {
	for _, tc := range []struct{ value, base, extra int64 }{
		{0, 0, 0}, {1, 1, 0}, {2, 1, 1}, {3, 2, 1}, {4, 3, 1}, {5, 3, 2},
	} {
		t.Run(fmt.Sprintf("%d_cents", tc.value), func(t *testing.T) {
			f := newSettlementFixture(t, 0, 0)
			f.admin.Role = models.RoleAdmin
			stayer, leaver := f.member(t, "stayer"), f.member(t, "leaver")
			// An indivisible actual count must not be rounded separately per hour.
			count := 1
			p, _, err := f.settlement.cost(f.session.ID, rates{actualShuttles: &count, baseHours: 2, extraHours: 1}, []LineInput{
				{UserID: stayer.ID, InBase: true, InExtra: true}, {UserID: leaver.ID, InBase: true},
			}, money.Stock{Units: 1, ValueCents: tc.value})
			if err != nil {
				t.Fatal(err)
			}
			if p.Bands["base"].ShuttleCents != tc.base || p.Bands["extra"].ShuttleCents != tc.extra || p.Totals.ChargedCents != tc.value || p.Totals.ShuttleUnits != 1 || p.StockAfter.Units != 0 || p.StockAfter.AmountCents != 0 {
				t.Fatalf("incorrect time allocation or stock consumption: %+v", p)
			}
		})
	}
}

func expenseFixture(t *testing.T) (*settlementFixture, *ExpenseService, ExpenseInput) {
	f := newSettlementFixture(t, 24, 10000)
	f.admin.Role = models.RoleAdmin
	users := []uuid.UUID{}
	for _, name := range []string{"Ada", "Ben", "Chen", "Dev"} {
		u := f.member(t, name)
		users = append(users, u.ID)
	}
	count := 8
	return f, NewExpenseService(f.settlement, f.ledger), ExpenseInput{TotalHours: 3, ShuttlesUsed: &count, ParticipantIDs: users}
}
func TestExpenseActualUsageAndEqualShares(t *testing.T) {
	f, s, in := expenseFixture(t)
	p, err := s.Preview(f.session.ID, in, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if p.Settlement.Totals.ShuttleUnits != 8 || p.Settlement.StockAfter.Units != 16 || p.Settlement.Totals.CourtCents != 8300 {
		t.Fatalf("preview=%+v", p)
	}
	var count int64
	database.DB.Model(&models.Settlement{}).Count(&count)
	if count != 0 {
		t.Fatal("preview wrote money")
	}
	min, max := p.Settlement.Lines[0].AmountCents, p.Settlement.Lines[0].AmountCents
	var total int64
	for _, l := range p.Settlement.Lines {
		total += l.AmountCents
		if l.AmountCents < min {
			min = l.AmountCents
		}
		if l.AmountCents > max {
			max = l.AmountCents
		}
	}
	if max-min > 1 || total != p.Settlement.Totals.ChargedCents {
		t.Fatal("not an exact equal split")
	}
	in = p.Input
	in.ExpectedPreview = p.Fingerprint
	record, err := s.Confirm(f.session.ID, in, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	if record.ActualShuttles == nil || *record.ActualShuttles != 8 {
		t.Fatal("actual count not recorded")
	}
	view, err := f.settlement.SettlementForSession(f.session.ID)
	if err != nil || view.Rates.ActualShuttles == nil || *view.Rates.ActualShuttles != 8 || view.Totals != p.Settlement.Totals || len(view.Lines) != 4 {
		t.Fatalf("history does not match the confirmed preview: %+v %v", view, err)
	}
	assertBalanced(t)
	if _, err = s.Confirm(f.session.ID, in, &f.admin); err == nil {
		t.Fatal("duplicate accepted")
	}
	if _, err = f.settlement.ReverseSettlement(record.ID, "correction", f.admin.ID); err != nil {
		t.Fatal(err)
	}
	stock, err := f.ledger.StockPosition(nil)
	if err != nil || stock.Units != 24 || stock.ValueCents != 10000 {
		t.Fatalf("reversal: %+v %v", stock, err)
	}
}
func TestExpenseRejectsStalePreview(t *testing.T) {
	for _, change := range []string{"stock", "rates", "players", "extra-hour players"} {
		t.Run(change, func(t *testing.T) {
			f, s, in := expenseFixture(t)
			p, err := s.Preview(f.session.ID, in, &f.admin)
			if err != nil {
				t.Fatal(err)
			}
			in = p.Input
			in.ExpectedPreview = p.Fingerprint
			switch change {
			case "stock":
				_, err = f.ledger.RecordShuttlePurchase(AssetPurchaseInput{AmountCents: 6000, Units: 12, CreatedBy: f.admin.ID})
			case "rates":
				err = database.DB.Model(&models.Club{}).Where("1=1").Update("base_rate_cents", 3100).Error
			case "players":
				in.ParticipantIDs = in.ParticipantIDs[:2]
				in.ExtraParticipantIDs = in.ParticipantIDs
			case "extra-hour players":
				in.ExtraParticipantIDs = in.ExtraParticipantIDs[:2]
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = s.Confirm(f.session.ID, in, &f.admin)
			e, ok := AsLedgerError(err)
			if !ok || e.Code != "preview_changed" {
				t.Fatalf("expected stale preview, got %v", err)
			}
			var count int64
			database.DB.Model(&models.Settlement{}).Count(&count)
			if count != 0 {
				t.Fatal("stale write")
			}
		})
	}
}
func TestExpenseValidationAndMemberAccess(t *testing.T) {
	f, s, in := expenseFixture(t)
	member := f.admin
	member.Role = models.RolePlayer
	if _, err := s.Preview(f.session.ID, in, &member); err == nil {
		t.Fatal("member can preview expenses")
	}
	if _, err := s.Confirm(f.session.ID, in, &member); err == nil {
		t.Fatal("member can confirm")
	}
	for _, n := range []int{-1, 201} {
		bad := in
		bad.ShuttlesUsed = &n
		if _, err := s.Preview(f.session.ID, bad, &f.admin); err == nil {
			t.Fatal("invalid count accepted")
		}
	}
	for _, h := range []int{0, 1, 4} {
		bad := in
		bad.TotalHours = h
		if _, err := s.Preview(f.session.ID, bad, &f.admin); err == nil {
			t.Fatal("invalid hours accepted")
		}
	}
	missing := in
	missing.ShuttlesUsed = nil
	if _, err := s.Preview(f.session.ID, missing, &f.admin); err == nil {
		t.Fatal("missing shuttles accepted")
	}
	empty := in
	empty.ParticipantIDs = []uuid.UUID{}
	if _, err := s.Preview(f.session.ID, empty, &f.admin); err == nil {
		t.Fatal("empty group accepted")
	}
	dup := in
	dup.ParticipantIDs = []uuid.UUID{in.ParticipantIDs[0], in.ParticipantIDs[0]}
	if _, err := s.Preview(f.session.ID, dup, &f.admin); err == nil {
		t.Fatal("duplicate accepted")
	}
	zero := 0
	in.ShuttlesUsed = &zero
	p, err := s.Preview(f.session.ID, in, &f.admin)
	if err != nil || p.Settlement.Totals.ShuttleUnits != 0 {
		t.Fatalf("zero rejected: %v", err)
	}
	if _, err = s.Confirm(f.session.ID, in, &f.admin); err == nil {
		t.Fatal("confirmation without preview accepted")
	}
}
func TestExpenseConcurrentConfirmation(t *testing.T) {
	f, s, in := expenseFixture(t)
	p, err := s.Preview(f.session.ID, in, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	in = p.Input
	in.ExpectedPreview = p.Fingerprint
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, err := s.Confirm(f.session.ID, in, &f.admin); results <- err }()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("settled %d times", success)
	}
	assertBalanced(t)
}

func TestExpenseDifferentSessionsCannotConsumeTheSameStock(t *testing.T) {
	f, s, in := expenseFixture(t)
	second, err := f.sessions.CreateSession(CreateSessionInput{Title: "Second game", SessionDate: f.session.SessionDate, StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: f.admin.ID})
	if err != nil {
		t.Fatal(err)
	}
	units := 20
	in.ShuttlesUsed = &units
	ids := []uuid.UUID{f.session.ID, second.ID}
	inputs := make([]ExpenseInput, 2)
	for i, id := range ids {
		p, err := s.Preview(id, in, &f.admin)
		if err != nil {
			t.Fatal(err)
		}
		inputs[i] = p.Input
		inputs[i].ExpectedPreview = p.Fingerprint
	}
	start := make(chan struct{})
	results := make(chan error, 2)
	for i := range ids {
		go func(i int) { <-start; _, err := s.Confirm(ids[i], inputs[i], &f.admin); results <- err }(i)
	}
	close(start)
	success := 0
	for range ids {
		err := <-results
		if err == nil {
			success++
		} else if e, ok := AsLedgerError(err); !ok || (e.Code != "shuttle_stock_short" && e.Code != "preview_changed") {
			t.Fatalf("unexpected conflict: %v", err)
		}
	}
	stock, err := f.ledger.StockPosition(nil)
	if err != nil || success != 1 || stock.Units != 4 {
		t.Fatalf("success=%d stock=%+v err=%v", success, stock, err)
	}
	assertBalanced(t)
}

func TestExpenseDefaultsAndUnsettledList(t *testing.T) {
	f, s, in := expenseFixture(t)
	statuses := []models.RSVPStatus{models.RSVPStatusIn, models.RSVPStatusIn, models.RSVPStatusWaitlisted, models.RSVPStatusOut}
	for i, id := range in.ParticipantIDs {
		if err := database.DB.Create(&models.RSVP{SessionID: f.session.ID, UserID: id, Status: statuses[i]}).Error; err != nil {
			t.Fatal(err)
		}
	}
	expected := append([]uuid.UUID(nil), in.ParticipantIDs[:2]...)
	in.ParticipantIDs = nil
	p, err := s.Preview(f.session.ID, in, &f.admin)
	if err != nil || len(p.Input.ParticipantIDs) != 2 {
		t.Fatalf("default RSVPs: %+v %v", p, err)
	}
	for _, id := range p.Input.ParticipantIDs {
		if id != expected[0] && id != expected[1] {
			t.Fatal("charged a waitlisted or out RSVP")
		}
	}
	for _, days := range []int{-3, -2, 1} {
		item, err := f.sessions.CreateSession(CreateSessionInput{Title: "Other game", SessionDate: utils.NowInSydney().AddDate(0, 0, days), StartTime: "18:00", EndTime: "20:00", Courts: 1, CreatedBy: f.admin.ID})
		if err != nil {
			t.Fatal(err)
		}
		if days == -2 {
			if err := database.DB.Model(item).Update("status", models.SessionStatusCancelled).Error; err != nil {
				t.Fatal(err)
			}
		}
	}
	items, err := s.ListUnsettledSessions()
	if err != nil || len(items) != 2 || items[1].SessionID != f.session.ID || items[1].PlayerCount != 2 {
		t.Fatalf("unsettled=%+v err=%v", items, err)
	}
	in = p.Input
	in.ExpectedPreview = p.Fingerprint
	record, err := s.Confirm(f.session.ID, in, &f.admin)
	if err != nil {
		t.Fatal(err)
	}
	items, err = s.ListUnsettledSessions()
	if err != nil || len(items) != 1 {
		t.Fatalf("settled session remains: %+v %v", items, err)
	}
	if _, err := f.settlement.ReverseSettlement(record.ID, "Test correction", f.admin.ID); err != nil {
		t.Fatal(err)
	}
	items, err = s.ListUnsettledSessions()
	if err != nil || len(items) != 2 {
		t.Fatalf("reversed session missing: %+v %v", items, err)
	}
}
