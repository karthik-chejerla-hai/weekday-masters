package services

import (
	"errors"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/database"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/utils"
)

// settlementFixture sets the club up the way it actually plays: one court, a
// standing two-hour Tuesday booking, and a bag holding two $50 tubes.
type settlementFixture struct {
	ledger     *LedgerService
	settlement *SettlementService
	sessions   *SessionService
	admin      models.User
	session    *models.Session
}

func newSettlementFixture(t *testing.T, stockUnits int, stockCents int64) *settlementFixture {
	t.Helper()
	requireDB(t)

	_, sessionService, _ := newTestServices(t)
	ledger := NewLedgerService()
	admin := newUser(t, "admin")

	session, err := sessionService.CreateSession(CreateSessionInput{
		Title:       "Tuesday Social",
		SessionDate: utils.NowInSydney().AddDate(0, 0, -1),
		StartTime:   "20:00",
		EndTime:     "22:00",
		Courts:      1,
		CreatedBy:   admin.ID,
	})
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if stockUnits > 0 {
		if _, err := ledger.RecordShuttlePurchase(AssetPurchaseInput{
			AmountCents: stockCents, Units: stockUnits, Description: "tubes", CreatedBy: admin.ID,
		}); err != nil {
			t.Fatalf("failed to stock shuttles: %v", err)
		}
	}

	// Enough court credit that settlement is never the thing that runs it dry.
	if _, err := ledger.RecordCourtCreditPurchase(AssetPurchaseInput{
		AmountCents: 50000, Description: "venue top-up", CreatedBy: admin.ID,
	}); err != nil {
		t.Fatalf("failed to buy court credit: %v", err)
	}

	return &settlementFixture{
		ledger:     ledger,
		settlement: NewSettlementService(ledger),
		sessions:   sessionService,
		admin:      admin,
		session:    session,
	}
}

func (f *settlementFixture) member(t *testing.T, label string) models.User {
	t.Helper()
	user := newUser(t, label)
	if _, err := f.ledger.EnsurePlayerAccount(user.ID, user.Name); err != nil {
		t.Fatalf("failed to create account for %s: %v", label, err)
	}
	return user
}

func ptrFloat(v float64) *float64 { return &v }

// The worked example from data-model.md, end to end.
//
// Six heads on the base band — five members plus a guest hosted by the admin —
// and four staying for the extra hour. Stock holds 24 shuttles bought for $100.
func TestSettleWorkedExample(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)

	members := make([]models.User, 5)
	for i := range members {
		members[i] = f.member(t, "player")
	}

	// members[0..2] plus the admin's guest stay for the extra hour.
	lines := []LineInput{
		{UserID: members[0].ID, InBase: true, InExtra: true},
		{UserID: members[1].ID, InBase: true, InExtra: true},
		{UserID: members[2].ID, InBase: true, InExtra: true},
		{UserID: members[3].ID, InBase: true, InExtra: false},
		{UserID: members[4].ID, InBase: true, InExtra: false},
		{UserID: f.admin.ID, GuestName: "Sanjay", InBase: true, InExtra: true},
	}

	_, preview, err := f.settlement.Settle(SettleInput{
		SessionID:  f.session.ID,
		ExtraHours: ptrFloat(1),
		Lines:      lines,
		SettledBy:  f.admin.ID,
	})
	if err != nil {
		t.Fatalf("settle failed: %v", err)
	}

	base := preview.Bands["base"]
	if base.CourtCents != 6000 {
		t.Errorf("base court = %d, want 6000 (2h at $30)", base.CourtCents)
	}
	if base.ShuttleUnits != 10 || base.ShuttleCents != 4167 {
		t.Errorf("base shuttles = %d units / %d cents, want 10 / 4167", base.ShuttleUnits, base.ShuttleCents)
	}
	if base.TotalCents != 10167 {
		t.Errorf("base band total = %d, want 10167", base.TotalCents)
	}
	if base.Heads != 6 {
		t.Errorf("base heads = %d, want 6 — the guest counts", base.Heads)
	}

	extra := preview.Bands["extra"]
	if extra.CourtCents != 2300 {
		t.Errorf("extra court = %d, want 2300 (1h at $23)", extra.CourtCents)
	}
	if extra.ShuttleUnits != 5 || extra.ShuttleCents != 2083 {
		t.Errorf("extra shuttles = %d units / %d cents, want 5 / 2083", extra.ShuttleUnits, extra.ShuttleCents)
	}
	if extra.TotalCents != 4383 {
		t.Errorf("extra band total = %d, want 4383", extra.TotalCents)
	}
	if extra.Heads != 4 {
		t.Errorf("extra heads = %d, want 4", extra.Heads)
	}

	if preview.Totals.ChargedCents != 14550 {
		t.Errorf("charged = %d, want 14550", preview.Totals.ChargedCents)
	}
	if got := preview.Totals.CourtCents + preview.Totals.ShuttleCents; got != preview.Totals.ChargedCents {
		t.Errorf("charged %d but consumed %d — the split must be exact",
			preview.Totals.ChargedCents, got)
	}
	if preview.StockAfter.Units != 9 || preview.StockAfter.AmountCents != 3750 {
		t.Errorf("stock after = %+v, want {9 3750}", preview.StockAfter)
	}

	assertBalanced(t)
}

