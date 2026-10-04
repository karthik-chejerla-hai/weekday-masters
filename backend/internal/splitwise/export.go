// Package splitwise validates source facts without accessing a database.
package splitwise

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/weekday-masters/backend/internal/utils"
)

const ClubName = "Badminton Account"

var decimal = regexp.MustCompile(`^-?[0-9]+\.[0-9]{2}$`)
var sessionTitle = regexp.MustCompile(`(?i)^game\b|\bextra hour\b`)

type Member struct {
	SourceName string `json:"source_name"`
	Name       string `json:"name"`
	Email      string `json:"email"`
}

type Mapping struct {
	Members    []Member `json:"members"`
	AdminEmail string   `json:"admin_email"`
	// Keys are one-based transaction numbers, excluding blank and total rows.
	SessionPayers map[string]string `json:"session_payers"`
}

type Row struct {
	Number                int
	Date                  time.Time
	Description, Category string
	CostCents             int64
	Amounts               []int64
	IsSession             bool
}

type Export struct {
	Hash      string
	Names     []string
	ClubIndex int
	Rows      []Row
	Totals    []int64
	Cutoff    time.Time
}

func Cents(s string) (int64, error) {
	if !decimal.MatchString(s) {
		return 0, fmt.Errorf("invalid cents amount %q", s)
	}
	// Parsing the concatenated digits checks overflow without floating point.
	return strconv.ParseInt(strings.ReplaceAll(s, ".", ""), 10, 64)
}

