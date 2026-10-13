package utils

import (
	"testing"
	"time"
)

func TestCalculateRSVPDeadlineElapsedHours(t *testing.T) {
	for _, date := range []string{"2026-03-02", "2026-10-05", "2026-04-06"} {
		t.Run(date, func(t *testing.T) {
			day, err := ParseDateInSydney(date)
			if err != nil {
				t.Fatal(err)
			}
			start, _, err := ResolveSessionTimes(day, "18:00", "20:00")
			if err != nil {
				t.Fatal(err)
			}
			deadline := CalculateRSVPDeadline(start)
			if start.Sub(deadline) != 48*time.Hour {
				t.Fatalf("deadline is not 48 elapsed hours before start: %v", deadline)
			}
			if deadline.Location() != SydneyLocation {
				t.Fatal("deadline lost Sydney display zone")
			}
			if date == "2026-10-05" && deadline.Hour() != 17 {
				t.Fatalf("spring DST wall clock should be 17:00, got %v", deadline)
			}
			if date == "2026-04-06" && deadline.Hour() != 19 {
				t.Fatalf("autumn DST wall clock should be 19:00, got %v", deadline)
			}
		})
	}
}

func TestNowInSydney(t *testing.T) {
	now := NowInSydney()
	if now.Location().String() != SydneyLocation.String() {
		t.Fatalf("expected Sydney location, got %v", now.Location())
	}
}

func TestParseDateInSydney(t *testing.T) {
	parsed, err := ParseDateInSydney("2026-05-10")
	if err != nil {
		t.Fatalf("unexpected error parsing date: %v", err)
	}
	if parsed.Year() != 2026 || parsed.Month() != 5 || parsed.Day() != 10 {
		t.Fatalf("unexpected date parsed: %v", parsed)
	}
	if parsed.Location().String() != SydneyLocation.String() {
		t.Fatalf("expected location %v, got %v", SydneyLocation, parsed.Location())
	}

	_, err = ParseDateInSydney("invalid-date")
	if err == nil {
		t.Fatal("expected error parsing invalid date string, got nil")
	}
}

func TestFormatDateForDisplay(t *testing.T) {
	date := time.Date(2026, 8, 22, 14, 0, 0, 0, SydneyLocation)
	formatted := FormatDateForDisplay(date)
	if formatted != "Saturday, 22 August 2026" {
		t.Fatalf("unexpected format: %q", formatted)
	}
}

func TestFormatTimeForDisplay(t *testing.T) {
	date := time.Date(2026, 8, 22, 18, 30, 0, 0, SydneyLocation)
	formatted := FormatTimeForDisplay(date)
	if formatted != "6:30 PM" {
		t.Fatalf("unexpected format: %q", formatted)
	}
}

func TestStartAndEndOfDay(t *testing.T) {
	date := time.Date(2026, 8, 22, 14, 35, 20, 123456, SydneyLocation)

	start := StartOfDay(date)
	if start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 || start.Nanosecond() != 0 {
		t.Fatalf("start of day should have 00:00:00.0, got %v", start)
	}

	end := EndOfDay(date)
	if end.Hour() != 23 || end.Minute() != 59 || end.Second() != 59 {
		t.Fatalf("end of day should have 23:59:59, got %v", end)
	}
}

// Sydney moves to daylight saving on the first Sunday in October and back on the
// first Sunday in April. A session's start is a wall-clock time on a given day,
// and it has to land on the right instant on both sides of those boundaries —
// this is the whole reason the timestamps are stored resolved rather than
// rebuilt on every read.
func TestResolveSessionTimesAcrossDSTBoundaries(t *testing.T) {
	tests := []struct {
		name       string
		date       string
		start      string
		end        string
		wantOffset string
	}{
		{name: "before the October change", date: "2026-10-03", start: "20:00", end: "22:00", wantOffset: "+1000"},
		{name: "after the October change", date: "2026-10-05", start: "20:00", end: "22:00", wantOffset: "+1100"},
		{name: "before the April change", date: "2026-04-04", start: "20:00", end: "22:00", wantOffset: "+1100"},
		{name: "after the April change", date: "2026-04-06", start: "20:00", end: "22:00", wantOffset: "+1000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			day, err := ParseDateInSydney(tt.date)
			if err != nil {
				t.Fatal(err)
			}

			startsAt, endsAt, err := ResolveSessionTimes(day, tt.start, tt.end)
			if err != nil {
				t.Fatal(err)
			}

			if got := startsAt.Format("-0700"); got != tt.wantOffset {
				t.Errorf("start offset = %s, want %s", got, tt.wantOffset)
			}
			if got := startsAt.Format("15:04"); got != tt.start {
				t.Errorf("start reads %s locally, want %s", got, tt.start)
			}
			if !endsAt.After(startsAt) {
				t.Errorf("end %s is not after start %s", endsAt, startsAt)
			}
		})
	}
}

// A session extended past midnight ends on the following day, not two hours
// before it started.
func TestResolveSessionTimesPastMidnight(t *testing.T) {
	day, err := ParseDateInSydney("2026-08-25")
	if err != nil {
		t.Fatal(err)
	}

	startsAt, endsAt, err := ResolveSessionTimes(day, "23:00", "00:30")
	if err != nil {
		t.Fatal(err)
	}

	if !endsAt.After(startsAt) {
		t.Fatalf("end %s is not after start %s", endsAt, startsAt)
	}
	if got := endsAt.Sub(startsAt); got != 90*time.Minute {
		t.Errorf("duration = %s, want 1h30m", got)
	}
}

// A two-hour session is two hours, and the standard booking plus its extension
// is three — the numbers settlement quotes have to match the clock.
func TestResolveSessionTimesDuration(t *testing.T) {
	day, err := ParseDateInSydney("2026-08-25")
	if err != nil {
		t.Fatal(err)
	}

	for _, tt := range []struct {
		start, end string
		want       time.Duration
	}{
		{"20:00", "22:00", 2 * time.Hour},
		{"20:00", "23:00", 3 * time.Hour},
		{"18:30", "20:30", 2 * time.Hour},
	} {
		startsAt, endsAt, err := ResolveSessionTimes(day, tt.start, tt.end)
		if err != nil {
			t.Fatal(err)
		}
		if got := endsAt.Sub(startsAt); got != tt.want {
			t.Errorf("%s–%s lasted %s, want %s", tt.start, tt.end, got, tt.want)
		}
	}
}

func TestResolveSessionTimesRejectsNonsense(t *testing.T) {
	day, _ := ParseDateInSydney("2026-08-25")

	if _, _, err := ResolveSessionTimes(day, "not-a-time", "22:00"); err == nil {
		t.Error("expected an unparseable start time to be refused")
	}
	if _, _, err := ResolveSessionTimes(day, "20:00", "25:99"); err == nil {
		t.Error("expected an impossible end time to be refused")
	}
}