// The band split is the point of the whole design: someone who went home after
// the standard hours pays nothing toward the extension, and the people who
// stayed cover it between them.
func TestSettleEarlyLeaverDoesNotSubsidiseTheExtraHour(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)

	stayed := f.member(t, "stayed")
	wentHome := f.member(t, "wenthome")

	_, preview, err := f.settlement.Settle(SettleInput{
		SessionID:  f.session.ID,
		ExtraHours: ptrFloat(1),
		Lines: []LineInput{
			{UserID: stayed.ID, InBase: true, InExtra: true},
			{UserID: wentHome.ID, InBase: true, InExtra: false},
		},
		SettledBy: f.admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	byUser := map[uuid.UUID]int64{}
	for _, line := range preview.Lines {
		byUser[line.UserID] = line.AmountCents
	}

	// The base band is an odd number of cents, so one of the two carries the
	// extra cent — what matters is that the early leaver pays a base share and
	// nothing more.
	baseTotal := preview.Bands["base"].TotalCents
	extraTotal := preview.Bands["extra"].TotalCents

	leaver := byUser[wentHome.ID]
	if leaver != baseTotal/2 && leaver != baseTotal/2+1 {
		t.Errorf("early leaver charged %d, want half the base band (%d or %d) and no more",
			leaver, baseTotal/2, baseTotal/2+1)
	}
	if got := byUser[stayed.ID]; got != baseTotal-leaver+extraTotal {
		t.Errorf("the player who stayed was charged %d, want their base share plus the whole extra hour (%d)",
			got, baseTotal-leaver+extraTotal)
	}
	if leaver+byUser[stayed.ID] != baseTotal+extraTotal {
		t.Errorf("charges sum to %d, want %d", leaver+byUser[stayed.ID], baseTotal+extraTotal)
	}

	assertBalanced(t)
}

// Comping is the club's decision, so the club bears it: the comped player still
// counts as a head, nobody else's charge moves, and the waived share lands on
// surplus.
func TestSettleCompedPlayerIsAbsorbedByTheClub(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)

	a := f.member(t, "alpha")
	b := f.member(t, "bravo")
	guest := f.member(t, "charlie")

	uncomped, err := f.settlement.Preview(SettleInput{
		SessionID: f.session.ID,
		Lines: []LineInput{
			{UserID: a.ID, InBase: true},
			{UserID: b.ID, InBase: true},
			{UserID: guest.ID, InBase: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	_, preview, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines: []LineInput{
			{UserID: a.ID, InBase: true},
			{UserID: b.ID, InBase: true},
			{UserID: guest.ID, InBase: true, Comped: true},
		},
		SettledBy: f.admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	charged := map[uuid.UUID]int64{}
	before := map[uuid.UUID]int64{}
	for _, line := range preview.Lines {
		charged[line.UserID] = line.AmountCents
	}
	for _, line := range uncomped.Lines {
		before[line.UserID] = line.AmountCents
	}

	if charged[a.ID] != before[a.ID] || charged[b.ID] != before[b.ID] {
		t.Errorf("comping changed someone else's charge: %d→%d and %d→%d",
			before[a.ID], charged[a.ID], before[b.ID], charged[b.ID])
	}
	if charged[guest.ID] != 0 {
		t.Errorf("comped player charged %d, want 0", charged[guest.ID])
	}

	surplus, _ := f.ledger.BalanceOfKind(nil, models.AccountKindSurplus)
	if surplus != -before[guest.ID] {
		t.Errorf("surplus = %d, want %d — the club absorbs the waived share",
			surplus, -before[guest.ID])
	}

	assertBalanced(t)
}

// A guest is a head in the split, and the charge lands on their host.
func TestSettleGuestChargesTheHost(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)

	host := f.member(t, "host")
	other := f.member(t, "other")

	_, preview, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines: []LineInput{
			{UserID: host.ID, InBase: true},
			{UserID: host.ID, GuestName: "Sanjay", InBase: true},
			{UserID: other.ID, InBase: true},
		},
		SettledBy: f.admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if preview.Bands["base"].Heads != 3 {
		t.Errorf("heads = %d, want 3 — a guest dilutes the cost like anyone else", preview.Bands["base"].Heads)
	}

	hostAccount, _ := f.ledger.PlayerAccountID(host.ID)
	otherAccount, _ := f.ledger.PlayerAccountID(other.ID)
	hostBalance, _ := f.ledger.BalanceOf(hostAccount)
	otherBalance, _ := f.ledger.BalanceOf(otherAccount)

	// The host pays roughly twice what a single player pays: their own share and
	// their guest's.
	if hostBalance >= otherBalance {
		t.Errorf("host balance %d should be further down than a single player's %d", hostBalance, otherBalance)
	}
	if hostBalance > otherBalance*2+1 || hostBalance < otherBalance*2-1 {
		t.Errorf("host charged %d, want about twice a single share (%d)", -hostBalance, -otherBalance*2)
	}

	// The guest is named in history without having an account.
	var lines []models.ChargeLine
	database.DB.Where("guest_name <> ''").Find(&lines)
	if len(lines) != 1 || lines[0].GuestName != "Sanjay" {
		t.Errorf("expected one guest line named Sanjay, got %+v", lines)
	}

	assertBalanced(t)
}

// Short stock is a domain condition with a fix attached, not a failure. Nothing
// may be written, or the admin would have to unpick a half-settled session.
func TestSettleRefusesWhenStockIsShort(t *testing.T) {
	f := newSettlementFixture(t, 3, 1250)
	player := f.member(t, "player")

	_, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	})

	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) || ledgerErr.Code != CodeShuttleStockShort {
		t.Fatalf("got %v, want %s", err, CodeShuttleStockShort)
	}
	if ledgerErr.Details["required_units"] != 10 || ledgerErr.Details["available_units"] != 3 {
		t.Errorf("details = %v, want required 10 and available 3", ledgerErr.Details)
	}

	var settlements int64
	database.DB.Model(&models.Settlement{}).Count(&settlements)
	if settlements != 0 {
		t.Errorf("%d settlements written despite the shortfall", settlements)
	}

	// Recording the missing purchase and retrying must then work.
	if _, err := f.ledger.RecordShuttlePurchase(AssetPurchaseInput{
		AmountCents: 5000, Units: 12, Description: "1 tube", CreatedBy: f.admin.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	}); err != nil {
		t.Fatalf("settle after restocking failed: %v", err)
	}

	assertBalanced(t)
}

