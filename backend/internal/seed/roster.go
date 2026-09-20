package seed

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// This file recovers a club roll and its closing balances from a Splitwise CSV
// export — the shape of file the club kept its books in before this app.
//
// The export is a matrix: one row per transaction, one column per account, and
// every row sums to zero across the columns because Splitwise is double-entry
// underneath. The final row restates each column's running total. That gives
// two independent checks, and LoadRoster insists on both before returning
// anything: every transaction must sum to zero, and the column sums it computes
// must equal the totals the export states. A fixture built from a file that
// does not reconcile would be worse than no fixture, because the balances would
// look authoritative and be wrong.
//
// The export itself is never committed. It carries real members' names and
// financial positions, and it is read from a path given at run time. See
// cmd/seed's -from flag.

const (
	// clubColumn is Splitwise's account for the club pot. It mirrors the
	// members rather than being one of them, so it is not part of the roster.
	clubColumn = "Badminton Account"

	// totalRowLabel marks the restated-totals row at the end of the export.
	totalRowLabel = "Total balance"

	// removedSuffix is how the export marks someone who has left. The club's
	// convention, not Splitwise's.
	removedSuffix = " (removed)"

	// fixedColumns are the five columns before the per-account ones.
	fixedColumns = 5

	// Splitwise repeats the currency on every row. Rally has no currency field
	// or conversion layer, so accepting anything other than AUD would silently
	// reinterpret foreign-currency amounts as Australian dollars.
	currencyColumn   = 4
	requiredCurrency = "AUD"
)

// Roster is a club roll recovered from an export.
type Roster struct {
	Members []RosterMember

	// ClubMirrorCents is the export's own club-account total. It should be the
	// exact negation of the members' combined credit.
	ClubMirrorCents int64

	// TransactionCount is how many rows were checked to get here.
	TransactionCount int
}

// RosterMember is one person and what the club owes them, in cents.
type RosterMember struct {
	Name         string
	Removed      bool
	BalanceCents int64
}

// MemberCreditCents is the club's total liability to its members.
func (r *Roster) MemberCreditCents() int64 {
	var total int64
	for _, m := range r.Members {
		total += m.BalanceCents
	}
	return total
}

