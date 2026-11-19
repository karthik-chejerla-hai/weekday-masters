package money

import (
	"testing"

	"github.com/google/uuid"
)

func someIDs(n int) []uuid.UUID {
	out := make([]uuid.UUID, n)
	for i := range out {
		out[i] = uuid.New()
	}
	return out
}

func TestSplitByWeightsSumsToTotalAtEveryHeadcount(t *testing.T) {
	seed := uuid.New()
	for n := 1; n <= 30; n++ {
		people := someIDs(n)
		weights := make([]int, n)
		for i := range weights {
			weights[i] = 1 + i%4
		}
		for _, total := range []int64{0, 1, 99, 100, 8400, 10167, 999999} {
			shares, err := SplitByWeights(total, people, weights, seed)
			if err != nil {
				t.Fatalf("n=%d total=%d: %v", n, total, err)
			}
			if got := SumShares(shares); got != total {
				t.Fatalf("n=%d total=%d: shares sum to %d", n, total, got)
			}
		}
	}
}

func TestSplitByWeightsIsProportional(t *testing.T) {
	people := someIDs(2)
	shares, err := SplitByWeights(9000, people, []int{2, 1}, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	if shares[0].ParticipantID != people[0] || shares[0].AmountCents != 6000 || shares[1].AmountCents != 3000 {
		t.Fatalf("got %+v", shares)
	}
}

func TestSplitByWeightsStaysWithinACentOfExact(t *testing.T) {
	shares, err := SplitByWeights(100, someIDs(3), []int{1, 1, 1}, uuid.New())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range shares {
		if s.AmountCents != 33 && s.AmountCents != 34 {
			t.Fatalf("share %d is more than a cent from 33.33", s.AmountCents)
		}
	}
}

func TestSplitByWeightsIsDeterministic(t *testing.T) {
	people, seed := someIDs(7), uuid.New()
	w := []int{1, 2, 3, 1, 2, 3, 1}
	a, _ := SplitByWeights(1000, people, w, seed)
	b, _ := SplitByWeights(1000, people, w, seed)
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("same input gave a different allocation")
		}
	}
}

func TestSplitByWeightsRefusesBadInput(t *testing.T) {
	people := someIDs(2)
	for name, tc := range map[string]struct {
		total   int64
		people  []uuid.UUID
		weights []int
	}{
		"no participants": {100, nil, nil},
		"negative total":  {-1, people, []int{1, 1}},
		"length mismatch": {100, people, []int{1}},
		"zero weight":     {100, people, []int{1, 0}},
		"negative weight": {100, people, []int{1, -1}},
	} {
		if _, err := SplitByWeights(tc.total, tc.people, tc.weights, uuid.New()); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestCheckExact(t *testing.T) {
	if err := CheckExact(100, []int64{60, 40}); err != nil {
		t.Fatal(err)
	}
	for name, amounts := range map[string][]int64{
		"short": {60, 39}, "over": {60, 41}, "zero": {100, 0}, "negative": {110, -10}, "empty": {},
	} {
		if err := CheckExact(100, amounts); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
