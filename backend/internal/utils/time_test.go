package utils

import (
	"testing"
	"time"
)

func TestCalculateRSVPDeadline(t *testing.T) {
	// A Sunday session on 2026-03-15 at 18:00 Sydney time
	sessionDate := time.Date(2026, 3, 15, 18, 0, 0, 0, SydneyLocation)

	deadline := CalculateRSVPDeadline(sessionDate)

	// Deadline should be 3 days earlier: Thursday 2026-03-12 23:59:59
	if deadline.Year() != 2026 || deadline.Month() != 3 || deadline.Day() != 12 {
		t.Fatalf("expected deadline date to be 2026-03-12, got %v", deadline.Format("2006-01-02"))
	}
	if deadline.Hour() != 23 || deadline.Minute() != 59 || deadline.Second() != 59 {
		t.Fatalf("expected deadline time to be 23:59:59, got %02d:%02d:%02d", deadline.Hour(), deadline.Minute(), deadline.Second())
	}
	if deadline.Location().String() != SydneyLocation.String() {
		t.Fatalf("expected location %v, got %v", SydneyLocation, deadline.Location())
	}
}

func TestCalculateRSVPDeadline_AcrossMonthBoundary(t *testing.T) {
	// 2nd of March session -> Deadline is 3 days before (February 27 in a non-leap year)
	sessionDate := time.Date(2026, 3, 2, 10, 0, 0, 0, SydneyLocation)
	deadline := CalculateRSVPDeadline(sessionDate)

	if deadline.Year() != 2026 || deadline.Month() != 2 || deadline.Day() != 27 {
		t.Fatalf("expected deadline date to be 2026-02-27, got %v", deadline.Format("2006-01-02"))
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
