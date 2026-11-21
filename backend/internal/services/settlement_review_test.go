package services

import (
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
)

// Settle posts only the costing somebody reviewed. These tests pin that rule at
// the service, below the handler, because the handler is not the only caller.

func expectLedgerCode(t *testing.T, err error, code LedgerErrorCode, status int) {
	t.Helper()
	ledgerErr, ok := AsLedgerError(err)
	if !ok {
		t.Fatalf("error = %v, want a ledger error with code %s", err, code)
	}
	if ledgerErr.Code != code || ledgerErr.Status != status {
		t.Fatalf("error = %s (%d), want %s (%d)", ledgerErr.Code, ledgerErr.Status, code, status)
	}
}

func liveSettlements(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := database.DB.Model(&models.Settlement{}).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSettleRefusesWithoutAReviewedPreview(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}}}

	entries, transactions := countEntries(t)
	for name, fingerprint := range map[string]string{
		"none":      "",
		"too short": strings.Repeat("a", 63),
		"too long":  strings.Repeat("a", 65),
	} {
		t.Run(name, func(t *testing.T) {
			input.ExpectedPreview = fingerprint
			_, _, err := f.settlement.Settle(input)
			expectLedgerCode(t, err, "preview_required", 409)
			if e, tr := countEntries(t); e != entries || tr != transactions || liveSettlements(t) != 0 {
				t.Fatal("an unreviewed settlement wrote financial records")
			}
		})
	}
}

// The reproduction, with its figures: reviewed at $101.67, and a tube recorded
// before confirming would have posted $122.50.
func TestSettleRefusesAPreviewThatAStockPurchaseMadeStale(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}}}

	reviewed, err := f.settlement.Preview(input)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Totals.ChargedCents != 10167 {
		t.Fatalf("reviewed charge = %d, want 10167", reviewed.Totals.ChargedCents)
	}

	// Twelve more for $125 takes the bag to 36 for $225: 625 cents a shuttle.
	if _, err := f.ledger.RecordShuttlePurchase(AssetPurchaseInput{
		AmountCents: 12500, Units: 12, Description: "dearer tube", CreatedBy: f.admin.ID,
	}); err != nil {
		t.Fatal(err)
	}
	now, err := f.settlement.Preview(input)
	if err != nil {
		t.Fatal(err)
	}
	if now.Totals.ChargedCents != 12250 {
		t.Fatalf("charge after the purchase = %d, want 12250", now.Totals.ChargedCents)
	}
	if now.Fingerprint == reviewed.Fingerprint {
		t.Fatal("a different costing kept the fingerprint")
	}

	entries, transactions := countEntries(t)
	input.ExpectedPreview = reviewed.Fingerprint
	_, _, err = f.settlement.Settle(input)
	expectLedgerCode(t, err, "preview_changed", 409)
	if e, tr := countEntries(t); e != entries || tr != transactions || liveSettlements(t) != 0 {
		t.Fatal("a stale settlement wrote financial records")
	}
	if got := playerBalance(t, f.ledger, player.ID); got != 0 {
		t.Fatalf("player balance = %d, want 0", got)
	}
	assertBalanced(t)

	// Reviewed again, the new figure is the one that posts.
	input.ExpectedPreview = now.Fingerprint
	if _, posted, err := f.settlement.Settle(input); err != nil {
		t.Fatal(err)
	} else if posted.Totals.ChargedCents != 12250 {
		t.Fatalf("posted %d, want the reviewed 12250", posted.Totals.ChargedCents)
	}
	if got := playerBalance(t, f.ledger, player.ID); got != -12250 {
		t.Fatalf("player balance = %d, want -12250", got)
	}
}