func Read(r io.Reader) (*Export, error) {
	data, err := io.ReadAll(io.LimitReader(r, 20<<20+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 20<<20 {
		return nil, fmt.Errorf("export exceeds 20 MiB")
	}
	hash := sha256.Sum256(data)
	reader := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\ufeff")))
	reader.FieldsPerRecord = -1
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 || len(rows[0]) < 7 {
		return nil, fmt.Errorf("missing Splitwise header")
	}
	for i, label := range []string{"Date", "Description", "Category", "Cost", "Currency"} {
		if rows[0][i] != label {
			return nil, fmt.Errorf("expected column %q", label)
		}
	}
	e := &Export{Hash: hex.EncodeToString(hash[:]), Names: rows[0][5:], ClubIndex: -1}
	seen := map[string]bool{}
	for i, n := range e.Names {
		if n == "" || seen[n] {
			return nil, fmt.Errorf("empty or duplicate participant %q", n)
		}
		seen[n] = true
		if n == ClubName {
			e.ClubIndex = i
		}
	}
	if e.ClubIndex < 0 {
		return nil, fmt.Errorf("missing %q", ClubName)
	}
	sums := make([]int64, len(e.Names))
	for _, cells := range rows[1:] {
		if strings.TrimSpace(strings.Join(cells, "")) == "" {
			continue
		}
		if len(cells) != len(rows[0]) {
			return nil, fmt.Errorf("row has %d columns, expected %d", len(cells), len(rows[0]))
		}
		if cells[4] != "AUD" {
			return nil, fmt.Errorf("only AUD is supported")
		}
		date, err := utils.ParseDateInSydney(cells[0])
		if err != nil {
			return nil, err
		}
		if e.Totals != nil {
			return nil, fmt.Errorf("data after total-balance row")
		}
		amounts := make([]int64, len(e.Names))
		var sum int64
		for i, s := range cells[5:] {
			amounts[i], err = Cents(s)
			if err != nil {
				return nil, err
			}
			// Bound source amounts well below int64 to keep sums safe.
			if amounts[i] > 1e12 || amounts[i] < -1e12 {
				return nil, fmt.Errorf("source amount exceeds supported range")
			}
			sum += amounts[i]
		}
		if sum != 0 {
			return nil, fmt.Errorf("row %q has residual %d cents", cells[1], sum)
		}
		if cells[1] == "Total balance" {
			e.Totals = amounts
			e.Cutoff = date
			continue
		}
		cost, err := Cents(cells[3])
		if err != nil {
			return nil, err
		}
		if cost < 0 || cost > 1e12 {
			return nil, fmt.Errorf("invalid cost")
		}
		for i, n := range amounts {
			sums[i] += n
		}
		e.Rows = append(e.Rows, Row{Number: len(e.Rows) + 1, Date: date, Description: cells[1], Category: cells[2], CostCents: cost, Amounts: amounts, IsSession: cells[2] != "Payment" && sessionTitle.MatchString(strings.TrimSpace(cells[1]))})
	}
	if e.Totals == nil || len(e.Rows) == 0 {
		return nil, fmt.Errorf("missing totals or transactions")
	}
	for i, n := range sums {
		if n != e.Totals[i] {
			return nil, fmt.Errorf("%s: calculated %d, stated %d", e.Names[i], n, e.Totals[i])
		}
	}
	for _, row := range e.Rows {
		if row.Date.After(e.Cutoff) {
			return nil, fmt.Errorf("transaction after cutoff")
		}
	}
	return e, nil
}

func (e *Export) ValidateMapping(m Mapping) error {
	names, emails := map[string]bool{}, map[string]bool{}
	for _, p := range m.Members {
		found := false
		for _, n := range e.Names {
			if n == p.SourceName && n != ClubName {
				found = true
			}
		}
		a, err := mail.ParseAddress(p.Email)
		if !found || strings.TrimSpace(p.Name) == "" || err != nil || a.Address != p.Email || p.Email != strings.ToLower(strings.TrimSpace(p.Email)) {
			return fmt.Errorf("invalid member mapping for %q", p.SourceName)
		}
		if names[p.SourceName] || emails[p.Email] {
			return fmt.Errorf("duplicate mapped name or email")
		}
		names[p.SourceName] = true
		emails[p.Email] = true
	}
	if len(names) == 0 || !emails[m.AdminEmail] {
		return fmt.Errorf("admin email must identify a mapped member")
	}
	for i, n := range e.Names {
		if i != e.ClubIndex && !names[n] && e.Totals[i] != 0 {
			return fmt.Errorf("inactive participant %q has non-zero closing balance %d", n, e.Totals[i])
		}
	}
	for key := range m.SessionPayers {
		n, err := strconv.Atoi(key)
		if err != nil || n < 1 || n > len(e.Rows) || !e.Rows[n-1].IsSession {
			return fmt.Errorf("invalid session payer row %q", key)
		}
	}
	for _, r := range e.Rows {
		if r.IsSession {
			date, _, err := PlayDate(r)
			if err != nil {
				return err
			}
			if date.After(e.Cutoff) {
				return fmt.Errorf("play date after export cutoff on row %d", r.Number)
			}
			if _, _, err := e.Charges(r, m); err != nil {
				return err
			}
		}
	}
	return nil
}

// Charges does not assume that a positive member net means sole payer. Such a
// row needs an explicit reviewed payer; ordinary club-funded rows are exact.
func (e *Export) Charges(r Row, m Mapping) (charges, paid []int64, err error) {
	charges = make([]int64, len(e.Names))
	paid = make([]int64, len(e.Names))
	payer := m.SessionPayers[strconv.Itoa(r.Number)]
	if payer == "" {
		payer = ClubName
	}
	pi := -1
	for i, n := range e.Names {
		if n == payer {
			pi = i
		}
	}
	if pi < 0 {
		return nil, nil, fmt.Errorf("unknown payer on row %d", r.Number)
	}
	paid[pi] = r.CostCents
	var total int64
	for i, n := range r.Amounts {
		charges[i] = paid[i] - n
		if charges[i] < 0 || (i == e.ClubIndex && charges[i] != 0) {
			return nil, nil, fmt.Errorf("session row %d needs a reviewed payer allocation", r.Number)
		}
		total += charges[i]
	}
	if total != r.CostCents {
		return nil, nil, fmt.Errorf("session row %d does not reconcile", r.Number)
	}
	return charges, paid, nil
}

func (m Mapping) Hash() string {
	data, _ := json.Marshal(m)
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