// Settlement is a one-way door, with a way back through reversal.
func TestSettleRefusesASecondSettlement(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	in := SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	}
	settlement, _, err := f.settlement.Settle(in)
	if err != nil {
		t.Fatal(err)
	}

	_, _, err = f.settlement.Settle(in)
	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) || ledgerErr.Code != CodeSessionAlreadySettled {
		t.Fatalf("got %v, want %s", err, CodeSessionAlreadySettled)
	}

	// Reversing frees the session, and re-settling with a different list works.
	if _, err := f.settlement.ReverseSettlement(settlement.ID, "wrong player list", f.admin.ID); err != nil {
		t.Fatalf("reversal failed: %v", err)
	}

	account, _ := f.ledger.PlayerAccountID(player.ID)
	if balance, _ := f.ledger.BalanceOf(account); balance != 0 {
		t.Errorf("balance after reversal = %d, want 0", balance)
	}

	other := f.member(t, "other")
	if _, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines: []LineInput{
			{UserID: player.ID, InBase: true},
			{UserID: other.ID, InBase: true},
		},
		SettledBy: f.admin.ID,
	}); err != nil {
		t.Fatalf("re-settling after reversal failed: %v", err)
	}

	assertBalanced(t)
}

// Reversal must put the shuttles back in the bag as well as the money.
func TestReverseSettlementRestoresStock(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	settlement, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := f.settlement.ReverseSettlement(settlement.ID, "", f.admin.ID); err != nil {
		t.Fatal(err)
	}

	stock, _ := f.ledger.StockPosition(nil)
	if stock.Units != 24 || stock.ValueCents != 10000 {
		t.Errorf("stock after reversal = %+v, want the original {10000 24}", stock)
	}
	assertBalanced(t)
}

