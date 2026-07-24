// Package timeparse turns developer-friendly time strings into time.Time.
//
// Supported forms (case-insensitive, anchored to the reference time's local
// timezone):
//
//	today, now
//	tomorrow, tomorrow 9am
//	mon, monday, fri 9am, next mon
//	+2h, +3d, +1w   (relative offsets from now)
//	2h, 3d           (same as +2h, +3d)
//	2025-12-01, 2025-12-01 14:30
//
// When a clock is given (e.g. "9am", "14:30") the date portion defaults to
// today (or tomorrow if that time has already passed). Otherwise the date
// defaults to the start of that day.
package timeparse

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var weekdays = map[string]time.Weekday{
	"sun": time.Sunday, "sunday": time.Sunday,
	"mon": time.Monday, "monday": time.Monday,
	"tue": time.Tuesday, "tuesday": time.Tuesday,
	"wed": time.Wednesday, "wednesday": time.Wednesday,
	"thu": time.Thursday, "thursday": time.Thursday,
	"fri": time.Friday, "friday": time.Friday,
	"sat": time.Saturday, "saturday": time.Saturday,
}

// Parse interprets s as a time relative to ref (typically now). It returns an
// absolute time in the local zone. Empty input returns the zero time and no
// error.
func Parse(s string, ref time.Time) (time.Time, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return time.Time{}, nil
	}

	// Relative offset: "+2h", "+3d", "2h", "3d".
	if t, ok, err := tryOffset(s, ref); ok {
		return t, err
	}
	// Absolute ISO date[ time].
	if t, ok, err := tryISO(s); ok {
		return t, err
	}

	now := ref
	parts := splitDateClock(s)
	dayPart := parts[0]
	clockPart := parts[1]

	day, err := parseDay(dayPart, now)
	if err != nil {
		return time.Time{}, err
	}
	clock, hasClock, err := parseClock(clockPart)
	if err != nil {
		return time.Time{}, err
	}
	result := day
	if hasClock {
		result = time.Date(day.Year(), day.Month(), day.Day(),
			clock.Hour, clock.Min, 0, 0, now.Location())
		// If the target lands on today but the clock already passed, roll to
		// tomorrow. (Explicit weekdays never land on today via nextWeekday.)
		if isSameDay(day, now) && result.Before(now) {
			result = result.AddDate(0, 0, 1)
		}
	} else {
		// No clock: keep the start of the day.
		result = time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, now.Location())
	}
	return result, nil
}

// splitDateClock splits "tomorrow 9am" -> ["tomorrow", "9am"]. Clock tokens
// contain digits, ':' or 'a'/'p'/'m'. Everything else is the day part.
func splitDateClock(s string) [2]string {
	tokens := strings.Fields(s)
	if len(tokens) == 0 {
		return [2]string{"", ""}
	}
	if len(tokens) == 1 {
		if looksLikeClock(tokens[0]) {
			return [2]string{"", tokens[0]}
		}
		return [2]string{tokens[0], ""}
	}
	// >1 tokens: treat the trailing token as the clock if it looks like one.
	last := tokens[len(tokens)-1]
	if looksLikeClock(last) {
		return [2]string{strings.Join(tokens[:len(tokens)-1], " "), last}
	}
	return [2]string{strings.Join(tokens, " "), ""}
}

func looksLikeClock(s string) bool {
	// "9am", "14:30", "9:00am"
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == ':' || r == 'a' || r == 'p' || r == 'm' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func parseDay(s string, now time.Time) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "today" || s == "now" {
		return now, nil
	}
	if s == "tomorrow" || s == "tom" {
		return now.AddDate(0, 0, 1), nil
	}
	// "next <weekday>"
	if strings.HasPrefix(s, "next ") {
		wd, ok := weekdays[strings.TrimSpace(strings.TrimPrefix(s, "next "))]
		if !ok {
			return time.Time{}, fmt.Errorf("unknown weekday in %q", s)
		}
		return nextWeekday(now, wd, 7), nil
	}
	// bare weekday
	if wd, ok := weekdays[s]; ok {
		return nextWeekday(now, wd, 1), nil
	}
	// "in 3d" / "in 2 days"
	if strings.HasPrefix(s, "in ") {
		body := strings.TrimSpace(strings.TrimPrefix(s, "in "))
		if t, ok, err := tryOffset(body, now); ok {
			return t, err
		}
	}
	return time.Time{}, fmt.Errorf("could not parse date %q", s)
}