// The fingerprint has to move whenever anything the costing rests on moves:
// who was there, the hours and rates, and the stock.
func TestSettlementFingerprintBindsAttendanceRatesAndStock(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	a, b := f.member(t, "a"), f.member(t, "b")
	base := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: a.ID, InBase: true}, {UserID: b.ID, InBase: true}}}

	fingerprint := func(t *testing.T, in SettleInput) string {
		t.Helper()
		preview, err := f.settlement.Preview(in)
		if err != nil {
			t.Fatal(err)
		}
		return preview.Fingerprint
	}
	reviewed := fingerprint(t, base)
	if again := fingerprint(t, base); again != reviewed || len(reviewed) != PreviewFingerprintLength {
		t.Fatalf("the same settlement gave %q then %q", reviewed, again)
	}

	one, rate := 1.0, int64(3500)
	hours, perHour := 3.0, 4.0
	inputs := map[string]SettleInput{
		"someone left out": {Lines: []LineInput{{UserID: a.ID, InBase: true}}},
		"a guest":          {Lines: []LineInput{{UserID: a.ID, InBase: true}, {UserID: a.ID, GuestName: "Sam", InBase: true}}},
		"someone comped":   {Lines: []LineInput{{UserID: a.ID, InBase: true}, {UserID: b.ID, InBase: true, Comped: true}}},
		// Same people, same total, other way round: the odd cent lands by line.
		"the order":         {Lines: []LineInput{{UserID: b.ID, InBase: true}, {UserID: a.ID, InBase: true}}},
		"an extra hour":     {ExtraHours: &one, Lines: []LineInput{{UserID: a.ID, InBase: true, InExtra: true}, {UserID: b.ID, InBase: true}}},
		"the court rate":    {BaseRateCents: &rate, Lines: base.Lines},
		"the hours":         {BaseHours: &hours, Lines: base.Lines},
		"shuttles per hour": {ShuttlesPerHour: &perHour, Lines: base.Lines},
	}
	for name, in := range inputs {
		t.Run(name, func(t *testing.T) {
			in.SessionID, in.SettledBy = f.session.ID, f.admin.ID
			if fingerprint(t, in) == reviewed {
				t.Fatal("the fingerprint did not change")
			}
		})
	}

	t.Run("club settings", func(t *testing.T) {
		if err := database.DB.Model(&models.Club{}).Where("1 = 1").Update("extra_rate_cents", 9900).Error; err != nil {
			t.Fatal(err)
		}
		// The extra rate prices nothing tonight, but it is still a rate this
		// settlement would snapshot.
		if fingerprint(t, base) == reviewed {
			t.Fatal("the fingerprint did not change")
		}
	})

	t.Run("stock at the same unit price", func(t *testing.T) {
		before := fingerprint(t, base)
		if _, err := f.ledger.RecordShuttlePurchase(AssetPurchaseInput{
			AmountCents: 10000, Units: 24, Description: "same again", CreatedBy: f.admin.ID,
		}); err != nil {
			t.Fatal(err)
		}
		if fingerprint(t, base) == before {
			t.Fatal("the fingerprint did not change")
		}
	})
}

// One review, confirmed eight times at once, is one settlement.
func TestOneReviewedPreviewSettlesExactlyOnceUnderConcurrency(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")
	input := SettleInput{SessionID: f.session.ID, SettledBy: f.admin.ID,
		Lines: []LineInput{{UserID: player.ID, InBase: true}}}
	reviewed, err := f.settlement.Preview(input)
	if err != nil {
		t.Fatal(err)
	}
	input.ExpectedPreview = reviewed.Fingerprint

	const attempts = 8
	errs := make([]error, attempts)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, _, errs[i] = f.settlement.Settle(input)
		}()
	}
	close(start)
	wg.Wait()

	var settled int
	for _, err := range errs {
		if err == nil {
			settled++
			continue
		}
		expectLedgerCode(t, err, CodeSessionAlreadySettled, 409)
	}
	if settled != 1 || liveSettlements(t) != 1 {
		t.Fatalf("%d attempts succeeded leaving %d settlements, want 1 and 1", settled, liveSettlements(t))
	}
	if got := playerBalance(t, f.ledger, player.ID); got != -reviewed.Totals.ChargedCents {
		t.Fatalf("player balance = %d, want %d", got, -reviewed.Totals.ChargedCents)
	}
	assertBalanced(t)
}

func playerBalance(t *testing.T, ledger *LedgerService, userID uuid.UUID) int64 {
	t.Helper()
	balance, err := ledger.BalanceOfUser(userID)
	if err != nil {
		t.Fatal(err)
	}
	return balance
}
