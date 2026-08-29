package models

import "testing"

// These helpers carry no database and no dependencies, but the club-position
// identity is defined in terms of the first one and the settlement figures on
// screen come from the rest — so they are worth pinning down exactly.

func TestAccountKindIsAsset(t *testing.T) {
	assets := []AccountKind{AccountKindBank, AccountKindCourtCredit, AccountKindShuttleStock}
	for _, kind := range assets {
		if !kind.IsAsset() {
			t.Errorf("%s should be an asset — it is something the club holds", kind)
		}
	}

	// A player balance is what the club owes, and surplus is the balancing
	// figure. Counting either as an asset would make the identity come to
	// something other than zero.
	for _, kind := range []AccountKind{AccountKindPlayer, AccountKindSurplus} {
		if kind.IsAsset() {
			t.Errorf("%s should not be an asset", kind)
		}
	}
}

func TestClubAccountKindsAreAllNonPlayer(t *testing.T) {
	for _, kind := range ClubAccountKinds {
		if kind == AccountKindPlayer {
			t.Error("the club account kinds should not include the player kind")
		}
	}
}

func TestSettlementIsLive(t *testing.T) {
	live := &Settlement{}
	if !live.IsLive() {
		t.Error("a settlement that has not been reversed should be live")
	}

	reversed := &Settlement{}
	when := reversed.SettledAt
	reversed.ReversedAt = &when
	if reversed.IsLive() {
		t.Error("a reversed settlement should not be live")
	}
}

// The two bands are costed separately and at different rates: the standard hours
// at the peak rate, any extension at the cheaper off-peak one.
func TestSettlementCourtCents(t *testing.T) {
	s := &Settlement{
		BaseHours: 2, BaseRateCents: 3000,
		ExtraHours: 1, ExtraRateCents: 2300,
	}
	if got := s.CourtCents(); got != 8300 {
		t.Errorf("CourtCents = %d, want 8300 (2×$30 + 1×$23)", got)
	}

	noExtension := &Settlement{BaseHours: 2, BaseRateCents: 3000, ExtraRateCents: 2300}
	if got := noExtension.CourtCents(); got != 6000 {
		t.Errorf("CourtCents without an extension = %d, want 6000", got)
	}
}

func TestSettlementShuttleCents(t *testing.T) {
	s := &Settlement{BaseShuttleCents: 4166, ExtraShuttleCents: 2083}
	if got := s.ShuttleCents(); got != 6249 {
		t.Errorf("ShuttleCents = %d, want 6249", got)
	}
	if got := (&Settlement{}).ShuttleCents(); got != 0 {
		t.Errorf("ShuttleCents with an empty bag = %d, want 0", got)
	}
}

// A guest is charged to the member who brought them, so the line carries a name
// instead of standing on its own.
func TestChargeLineIsGuest(t *testing.T) {
	if !(&ChargeLine{GuestName: "Sam"}).IsGuest() {
		t.Error("a line with a guest name should read as a guest")
	}
	if (&ChargeLine{}).IsGuest() {
		t.Error("a line without a guest name should be the member themselves")
	}
}