// Settled sessions must be immune to later rate changes.
func TestSettlementSnapshotsRates(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	settlement, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	var club models.Club
	if err := database.DB.First(&club).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.DB.Model(&club).
		Updates(map[string]any{"base_rate_cents": 9999, "shuttles_per_hour": 99}).Error; err != nil {
		t.Fatal(err)
	}

	var reloaded models.Settlement
	if err := database.DB.First(&reloaded, "id = ?", settlement.ID).Error; err != nil {
		t.Fatal(err)
	}
	if reloaded.BaseRateCents != 3000 {
		t.Errorf("settled rate reads %d after a club rate change, want the snapshotted 3000", reloaded.BaseRateCents)
	}
	if reloaded.ShuttlesPerHour != 5 {
		t.Errorf("settled shuttle rate reads %g, want the snapshotted 5", reloaded.ShuttlesPerHour)
	}
}

// A session with nobody on it is the admin covering the night, not an error.
func TestSettleWithNobodyChargedFallsToSurplus(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	_, preview, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true, Comped: true}},
		SettledBy: f.admin.ID,
	})
	if err != nil {
		t.Fatal(err)
	}

	if preview.Totals.ChargedCents != 0 {
		t.Errorf("charged = %d, want 0", preview.Totals.ChargedCents)
	}

	surplus, _ := f.ledger.BalanceOfKind(nil, models.AccountKindSurplus)
	if surplus != -preview.Bands["base"].TotalCents {
		t.Errorf("surplus = %d, want the whole band absorbed (%d)", surplus, -preview.Bands["base"].TotalCents)
	}
	assertBalanced(t)
}

// Settling must not touch the bank: that cash moved when members topped up and
// when the admin bought credit and shuttles.
func TestSettleDoesNotTouchTheBank(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	if _, err := f.ledger.RecordTopup(CashInput{
		UserID: player.ID, AmountCents: 5000, CreatedBy: f.admin.ID,
	}); err != nil {
		t.Fatal(err)
	}
	bankBefore, _ := f.ledger.BalanceOfKind(nil, models.AccountKindBank)

	if _, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	}); err != nil {
		t.Fatal(err)
	}

	bankAfter, _ := f.ledger.BalanceOfKind(nil, models.AccountKindBank)
	if bankAfter != bankBefore {
		t.Errorf("bank moved from %d to %d during settlement", bankBefore, bankAfter)
	}
	assertBalanced(t)
}

