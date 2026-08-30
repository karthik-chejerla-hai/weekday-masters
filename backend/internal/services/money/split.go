package money

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"sort"

	"github.com/google/uuid"
)

// ErrNoParticipants is returned when a cost is split across nobody. Callers
// should decide what an empty band means rather than have this package invent a
// zero for them.
var ErrNoParticipants = errors.New("money: cannot split across zero participants")

// ErrNegativeTotal is returned when asked to split a negative amount. Charges
// are always non-negative; a negative total means a caller computed something
// wrong upstream, and silently splitting it would bury the bug.
var ErrNegativeTotal = errors.New("money: cannot split a negative total")

// Share is one participant's part of a split.
type Share struct {
	ParticipantID uuid.UUID
	AmountCents   int64
}

// SplitLargestRemainder divides total across participants so that the shares
// sum to exactly total — never a cent more or less.
//
// Dividing $101.67 six ways gives 1694.5 cents each, which does not exist. The
// naive fixes both fail: rounding every share up invents money, rounding every
// share down loses it. Largest remainder instead gives everyone the floor and
// hands the leftover cents out one at a time, so six people pay 1695, 1695,
// 1695, 1694, 1694, 1694 and the total is preserved.
//
// Who gets the extra cent is decided by hashing the seed together with each
// participant's id. Ordering by participant id alone would be equally
// deterministic, but the same person would pay the extra cent every single
// week. Seeding with the settlement id keeps it reproducible — the same
// settlement always produces the same allocation, which is what makes it
// testable — while varying who is unlucky from one session to the next.
//
// Returned shares are in allocation order, so the first entries are the ones
// carrying the extra cent.
func SplitLargestRemainder(total int64, participants []uuid.UUID, seed uuid.UUID) ([]Share, error) {
	n := int64(len(participants))
	if n == 0 {
		return nil, ErrNoParticipants
	}
	if total < 0 {
		return nil, ErrNegativeTotal
	}

	base := total / n
	remainder := total - base*n

	ordered := allocationOrder(participants, seed)

	shares := make([]Share, 0, len(ordered))
	for i, id := range ordered {
		amount := base
		if int64(i) < remainder {
			amount++
		}
		shares = append(shares, Share{ParticipantID: id, AmountCents: amount})
	}
	return shares, nil
}

// allocationOrder sorts participants by sha256(seed || participantID), giving a
// per-seed shuffle that is stable for a given seed and independent of the order
// the caller happened to supply.
func allocationOrder(participants []uuid.UUID, seed uuid.UUID) []uuid.UUID {
	type keyed struct {
		id  uuid.UUID
		key [sha256.Size]byte
	}

	keys := make([]keyed, len(participants))
	for i, id := range participants {
		h := sha256.New()
		h.Write(seed[:])
		h.Write(id[:])
		var sum [sha256.Size]byte
		copy(sum[:], h.Sum(nil))
		keys[i] = keyed{id: id, key: sum}
	}

	sort.Slice(keys, func(i, j int) bool {
		if c := bytes.Compare(keys[i].key[:], keys[j].key[:]); c != 0 {
			return c < 0
		}
		// Identical hashes are not reachable in practice, but ordering must be
		// total for sort.Slice to be deterministic.
		return bytes.Compare(keys[i].id[:], keys[j].id[:]) < 0
	})

	out := make([]uuid.UUID, len(keys))
	for i, k := range keys {
		out[i] = k.id
	}
	return out
}

// SumShares totals an allocation. Callers assert this equals the amount they
// asked to split; the invariant is cheap and the failure mode is money.
func SumShares(shares []Share) int64 {
	var total int64
	for _, s := range shares {
		total += s.AmountCents
	}
	return total
}
