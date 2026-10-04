package splitwise

import (
	"strings"
	"testing"

	"github.com/weekday-masters/backend/internal/utils"
)

func TestReadRejectsUnbalancedAndMalformedExports(t *testing.T) {
	valid := "Date,Description,Category,Cost,Currency,Alice,Badminton Account\n2024-01-01,Topup,General,10.00,AUD,10.00,-10.00\n2024-01-01,Total balance, , ,AUD,10.00,-10.00\n"
	if _, err := Read(strings.NewReader(valid)); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		strings.Replace(valid, "10.00,-10.00", "10.00,-9.99", 1),
		strings.ReplaceAll(valid, "AUD", "USD"),
		strings.Replace(valid, "Total balance, , ,AUD,10.00,-10.00", "Total balance, , ,AUD,9.00,-9.00", 1),
		strings.Replace(valid, "Alice,Badminton Account", "Alice,Alice", 1),
		valid + "2024-01-01,Other,General,1.00,AUD,1.00,-1.00\n",
	} {
		if _, err := Read(strings.NewReader(s)); err == nil {
			t.Fatalf("accepted invalid source %q", s)
		}
	}
}

func TestCentsNeverRoundsOrAcceptsExponents(t *testing.T) {
	for s, want := range map[string]int64{"0.01": 1, "-0.84": -84, "598.23": 59823} {
		got, err := Cents(s)
		if err != nil || got != want {
			t.Fatalf("%s: %d %v", s, got, err)
		}
	}
	for _, s := range []string{"1e3", "1.234", "1", "NaN", "9223372036854775808.00"} {
		if _, err := Cents(s); err == nil {
			t.Fatal(s)
		}
	}
}

func TestTitleDatesAndRecordedFallback(t *testing.T) {
	for _, tc := range []struct{ title, recorded, want, basis string }{
		{"Game : 27th September", "2022-09-30", "2022-09-27", "title"},
		{"Game - Dec3", "2024-12-05", "2024-12-03", "title"},
		{"Game - 03/24", "2026-03-24", "2026-03-24", "title"},
		{"Game - 28:Oct", "2024-10-29", "2024-10-28", "title"},
		{"Game - July 11", "2023-07-11", "2023-07-11", "title"},
		{"Game 31/12", "2025-01-02", "2024-12-31", "title"},
		{"Game - 13 October 2022", "2022-10-13", "2022-10-13", "title"},
		{"Game", "2024-04-09", "2024-04-09", "recorded"},
	} {
		t.Run(tc.title, func(t *testing.T) {
			d, _ := utils.ParseDateInSydney(tc.recorded)
			got, basis, err := PlayDate(Row{Description: tc.title, Date: d})
			if err != nil || got.Format("2006-01-02") != tc.want || basis != tc.basis {
				t.Fatalf("%v %s %v", got, basis, err)
			}
		})
	}
}