// nextWeekday returns the next occurrence of wd on or after now. If daysAway
// is 1, a match today is allowed; if 7, it always rolls to next week ("next mon").
func nextWeekday(now time.Time, wd time.Weekday, minDays int) time.Time {
	delta := (int(wd) - int(now.Weekday()) + 7) % 7
	if delta < minDays {
		delta += 7
	}
	return now.AddDate(0, 0, delta)
}

type clk struct{ Hour, Min int }

func parseClock(s string) (clk, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return clk{}, false, nil
	}
	am := strings.HasSuffix(s, "am")
	pm := strings.HasSuffix(s, "pm")
	if am || pm {
		s = strings.TrimSuffix(s, "am")
		s = strings.TrimSuffix(s, "pm")
	}
	s = strings.TrimSpace(s)
	var hour, min int
	var err error
	if strings.Contains(s, ":") {
		parts := strings.SplitN(s, ":", 2)
		hour, err = strconv.Atoi(parts[0])
		if err != nil {
			return clk{}, true, fmt.Errorf("invalid hour in %q", s)
		}
		min, err = strconv.Atoi(parts[1])
		if err != nil {
			return clk{}, true, fmt.Errorf("invalid minutes in %q", s)
		}
	} else {
		hour, err = strconv.Atoi(s)
		if err != nil {
			return clk{}, true, fmt.Errorf("invalid hour %q", s)
		}
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return clk{}, true, fmt.Errorf("time out of range: %dh%dm", hour, min)
	}
	if (am || pm) && hour > 12 {
		return clk{}, true, fmt.Errorf("12-hour clock hour > 12: %d", hour)
	}
	if pm && hour < 12 {
		hour += 12
	}
	if am && hour == 12 {
		hour = 0 // 12am == midnight
	}
	return clk{Hour: hour, Min: min}, true, nil
}

// tryOffset handles "+2h", "3d", "1w", and "in 3 days"-style phrases.
func tryOffset(s string, ref time.Time) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "+")
	if s == "" {
		return time.Time{}, false, nil
	}
	// must be <number><unit>
	if len(s) < 2 {
		return time.Time{}, false, nil
	}
	last := s[len(s)-1]
	switch last {
	case 'h', 'd', 'w', 'm':
	default:
		return time.Time{}, false, nil
	}
	numStr := s[:len(s)-1]
	n, err := strconv.Atoi(numStr)
	if err != nil {
		return time.Time{}, false, nil
	}
	var d time.Duration
	switch last {
	case 'h':
		d = time.Duration(n) * time.Hour
	case 'd':
		d = time.Duration(n) * 24 * time.Hour
	case 'w':
		d = time.Duration(n) * 7 * 24 * time.Hour
	case 'm':
		d = time.Duration(n) * time.Minute
	}
	return ref.Add(d), true, nil
}

// tryISO parses absolute forms "2006-01-02" and "2006-01-02 15:04".
func tryISO(s string) (time.Time, bool, error) {
	s = strings.TrimSpace(s)
	if len(s) < 10 || s[4] != '-' {
		return time.Time{}, false, nil
	}
	layouts := []string{
		"2006-01-02 15:04:05",
		"2006-01-02 15:04",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true, nil
		}
	}
	return time.Time{}, false, nil
}

func isSameDay(a, b time.Time) bool {
	ay, amm, ad := a.Date()
	by, bmm, bd := b.Date()
	return ay == by && amm == bmm && ad == bd
}
