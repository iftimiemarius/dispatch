// Package calendar renders calendar blocks as RFC 5545 iCalendar (.ics)
// content for import into any calendar application.
package calendar

import (
	"fmt"
	"strings"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

// Calendar is a collection of events to be serialized.
type Calendar struct {
	ProdID  string // identifies the product that created the calendar
	Events  []Event
}

// Event is a single VEVENT.
type Event struct {
	UID       string
	Summary   string
	Description string
	Start     time.Time
	End       time.Time
}

// FromBlocks builds a Calendar from blocks, resolving titles/notes. If
// taskTitles is non-nil it is consulted to title events tied to a task.
func FromBlocks(blocks []*models.Block, taskTitles map[string]string) *Calendar {
	cal := &Calendar{ProdID: "dispatch"}
	for _, b := range blocks {
		summary := b.Title
		desc := b.Notes
		if b.TaskID != nil {
			if title, ok := taskTitles[*b.TaskID]; ok && summary == "" {
				summary = title
			}
			if desc == "" {
				desc = "task: " + *b.TaskID
			}
		}
		cal.Events = append(cal.Events, Event{
			UID:         "block-" + b.ID + "@dispatch",
			Summary:     summary,
			Description: desc,
			Start:       b.StartsAt,
			End:         b.EndsAt,
		})
	}
	return cal
}

// Encode renders the calendar to RFC 5545 text. Times are emitted as
// "floating" local times (no Z suffix): blocks apply in the user's local zone
// wherever the calendar is opened.
func (c *Calendar) Encode() string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//dispatch//work orchestration//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	for _, e := range c.Events {
		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString("UID:" + e.UID + "\r\n")
		b.WriteString("DTSTAMP:" + formatUTC(time.Now().UTC()) + "\r\n")
		b.WriteString("DTSTART:" + formatLocal(e.Start) + "\r\n")
		b.WriteString("DTEND:" + formatLocal(e.End) + "\r\n")
		b.WriteString("SUMMARY:" + escape(e.Summary) + "\r\n")
		if e.Description != "" {
			b.WriteString("DESCRIPTION:" + escape(e.Description) + "\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// formatLocal emits a floating local time: YYYYMMDDTHHMMSS (no zone).
func formatLocal(t time.Time) string {
	t = t.Local()
	return fmt.Sprintf("%04d%02d%02dT%02d%02d%02d",
		t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// formatUTC emits a UTC time with the trailing Z.
func formatUTC(t time.Time) string {
	return fmt.Sprintf("%04d%02d%02dT%02d%02d%02dZ",
		t.Year(), int(t.Month()), t.Day(), t.Hour(), t.Minute(), t.Second())
}

// escape applies minimal RFC 5545 text escaping (comma, semicolon, newline,
// backslash).
func escape(s string) string {
	r := strings.NewReplacer(
		"\\", "\\\\",
		";", "\\;",
		",", "\\,",
		"\n", "\\n",
	)
	return r.Replace(s)
}