// LoadRoster reads an export and returns the roll, refusing anything that does
// not reconcile.
func LoadRoster(path string) (*Roster, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening the export: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	// The export has a blank line after the header and a short totals row, so
	// the field count is not constant.
	reader.FieldsPerRecord = -1

	rows, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading the export: %w", err)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("the export is empty")
	}

	header := rows[0]
	if len(header) <= fixedColumns {
		return nil, fmt.Errorf("the export has no account columns; is this a Splitwise export?")
	}
	if strings.TrimSpace(header[currencyColumn]) != "Currency" {
		return nil, fmt.Errorf("column %d is %q, expected the Splitwise Currency column",
			currencyColumn+1, header[currencyColumn])
	}
	accounts := header[fixedColumns:]

	var transactions [][]string
	var totalRow []string
	for _, row := range rows[1:] {
		if !hasContent(row) {
			continue
		}
		if currency := strings.TrimSpace(cell(row, currencyColumn)); currency != requiredCurrency {
			return nil, fmt.Errorf("row %q uses currency %q; Rally opening balances require %s",
				cell(row, 1), currency, requiredCurrency)
		}
		if len(row) > 1 && strings.TrimSpace(row[1]) == totalRowLabel {
			totalRow = row
			continue
		}
		transactions = append(transactions, row)
	}
	if totalRow == nil {
		return nil, fmt.Errorf("the export has no %q row to check the balances against", totalRowLabel)
	}
	if len(transactions) == 0 {
		return nil, fmt.Errorf("the export has no transactions")
	}

	// Check one: every transaction must sum to zero across the accounts.
	sums := make([]int64, len(accounts))
	for _, row := range transactions {
		var rowSum int64
		for i := range accounts {
			cents, err := parseCents(cell(row, fixedColumns+i))
			if err != nil {
				return nil, fmt.Errorf("row %q, column %q: %w",
					cell(row, 1), accounts[i], err)
			}
			sums[i] += cents
			rowSum += cents
		}
		if rowSum != 0 {
			return nil, fmt.Errorf(
				"transaction %q on %s does not balance: its columns sum to %s, not zero",
				cell(row, 1), cell(row, 0), formatCents(rowSum))
		}
	}

	// Check two: the sums must match what the export says they are.
	for i, account := range accounts {
		stated, err := parseCents(cell(totalRow, fixedColumns+i))
		if err != nil {
			return nil, fmt.Errorf("the %q row, column %q: %w", totalRowLabel, account, err)
		}
		if stated != sums[i] {
			return nil, fmt.Errorf(
				"%q does not reconcile: the transactions sum to %s but the export states %s",
				account, formatCents(sums[i]), formatCents(stated))
		}
	}

	roster := &Roster{TransactionCount: len(transactions)}
	for i, account := range accounts {
		name := strings.TrimSpace(account)
		if name == clubColumn {
			roster.ClubMirrorCents = sums[i]
			continue
		}
		if name == "" {
			continue
		}

		member := RosterMember{Name: name, BalanceCents: sums[i]}
		if strings.HasSuffix(name, removedSuffix) {
			member.Removed = true
			member.Name = strings.TrimSpace(strings.TrimSuffix(name, removedSuffix))
		}
		roster.Members = append(roster.Members, member)
	}

	if len(roster.Members) == 0 {
		return nil, fmt.Errorf("the export names no members")
	}
	if err := validateRosterIdentities(roster.Members); err != nil {
		return nil, err
	}

	// Check three: the club column must mirror the members exactly. This is
	// implied by the first two, but it is the property the seed depends on, so
	// it is worth failing on directly rather than by inference.
	if credit := roster.MemberCreditCents(); credit+roster.ClubMirrorCents != 0 {
		return nil, fmt.Errorf(
			"the books do not close: members hold %s but the club account mirrors %s",
			formatCents(credit), formatCents(roster.ClubMirrorCents))
	}

	return roster, nil
}

// validateRosterIdentities proves that the lossy slug used for fixture emails
// and Auth0 subjects is still one-to-one for this particular export. Without
// this check two different names can collapse into one user and ledger account.
func validateRosterIdentities(members []RosterMember) error {
	bySlug := make(map[string]string, len(members))
	for _, member := range members {
		key := slug(member.Name)
		if key == "" {
			return fmt.Errorf("member %q cannot be turned into a fixture identity", member.Name)
		}
		if previous, exists := bySlug[key]; exists {
			return fmt.Errorf("members %q and %q both map to fixture identity %q",
				previous, member.Name, key)
		}
		bySlug[key] = member.Name
	}
	return nil
}

// parseCents converts an export amount to integer cents without ever becoming a
// float, per constitution principle V. The export writes every amount as
// -?ddd.dd, and anything else is rejected rather than guessed at.
func parseCents(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}

	negative := strings.HasPrefix(value, "-")
	value = strings.TrimPrefix(value, "-")

	whole, frac, found := strings.Cut(value, ".")
	if !found {
		return 0, fmt.Errorf("%q has no decimal point; expected an amount like 12.34", value)
	}
	if len(frac) != 2 {
		return 0, fmt.Errorf("%q does not have exactly two decimal places", value)
	}

	wholeCents, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount: %w", value, err)
	}
	fracCents, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%q is not an amount: %w", value, err)
	}

	cents := wholeCents*100 + fracCents
	if negative {
		cents = -cents
	}
	return cents, nil
}

// formatCents renders cents for an error message, integers all the way.
func formatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "-"
		cents = -cents
	}
	return fmt.Sprintf("%s$%d.%02d", sign, cents/100, cents%100)
}

func hasContent(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return true
		}
	}
	return false
}

func cell(row []string, i int) string {
	if i >= len(row) {
		return ""
	}
	return row[i]
}

// slug turns a member's name into the local part of their fixture address.
// "SrinivasAddagatla" and "Hari Prasad" both have to come out usable.
func slug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			b.WriteRune('-')
		}
	}
	return strings.Trim(b.String(), "-")
}
