package money

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func ids(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

// The whole point of largest remainder: whatever the total and however many
// people, not one cent is lost or invented.
func TestSplitSumsExactly(t *testing.T) {
	seed := uuid.New()
	totals := []int64{0, 1, 2, 7, 99, 100, 101, 4383, 10167, 14550, 999999, 1000000}

	for n := 1; n <= 20; n++ {
		participants := ids(n)
		for _, total := range totals {
			shares, err := SplitLargestRemainder(total, participants, seed)
			if err != nil {
				t.Fatalf("n=%d total=%d: unexpected error: %v", n, total, err)
			}
			if got := SumShares(shares); got != total {
				t.Errorf("n=%d total=%d: shares sum to %d, want %d", n, total, got, total)
			}
			if len(shares) != n {
				t.Errorf("n=%d: got %d shares, want %d", n, len(shares), n)
			}
		}
	}
}

// No participant may be more than one cent better off than another; the
// remainder is spread, not dumped on one person.
func TestSplitSharesDifferByAtMostOneCent(t *testing.T) {
	seed := uuid.New()
	participants := ids(7)

	shares, err := SplitLargestRemainder(10167, participants, seed)
	if err != nil {
		t.Fatal(err)
	}

	min, max := shares[0].AmountCents, shares[0].AmountCents
	for _, s := range shares {
		if s.AmountCents < min {
			min = s.AmountCents
		}
		if s.AmountCents > max {
			max = s.AmountCents
		}
	}
	if max-min > 1 {
		t.Errorf("spread of %d cents between highest and lowest share; want at most 1", max-min)
	}
}

// The worked example from data-model.md: $101.67 across six heads.
func TestSplitWorkedExampleBaseBand(t *testing.T) {
	shares, err := SplitLargestRemainder(10167, ids(6), uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	var at1695, at1694 int
	for _, s := range shares {
		switch s.AmountCents {
		case 1695:
			at1695++
		case 1694:
			at1694++
		default:
			t.Fatalf("unexpected share %d; want 1694 or 1695", s.AmountCents)
		}
	}
	if at1695 != 3 || at1694 != 3 {
		t.Errorf("got %d at 1695 and %d at 1694; want 3 and 3", at1695, at1694)
	}
}

// And the extra band: $43.83 across four.
func TestSplitWorkedExampleExtraBand(t *testing.T) {
	shares, err := SplitLargestRemainder(4383, ids(4), uuid.New())
	if err != nil {
		t.Fatal(err)
	}

	var at1096, at1095 int
	for _, s := range shares {
		switch s.AmountCents {
		case 1096:
			at1096++
		case 1095:
			at1095++
		default:
			t.Fatalf("unexpected share %d; want 1095 or 1096", s.AmountCents)
		}
	}
	if at1096 != 3 || at1095 != 1 {
		t.Errorf("got %d at 1096 and %d at 1095; want 3 and 1", at1096, at1095)
	}
}

// Same seed, same allocation — otherwise a settlement could not be re-derived
// from its inputs and the tests above would be meaningless.
func TestSplitIsDeterministicForASeed(t *testing.T) {
	seed := uuid.New()
	participants := ids(6)

	first, err := SplitLargestRemainder(10167, participants, seed)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		again, err := SplitLargestRemainder(10167, participants, seed)
		if err != nil {
			t.Fatal(err)
		}
		for j := range first {
			if first[j] != again[j] {
				t.Fatalf("allocation changed between runs at index %d", j)
			}
		}
	}
}

// Supplying participants in a different order must not change who pays what;
// the allocation depends on the seed and the ids, not on call order.
func TestSplitIndependentOfInputOrder(t *testing.T) {
	seed := uuid.New()
	participants := ids(6)

	forward, err := SplitLargestRemainder(10167, participants, seed)
	if err != nil {
		t.Fatal(err)
	}

	reversed := make([]uuid.UUID, len(participants))
	for i, id := range participants {
		reversed[len(participants)-1-i] = id
	}
	backward, err := SplitLargestRemainder(10167, reversed, seed)
	if err != nil {
		t.Fatal(err)
	}

	got := map[uuid.UUID]int64{}
	for _, s := range backward {
		got[s.ParticipantID] = s.AmountCents
	}
	for _, s := range forward {
		if got[s.ParticipantID] != s.AmountCents {
			t.Errorf("participant %s got %d one way and %d the other", s.ParticipantID, s.AmountCents, got[s.ParticipantID])
		}
	}
}

// Different settlements must not punish the same person every week. Across many
// seeds the extra cent should land on more than one participant.
func TestSplitRotatesTheOddCent(t *testing.T) {
	participants := ids(6)
	unlucky := map[uuid.UUID]int{}

	for i := 0; i < 200; i++ {
		shares, err := SplitLargestRemainder(10167, participants, uuid.New())
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range shares {
			if s.AmountCents == 1695 {
				unlucky[s.ParticipantID]++
			}
		}
	}

	if len(unlucky) < len(participants) {
		t.Errorf("only %d of %d participants ever paid the extra cent across 200 settlements",
			len(unlucky), len(participants))
	}
}

func TestSplitRejectsEmptyAndNegative(t *testing.T) {
	if _, err := SplitLargestRemainder(100, nil, uuid.New()); !errors.Is(err, ErrNoParticipants) {
		t.Errorf("splitting across nobody: got %v, want ErrNoParticipants", err)
	}
	if _, err := SplitLargestRemainder(-1, ids(3), uuid.New()); !errors.Is(err, ErrNegativeTotal) {
		t.Errorf("splitting a negative total: got %v, want ErrNegativeTotal", err)
	}
}

// One participant pays the lot — the single-player session.
func TestSplitSingleParticipantTakesEverything(t *testing.T) {
	shares, err := SplitLargestRemainder(10167, ids(1), uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if shares[0].AmountCents != 10167 {
		t.Errorf("got %d, want the whole 10167", shares[0].AmountCents)
	}
}
