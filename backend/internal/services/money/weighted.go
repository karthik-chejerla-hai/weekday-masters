package money

import (
	"errors"
	"sort"

	"github.com/google/uuid"
)

var (
	// ErrBadWeights covers a weight below one or a weight list that does not
	// line up with the participants.
	ErrBadWeights = errors.New("money: every participant needs a weight of at least one")
	// ErrExactMismatch means typed amounts do not add up to the bill.
	ErrExactMismatch = errors.New("money: exact amounts must be positive and sum to the total")
)

// SplitByWeights divides total in proportion to weights so the shares sum to
// exactly total. Everyone gets the floor of their proportional amount; the
// leftover cents go one each to the largest fractional remainders. Ties are
// broken by the same seeded order SplitLargestRemainder uses, so no one is
// systematically unlucky.
//
// Shares are returned in the order participants were supplied.
func SplitByWeights(total int64, participants []uuid.UUID, weights []int, seed uuid.UUID) ([]Share, error) {
	if len(participants) == 0 {
		return nil, ErrNoParticipants
	}
	if total < 0 {
		return nil, ErrNegativeTotal
	}
	if len(weights) != len(participants) {
		return nil, ErrBadWeights
	}
	var sum int64
	for _, w := range weights {
		if w < 1 {
			return nil, ErrBadWeights
		}
		sum += int64(w)
	}

	rank := make(map[uuid.UUID]int, len(participants))
	for i, id := range allocationOrder(participants, seed) {
		rank[id] = i
	}

	type part struct {
		index     int
		remainder int64
	}
	shares := make([]Share, len(participants))
	parts := make([]part, len(participants))
	var allocated int64
	for i, id := range participants {
		numerator := total * int64(weights[i])
		shares[i] = Share{ParticipantID: id, AmountCents: numerator / sum}
		parts[i] = part{index: i, remainder: numerator % sum}
		allocated += shares[i].AmountCents
	}
	sort.Slice(parts, func(a, b int) bool {
		if parts[a].remainder != parts[b].remainder {
			return parts[a].remainder > parts[b].remainder
		}
		return rank[participants[parts[a].index]] < rank[participants[parts[b].index]]
	})
	for i := int64(0); i < total-allocated; i++ {
		shares[parts[i].index].AmountCents++
	}
	return shares, nil
}

// CheckExact accepts typed amounts only when each is positive and they sum to
// the total. It never adjusts them: a bill that does not add up is the admin's
// to fix, not this package's to round.
func CheckExact(total int64, amounts []int64) error {
	if len(amounts) == 0 {
		return ErrNoParticipants
	}
	var sum int64
	for _, a := range amounts {
		if a <= 0 {
			return ErrExactMismatch
		}
		sum += a
	}
	if sum != total {
		return ErrExactMismatch
	}
	return nil
}
