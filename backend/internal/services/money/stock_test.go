package money

import (
	"errors"
	"testing"
)

// A $50 tube of twelve is 416.66... cents a shuttle — a number that cannot be
// stored. What matters is that consuming from it stays exact.
func TestConsumeTubeOfTwelve(t *testing.T) {
	stock := Stock{ValueCents: 5000, Units: 12}

	if got := stock.UnitCostCents(); got != 417 {
		t.Errorf("display unit cost = %d, want 417", got)
	}

	consumed, remaining, err := stock.Consume(12)
	if err != nil {
		t.Fatal(err)
	}
	if consumed != 5000 {
		t.Errorf("consuming the whole tube cost %d, want the full 5000", consumed)
	}
	if remaining.ValueCents != 0 || remaining.Units != 0 {
		t.Errorf("remaining = %+v, want an empty bag with no stranded value", remaining)
	}
}

// The worked example from data-model.md, both bands in sequence.
func TestConsumeWorkedExample(t *testing.T) {
	stock := Stock{ValueCents: 10000, Units: 24} // two $50 tubes

	baseCost, afterBase, err := stock.Consume(10)
	if err != nil {
		t.Fatal(err)
	}
	if baseCost != 4167 {
		t.Errorf("base band shuttles = %d, want 4167", baseCost)
	}
	if afterBase.ValueCents != 5833 || afterBase.Units != 14 {
		t.Errorf("after base = %+v, want {5833 14}", afterBase)
	}

	extraCost, afterExtra, err := afterBase.Consume(5)
	if err != nil {
		t.Fatal(err)
	}
	if extraCost != 2083 {
		t.Errorf("extra band shuttles = %d, want 2083", extraCost)
	}
	if afterExtra.ValueCents != 3750 || afterExtra.Units != 9 {
		t.Errorf("after extra = %+v, want {3750 9}", afterExtra)
	}

	// Consumption across the night must equal what left the stock account.
	if baseCost+extraCost != 10000-afterExtra.ValueCents {
		t.Errorf("consumed %d but stock fell by %d", baseCost+extraCost, 10000-afterExtra.ValueCents)
	}

	// And nothing bought or sold in between means the blended rate is unchanged.
	if got := afterExtra.UnitCostCents(); got != 417 {
		t.Errorf("unit cost drifted to %d; want 417", got)
	}
}

// Buying at a new price blends; it must never revalue what is already in the bag.
func TestAddBlendsWithoutRevaluing(t *testing.T) {
	stock := Stock{ValueCents: 3750, Units: 9} // 416.67 each
	blended := stock.Add(12, 6000)             // a dearer tube at 500 each

	if blended.Units != 21 {
		t.Errorf("units = %d, want 21", blended.Units)
	}
	if blended.ValueCents != 9750 {
		t.Errorf("value = %d, want 9750 — the old value untouched plus the new purchase", blended.ValueCents)
	}
	if got := blended.UnitCostCents(); got != 464 {
		t.Errorf("blended unit cost = %d, want 464", got)
	}
}

// Consuming everything must never strand a cent, at any awkward ratio.
func TestConsumeAllNeverStrandsValue(t *testing.T) {
	cases := []Stock{
		{ValueCents: 5000, Units: 12},
		{ValueCents: 10000, Units: 24},
		{ValueCents: 3750, Units: 9},
		{ValueCents: 1, Units: 7},
		{ValueCents: 99999, Units: 13},
	}
	for _, stock := range cases {
		consumed, remaining, err := stock.Consume(stock.Units)
		if err != nil {
			t.Fatalf("%+v: %v", stock, err)
		}
		if consumed != stock.ValueCents {
			t.Errorf("%+v: consumed %d, want the full %d", stock, consumed, stock.ValueCents)
		}
		if remaining.ValueCents != 0 {
			t.Errorf("%+v: %d cents stranded in an empty bag", stock, remaining.ValueCents)
		}
	}
}

// Partial consumption must never take more value than exists, or leave a
// negative remainder, however the numbers divide.
func TestConsumePartialStaysWithinStock(t *testing.T) {
	stock := Stock{ValueCents: 10000, Units: 24}
	for units := 0; units <= 24; units++ {
		consumed, remaining, err := stock.Consume(units)
		if err != nil {
			t.Fatalf("units=%d: %v", units, err)
		}
		if consumed < 0 || consumed > stock.ValueCents {
			t.Errorf("units=%d: consumed %d, outside 0..%d", units, consumed, stock.ValueCents)
		}
		if remaining.ValueCents < 0 || remaining.Units < 0 {
			t.Errorf("units=%d: remaining %+v went negative", units, remaining)
		}
		if consumed+remaining.ValueCents != stock.ValueCents {
			t.Errorf("units=%d: %d + %d != %d", units, consumed, remaining.ValueCents, stock.ValueCents)
		}
	}
}

// Short stock is a domain condition, not a crash: it carries both numbers so
// the admin can be told exactly what to buy.
func TestConsumeReportsShortfall(t *testing.T) {
	stock := Stock{ValueCents: 1250, Units: 3}

	_, _, err := stock.Consume(10)
	var short *InsufficientStockError
	if !errors.As(err, &short) {
		t.Fatalf("got %v, want *InsufficientStockError", err)
	}
	if short.RequiredUnits != 10 || short.AvailableUnits != 3 {
		t.Errorf("got required=%d available=%d, want 10 and 3", short.RequiredUnits, short.AvailableUnits)
	}
}

// An empty bag cannot supply anything, and must not divide by zero trying.
func TestConsumeFromEmptyStock(t *testing.T) {
	empty := Stock{}

	if got := empty.UnitCostCents(); got != 0 {
		t.Errorf("unit cost of nothing = %d, want 0", got)
	}

	consumed, _, err := empty.Consume(0)
	if err != nil || consumed != 0 {
		t.Errorf("consuming nothing from nothing: got %d, %v; want 0, nil", consumed, err)
	}

	var short *InsufficientStockError
	if _, _, err := empty.Consume(1); !errors.As(err, &short) {
		t.Errorf("got %v, want *InsufficientStockError", err)
	}
}

func TestConsumeRejectsNegativeUnits(t *testing.T) {
	stock := Stock{ValueCents: 5000, Units: 12}
	if _, _, err := stock.Consume(-1); !errors.Is(err, ErrNegativeUnits) {
		t.Errorf("got %v, want ErrNegativeUnits", err)
	}
}
