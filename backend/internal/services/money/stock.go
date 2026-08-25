package money

import (
	"errors"
	"fmt"
)

// InsufficientStockError reports that a session would consume more shuttles
// than the club holds. It carries both numbers so the API can tell the admin
// exactly how short they are and offer to record the missing purchase, rather
// than failing with a bare error.
type InsufficientStockError struct {
	RequiredUnits  int
	AvailableUnits int
}

func (e *InsufficientStockError) Error() string {
	return fmt.Sprintf("money: need %d shuttles, stock holds %d", e.RequiredUnits, e.AvailableUnits)
}

// ErrNegativeUnits guards against a caller computing a negative shuttle count.
var ErrNegativeUnits = errors.New("money: shuttle count cannot be negative")

// Stock is the shuttle position: what the club paid for what it still holds.
//
// Both halves are authoritative and both are derived from ledger entries. The
// cost of a single shuttle is never stored, because it usually cannot be: a $50
// tube of twelve is 416.666... cents each, and any rounded version of that
// number leaks value every time it is multiplied back out.
type Stock struct {
	ValueCents int64
	Units      int
}

// UnitCostCents reports the blended cost of one shuttle, rounded, for display
// only. Never use this for a calculation — use Consume, which works from the
// unrounded pair.
func (s Stock) UnitCostCents() int64 {
	if s.Units <= 0 {
		return 0
	}
	return roundedDiv(s.ValueCents, int64(s.Units))
}

// Consume values a number of shuttles taken from stock and reports what remains.
//
// The value comes from the whole stock position rather than a per-unit price,
// so buying tubes at different prices blends automatically and the remaining
// value stays exactly what was paid less what was used. Consuming all of the
// stock returns all of its value, with nothing stranded by rounding.
//
// Returns an *InsufficientStockError when stock is short. Settlement checks
// this before it posts anything, which is what keeps stock from ever going
// negative — and that in turn is why no fallback price is needed anywhere: the
// unit count at the moment of consumption is always greater than zero.
func (s Stock) Consume(units int) (consumedCents int64, remaining Stock, err error) {
	if units < 0 {
		return 0, s, ErrNegativeUnits
	}
	if units == 0 {
		return 0, s, nil
	}
	if units > s.Units {
		return 0, s, &InsufficientStockError{RequiredUnits: units, AvailableUnits: s.Units}
	}

	consumedCents = roundedDiv(int64(units)*s.ValueCents, int64(s.Units))

	// Taking every shuttle must take every cent with it; rounding must not
	// strand value in an empty bag.
	if units == s.Units {
		consumedCents = s.ValueCents
	}

	return consumedCents, Stock{
		ValueCents: s.ValueCents - consumedCents,
		Units:      s.Units - units,
	}, nil
}

// Add records a purchase. The blend is implicit: value and units both rise, and
// the next Consume derives its cost from the new pair. Stock bought earlier at
// a different price is never revalued.
func (s Stock) Add(units int, costCents int64) Stock {
	return Stock{ValueCents: s.ValueCents + costCents, Units: s.Units + units}
}

// roundedDiv divides two non-negative integers, rounding halves away from zero,
// without ever converting to a float.
func roundedDiv(num, den int64) int64 {
	if den == 0 {
		return 0
	}
	return (2*num + den) / (2 * den)
}
