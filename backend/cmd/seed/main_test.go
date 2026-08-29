package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
	"github.com/weekday-masters/backend/internal/models"
	"github.com/weekday-masters/backend/internal/services"
)

// The export is the only input to this tool that nobody checks by hand, and a
// misread balance would be posted as an opening balance — which the ledger
// accepts exactly once. So the parsing is what these tests cover.

func TestParseSplitwiseReadsTheClosingBalances(t *testing.T) {
	members, err := parseSplitwise(filepath.Join("testdata", "group.html"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	want := []member{
		{Name: "Karthik C.", BalanceCents: 12661},
		{Name: "Naresh K.", BalanceCents: -7122},
		{Name: "Krishna", BalanceCents: 0},
		{Name: "Big Spender", BalanceCents: 123456},
		{Name: clubAccountName, BalanceCents: -128995},
	}
	if len(members) != len(want) {
		t.Fatalf("got %d members, want %d: %+v", len(members), len(want), members)
	}
	for i, w := range want {
		if members[i] != w {
			t.Errorf("member %d = %+v, want %+v", i, members[i], w)
		}
	}
}

// The club account mirrors everyone else. If that identity does not hold, the
// export was misread and seeding it would put the ledger out by the difference.
func TestParsedExportBalancesAgainstTheClubAccount(t *testing.T) {
	members, err := parseSplitwise(filepath.Join("testdata", "group.html"))
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	var players, clubOwes int64
	for _, m := range members {
		if m.Name == clubAccountName {
			clubOwes = -m.BalanceCents
			continue
		}
		players += m.BalanceCents
	}
	if players != clubOwes {
		t.Errorf("players total %d but the club account says %d", players, clubOwes)
	}
}

func TestParseSplitwiseRejectsAnUnexpectedFile(t *testing.T) {
	dir := t.TempDir()
	notAnExport := filepath.Join(dir, "other.html")
	if err := os.WriteFile(notAnExport, []byte("<html><body>hello</body></html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := parseSplitwise(notAnExport); err == nil {
		t.Error("a file with no TOTAL BALANCE section parsed without complaint")
	}
	if _, err := parseSplitwise(filepath.Join(dir, "missing.html")); err == nil {
		t.Error("a missing file parsed without complaint")
	}
}

func TestParseCents(t *testing.T) {
	ok := map[string]int64{
		"0.00":     0,
		"5.84":     584,
		"126.61":   12661,
		"1,234.56": 123456,
		"0.05":     5,
	}
	for in, want := range ok {
		got, err := parseCents(in)
		if err != nil {
			t.Errorf("parseCents(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("parseCents(%q) = %d, want %d", in, got, want)
		}
	}

	// Anything that is not exactly two decimal places is a misread, not a number
	// to round — rounding here would silently move money.
	for _, bad := range []string{"12", "12.3", "12.345", "", "abc", "1.2a"} {
		if _, err := parseCents(bad); err == nil {
			t.Errorf("parseCents(%q) accepted a non-amount", bad)
		}
	}
}

func TestParseEmails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "emails.csv")
	body := "# a comment\n\nKarthik C.,karthik@example.com\n  Naresh K. , naresh@example.com  \n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	mapping, err := parseEmails(path)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}
	if len(mapping) != 2 {
		t.Fatalf("got %d entries, want 2: %+v", len(mapping), mapping)
	}
	if mapping["Karthik C."] != "karthik@example.com" || mapping["Naresh K."] != "naresh@example.com" {
		t.Errorf("mapping = %+v", mapping)
	}

	if mapping, err := parseEmails(""); mapping != nil || err != nil {
		t.Errorf("no path should mean no mapping, got %+v / %v", mapping, err)
	}
}

func TestParseEmailsRejectsMalformedLines(t *testing.T) {
	dir := t.TempDir()
	for name, body := range map[string]string{
		"no comma":     "Karthik C. karthik@example.com\n",
		"no address":   "Karthik C.,\n",
		"no name":      ",karthik@example.com\n",
		"not an email": "Karthik C.,karthik\n",
		"listed twice": "Karthik C.,a@example.com\nKarthik C.,b@example.com\n",
	} {
		path := filepath.Join(dir, slugify(name)+".csv")
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := parseEmails(path); err == nil {
			t.Errorf("%s: accepted %q", name, body)
		}
	}
}

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Karthik C.":        "karthik.c",
		"SrinivasAddagatla": "srinivasaddagatla",
		"Naresh K.":         "naresh.k",
		"Krishna":           "krishna",
		"  Odd   Name  ":    "odd.name",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatCents(t *testing.T) {
	cases := map[int64]string{
		0: "$0.00", 5: "$0.05", 584: "$5.84", -7122: "-$71.22", 123456: "$1234.56",
	}
	for in, want := range cases {
		if got := formatCents(in); got != want {
			t.Errorf("formatCents(%d) = %q, want %q", in, got, want)
		}
	}
}

// The connection string is printed so the operator can see which database they
// are about to write to. It must not carry the password to the terminal.
func TestRedactHidesThePassword(t *testing.T) {
	got := redact("postgres://badminton:s3cret@db.example.com:5432/club?sslmode=require")
	if got != "postgres://***@db.example.com:5432/club?sslmode=require" {
		t.Errorf("redact = %q", got)
	}
	if redact("not a url") != "not a url" {
		t.Error("redact mangled a string with no credentials")
	}
}

func TestBalanceOf(t *testing.T) {
	players := []member{{Name: "Karthik C.", BalanceCents: 12661}, {Name: "Krishna"}}
	if got := balanceOf(players, "Karthik C."); got != 12661 {
		t.Errorf("balanceOf = %d, want 12661", got)
	}
	if got := balanceOf(players, "Nobody"); got != 0 {
		t.Errorf("balanceOf for an unknown name = %d, want 0", got)
	}
}

func TestAdminOrFirst(t *testing.T) {
	admin := models.User{ID: uuid.New()}
	first := services.OpeningPlayerBalance{UserID: uuid.New()}

	if got := adminOrFirst(admin, []services.OpeningPlayerBalance{first}); got != admin.ID {
		t.Error("the admin should be attributed where one exists")
	}
	if got := adminOrFirst(models.User{}, []services.OpeningPlayerBalance{first}); got != first.UserID {
		t.Error("without an admin it should fall back to the first player")
	}
	if got := adminOrFirst(models.User{}, nil); got != uuid.Nil {
		t.Errorf("with nobody at all it should be the nil UUID, got %s", got)
	}
}
