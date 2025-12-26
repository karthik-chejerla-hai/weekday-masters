package utils

import (
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

// CalculateRSVPDeadline calculates the RSVP deadline for a session
// The deadline is 3 days before the session date at 23:59:59 Sydney time
func CalculateRSVPDeadline(sessionDate time.Time) time.Time {
	// Convert to Sydney time
	sessionInSydney := sessionDate.In(SydneyLocation)

	// Subtract 3 days
	deadline := sessionInSydney.AddDate(0, 0, -3)

	// Set time to end of day (23:59:59)
	deadline = time.Date(
		deadline.Year(),
		deadline.Month(),
		deadline.Day(),
		23, 59, 59, 0,
		SydneyLocation,
	)

	return deadline
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