// The default participant list is everyone who said they were coming, no-shows
// included: the court was booked for them.
func TestSettleDefaultsToEveryoneWhoSaidTheyWereComing(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	rsvpService := NewRSVPService(&recordingNotifier{})

	coming := f.member(t, "coming")
	alsoComing := f.member(t, "alsocoming")
	notComing := f.member(t, "notcoming")

	// The fixture session is in the past, so the RSVP deadline has gone; record
	// these as the admin, which is how a late change gets in.
	for _, entry := range []struct {
		user   models.User
		status models.RSVPStatus
	}{
		{coming, models.RSVPStatusIn},
		{alsoComing, models.RSVPStatusIn},
		{notComing, models.RSVPStatusOut},
	} {
		if _, err := rsvpService.CreateOrUpdateRSVP(RSVPInput{
			SessionID: f.session.ID, UserID: entry.user.ID, Status: entry.status,
		}, true); err != nil {
			t.Fatal(err)
		}
	}

	preview, err := f.settlement.Preview(SettleInput{SessionID: f.session.ID})
	if err != nil {
		t.Fatal(err)
	}

	if preview.Bands["base"].Heads != 2 {
		t.Errorf("heads = %d, want 2 — only those who said they were coming", preview.Bands["base"].Heads)
	}
	for _, line := range preview.Lines {
		if line.UserID == notComing.ID {
			t.Error("a player who said they were not coming is on the settlement")
		}
	}
}

// Two admins pressing settle at the same moment must produce one settlement, not
// two sets of charges. Mirrors the existing RSVP capacity concurrency test.
func TestSettleIsSafeUnderConcurrency(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	const attempts = 8
	var wg sync.WaitGroup
	results := make([]error, attempts)

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, _, err := f.settlement.Settle(SettleInput{
				SessionID: f.session.ID,
				Lines:     []LineInput{{UserID: player.ID, InBase: true}},
				SettledBy: f.admin.ID,
			})
			results[idx] = err
		}(i)
	}
	wg.Wait()

	var succeeded int
	for _, err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Errorf("%d of %d concurrent settle attempts succeeded, want exactly 1", succeeded, attempts)
	}

	var settlements int64
	database.DB.Model(&models.Settlement{}).Where("reversed_at IS NULL").Count(&settlements)
	if settlements != 1 {
		t.Errorf("%d live settlements, want 1", settlements)
	}

	assertBalanced(t)
}

// A cancelled session has nothing to settle.
func TestSettleRefusesACancelledSession(t *testing.T) {
	f := newSettlementFixture(t, 24, 10000)
	player := f.member(t, "player")

	if err := database.DB.Model(&models.Session{}).
		Where("id = ?", f.session.ID).
		Update("status", models.SessionStatusCancelled).Error; err != nil {
		t.Fatal(err)
	}

	_, _, err := f.settlement.Settle(SettleInput{
		SessionID: f.session.ID,
		Lines:     []LineInput{{UserID: player.ID, InBase: true}},
		SettledBy: f.admin.ID,
	})

	var ledgerErr *LedgerError
	if !errors.As(err, &ledgerErr) || ledgerErr.Code != CodeNotSettleable {
		t.Fatalf("got %v, want %s", err, CodeNotSettleable)
	}
}

// Charges must sum to the cost exactly, at every awkward headcount.
func TestSettleSplitIsExactAtEveryHeadcount(t *testing.T) {
	for n := 1; n <= 7; n++ {
		f := newSettlementFixture(t, 24, 10000)

		lines := make([]LineInput, 0, n)
		for i := 0; i < n; i++ {
			lines = append(lines, LineInput{UserID: f.member(t, "p").ID, InBase: true, InExtra: true})
		}

		_, preview, err := f.settlement.Settle(SettleInput{
			SessionID:  f.session.ID,
			ExtraHours: ptrFloat(1),
			Lines:      lines,
			SettledBy:  f.admin.ID,
		})
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}

		consumed := preview.Totals.CourtCents + preview.Totals.ShuttleCents
		if preview.Totals.ChargedCents != consumed {
			t.Errorf("n=%d: charged %d but consumed %d", n, preview.Totals.ChargedCents, consumed)
		}

		var lineTotal int64
		for _, line := range preview.Lines {
			lineTotal += line.AmountCents
		}
		if lineTotal != consumed {
			t.Errorf("n=%d: lines sum to %d, want %d", n, lineTotal, consumed)
		}
	}
}
