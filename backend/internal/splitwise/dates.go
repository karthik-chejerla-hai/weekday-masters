package splitwise

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var numericDate = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})(?:/(\d{4}))?\b`)

const monthPattern = `(jan(?:uary)?|feb(?:ruary)?|mar(?:ch)?|apr(?:il)?|may|jun(?:e)?|jul(?:y)?|aug(?:ust)?|sep(?:t(?:ember)?)?|oct(?:ober)?|nov(?:ember)?|dec(?:ember)?)`

var dayMonth = regexp.MustCompile(`(?i)\b(\d{1,2})(?:st|nd|rd|th)?[\s:-]*` + monthPattern + `\b(?:\s+(\d{4}))?`)
var monthDay = regexp.MustCompile(`(?i)\b` + monthPattern + `\s*(\d{1,2})(?:st|nd|rd|th)?\b(?:\s+(\d{4}))?`)

// PlayDate uses explicit title dates, with the closest year to the source date
// when omitted. It never invents a clock time. Date-less titles are flagged.
func PlayDate(r Row) (time.Time, string, error) {
	d, m, y := 0, 0, 0
	if p := numericDate.FindStringSubmatch(r.Description); p != nil {
		d, _ = strconv.Atoi(p[1])
		m, _ = strconv.Atoi(p[2])
		y, _ = strconv.Atoi(p[3])
		if m > 12 && d <= 12 {
			d, m = m, d
		} // Only unambiguous US-style dates.
	} else if p := dayMonth.FindStringSubmatch(r.Description); p != nil {
		d, _ = strconv.Atoi(p[1])
		m = monthNumber(p[2])
		y, _ = strconv.Atoi(p[3])
	} else if p := monthDay.FindStringSubmatch(r.Description); p != nil {
		m = monthNumber(p[1])
		d, _ = strconv.Atoi(p[2])
		y, _ = strconv.Atoi(p[3])
	} else {
		return r.Date, "recorded", nil
	}
	if y == 0 {
		y = r.Date.Year()
		best := time.Duration(1<<63 - 1)
		for _, candidate := range []int{r.Date.Year() - 1, r.Date.Year(), r.Date.Year() + 1} {
			t := time.Date(candidate, time.Month(m), d, 0, 0, 0, 0, r.Date.Location())
			distance := t.Sub(r.Date)
			if distance < 0 {
				distance = -distance
			}
			if distance < best {
				best = distance
				y = candidate
			}
		}
	}
	date := time.Date(y, time.Month(m), d, 0, 0, 0, 0, r.Date.Location())
	if int(date.Month()) != m || date.Day() != d {
		return time.Time{}, "", fmt.Errorf("invalid play date on row %d", r.Number)
	}
	return date, "title", nil
}

func monthNumber(s string) int {
	for i, m := range []string{"jan", "feb", "mar", "apr", "may", "jun", "jul", "aug", "sep", "oct", "nov", "dec"} {
		if strings.HasPrefix(strings.ToLower(s), m) {
			return i + 1
		}
	}
	return 0
}
