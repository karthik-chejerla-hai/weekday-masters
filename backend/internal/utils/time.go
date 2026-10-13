package utils

import (
	"fmt"
	"time"
)

var SydneyLocation *time.Location

func init() {
	var err error
	SydneyLocation, err = time.LoadLocation("Australia/Sydney")
	if err != nil {
		panic("Failed to load Sydney timezone: " + err.Error())
	}
}

// CalculateRSVPDeadline closes RSVP exactly 48 elapsed hours before the
// resolved session start. Elapsed time stays correct across Sydney DST changes.
func CalculateRSVPDeadline(startsAt time.Time) time.Time {
	return startsAt.Add(-48 * time.Hour).In(SydneyLocation)
}

// NowInSydney returns the current time in Sydney timezone
func NowInSydney() time.Time {
	return time.Now().In(SydneyLocation)
}

// ParseDateInSydney parses a date string (YYYY-MM-DD) in Sydney timezone
func ParseDateInSydney(dateStr string) (time.Time, error) {
	return time.ParseInLocation("2006-01-02", dateStr, SydneyLocation)
}

// FormatDateForDisplay formats a date for display
func FormatDateForDisplay(t time.Time) string {
	return t.In(SydneyLocation).Format("Monday, 2 January 2006")
}

// FormatTimeForDisplay formats a time for display
func FormatTimeForDisplay(t time.Time) string {
	return t.In(SydneyLocation).Format("3:04 PM")
}

// StartOfDay returns the start of day in Sydney timezone
func StartOfDay(t time.Time) time.Time {
	inSydney := t.In(SydneyLocation)
	return time.Date(
		inSydney.Year(),
		inSydney.Month(),
		inSydney.Day(),
		0, 0, 0, 0,
		SydneyLocation,
	)
}

// EndOfDay returns the end of day in Sydney timezone
func EndOfDay(t time.Time) time.Time {
	inSydney := t.In(SydneyLocation)
	return time.Date(
		inSydney.Year(),
		inSydney.Month(),
		inSydney.Day(),
		23, 59, 59, 999999999,
		SydneyLocation,
	)
}

// ResolveSessionTimes turns a session's date and its "HH:MM" start and end into
// real instants in Sydney.
//
// Constitution Principle IV asks for times that participate in business rules to
// be stored resolved rather than rebuilt on every read. The reason is DST: a
// Sydney day is not always 24 hours, and reconstructing an instant from a bare
// date plus a wall-clock string is correct right up until the first Sunday in
// October, then quietly is not.
//
// An end time earlier than the start is read as running past midnight, which is
// what a session extended into a third hour actually does.
func ResolveSessionTimes(sessionDate time.Time, startTime, endTime string) (startsAt, endsAt time.Time, err error) {
	day := sessionDate.In(SydneyLocation)

	startsAt, err = resolveClockTime(day, startTime)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	endsAt, err = resolveClockTime(day, endTime)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	if !endsAt.After(startsAt) {
		endsAt = endsAt.AddDate(0, 0, 1)
	}

	return startsAt, endsAt, nil
}

// resolveClockTime places an "HH:MM" wall-clock time on a given Sydney day.
func resolveClockTime(day time.Time, clock string) (time.Time, error) {
	parsed, err := time.Parse("15:04", clock)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time %q, expected HH:MM: %w", clock, err)
	}

	return time.Date(
		day.Year(), day.Month(), day.Day(),
		parsed.Hour(), parsed.Minute(), 0, 0,
		SydneyLocation,
	), nil
}
