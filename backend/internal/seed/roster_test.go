package seed

import (
	"os"
	"path/filepath"
	"testing"
)

// These need no database: the parser is pure, which is the point of keeping it
// in its own file away from the seeding.

// A miniature export in the real one's shape: header, blank line, transactions,
// then the restated totals.
const goodExport = `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone,Jo Patel,Sam Reilly (removed)

2026-01-05,Court bookings,General,120.00,AUD,-120.00,120.00,0.00,0.00
2026-01-12,Game - 12 Jan,General,90.00,AUD,90.00,-30.00,-30.00,-30.00
2026-01-19,Top-Up,General,50.00,AUD,-50.00,0.00,50.00,0.00
2026-01-26,Sam paid Badminton A.,Payment,30.00,AUD,-30.00,0.00,0.00,30.00

2026-01-26,Total balance, , ,AUD,-110.00,90.00,20.00,0.00
`

func writeExport(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "export.csv")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRosterReadsBalancesAndRemovals(t *testing.T) {
	roster, err := LoadRoster(writeExport(t, goodExport))
	if err != nil {
		t.Fatalf("LoadRoster: %v", err)
	}

	if roster.TransactionCount != 4 {
		t.Errorf("read %d transactions, want 4", roster.TransactionCount)
	}
	if len(roster.Members) != 3 {
		t.Fatalf("found %d members, want 3 — the club account is not one", len(roster.Members))
	}

	want := map[string]struct {
		cents   int64
		removed bool
	}{
		"Alex Stone": {9000, false},
		"Jo Patel":   {2000, false},
		"Sam Reilly": {0, true},
	}
	for _, member := range roster.Members {
		expected, ok := want[member.Name]
		if !ok {
			t.Errorf("unexpected member %q", member.Name)
			continue
		}
		if member.BalanceCents != expected.cents {
			t.Errorf("%s balance = %d, want %d", member.Name, member.BalanceCents, expected.cents)
		}
		if member.Removed != expected.removed {
			t.Errorf("%s removed = %t, want %t", member.Name, member.Removed, expected.removed)
		}
	}

	if roster.MemberCreditCents() != 11000 {
		t.Errorf("member credit = %d, want 11000", roster.MemberCreditCents())
	}
	if roster.ClubMirrorCents != -11000 {
		t.Errorf("club mirror = %d, want -11000", roster.ClubMirrorCents)
	}
}

// A fixture that silently disagrees with the spreadsheet is worse than none, so
// each way the file can lie has to be caught.

func TestLoadRosterRejectsATransactionThatDoesNotBalance(t *testing.T) {
	broken := `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone

2026-01-12,Game,General,30.00,AUD,30.00,-25.00

2026-01-12,Total balance, , ,AUD,30.00,-25.00
`
	_, err := LoadRoster(writeExport(t, broken))
	if err == nil {
		t.Fatal("a transaction summing to 5.00 was accepted")
	}
}

func TestLoadRosterRejectsTotalsThatDoNotMatch(t *testing.T) {
	broken := `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone

2026-01-12,Game,General,30.00,AUD,-30.00,30.00

2026-01-12,Total balance, , ,AUD,-30.00,99.00
`
	_, err := LoadRoster(writeExport(t, broken))
	if err == nil {
		t.Fatal("a stated total that disagrees with the transactions was accepted")
	}
}

func TestLoadRosterRequiresATotalsRow(t *testing.T) {
	missing := `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone

2026-01-12,Game,General,30.00,AUD,-30.00,30.00
`
	_, err := LoadRoster(writeExport(t, missing))
	if err == nil {
		t.Fatal("an export with nothing to check against was accepted")
	}
}

func TestLoadRosterRejectsForeignOrMixedCurrency(t *testing.T) {
	for name, broken := range map[string]string{
		"all USD": `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone

2026-01-12,Game,General,30.00,USD,-30.00,30.00

2026-01-12,Total balance, , ,USD,-30.00,30.00
`,
		"mixed": `Date,Description,Category,Cost,Currency,Badminton Account,Alex Stone

2026-01-05,Top-Up,General,30.00,AUD,-30.00,30.00
2026-01-12,Game,General,30.00,USD,30.00,-30.00

2026-01-12,Total balance, , ,AUD,0.00,0.00
`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadRoster(writeExport(t, broken)); err == nil {
				t.Fatal("a non-AUD export was accepted")
			}
		})
	}
}

func TestLoadRosterRejectsCollidingFixtureIdentities(t *testing.T) {
	broken := `Date,Description,Category,Cost,Currency,Badminton Account,Anne Marie,Anne-Marie

2026-01-12,Top-Up,General,30.00,AUD,-30.00,15.00,15.00

2026-01-12,Total balance, , ,AUD,-30.00,15.00,15.00
`
	if _, err := LoadRoster(writeExport(t, broken)); err == nil {
		t.Fatal("two members with the same fixture identity were accepted")
	}
}

func TestLoadRosterRejectsAMissingFile(t *testing.T) {
	if _, err := LoadRoster(filepath.Join(t.TempDir(), "nope.csv")); err == nil {
		t.Fatal("a missing export was accepted")
	}
}

// Money must never round-trip through a float, so the parser is exact or it
// refuses.
func TestParseCentsIsExact(t *testing.T) {
	cases := map[string]int64{
		"0.00": 0, "0.01": 1, "12.34": 1234, "-637.40": -63740,
		"275.78": 27578, "-0.62": -62, "1000.00": 100000, "": 0,
	}
	for in, want := range cases {
		got, err := parseCents(in)
		if err != nil {
			t.Errorf("parseCents(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseCents(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseCentsRejectsAmbiguousAmounts(t *testing.T) {
	for _, in := range []string{"12", "12.3", "12.345", "1,234.00", "abc", "12.3x"} {
		if _, err := parseCents(in); err == nil {
			t.Errorf("parseCents(%q) was accepted; a guess here is a wrong balance", in)
		}
	}
}

func TestSlugMakesUsableAddresses(t *testing.T) {
	cases := map[string]string{
		"Karthik Chejerla":  "karthik-chejerla",
		"SrinivasAddagatla": "srinivasaddagatla",
		"Hari Prasad":       "hari-prasad",
		"Ram":               "ram",
	}
	for name, want := range cases {
		if got := slug(name); got != want {
			t.Errorf("slug(%q) = %q, want %q", name, got, want)
		}
	}
}
