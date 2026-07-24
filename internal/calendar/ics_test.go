package calendar

import (
	"strings"
	"testing"
	"time"

	"github.com/iftimiemarius/dispatch/internal/models"
)

func TestFromBlocks_Encode(t *testing.T) {
	start := time.Date(2025, 7, 24, 9, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	taskID := "01TASK"
	blocks := []*models.Block{
		{
			ID: "B1", Title: "deep work", Notes: "auth refactor",
			StartsAt: start, EndsAt: end, TaskID: &taskID,
		},
		{
			ID: "B2", Title: "standup", StartsAt: end, EndsAt: end.Add(30 * time.Minute),
		},
	}

	cal := FromBlocks(blocks, map[string]string{"01TASK": "fix login"})
	out := cal.Encode()

	checks := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//dispatch//work orchestration//EN",
		"BEGIN:VEVENT",
		"UID:block-B1@dispatch",
		"SUMMARY:deep work",
		"DESCRIPTION:auth refactor",
		"END:VEVENT",
		"END:VCALENDAR",
	}
	for _, c := range checks {
		if !strings.Contains(out, c) {
			t.Errorf("missing %q in output:\n%s", c, out)
		}
	}

	// The DTSTART line must be present; floating local format has no 'Z'.
	if !strings.Contains(out, "DTSTART:") {
		t.Fatalf("missing DTSTART")
	}
	for _, line := range strings.Split(out, "\r\n") {
		if strings.HasPrefix(line, "DTSTART:") {
			if strings.HasSuffix(line, "Z") {
				t.Errorf("DTSTART should be floating local, got %q", line)
			}
		}
	}

	// Two events.
	if got := strings.Count(out, "BEGIN:VEVENT"); got != 2 {
		t.Errorf("want 2 events, got %d", got)
	}
}

func TestEncode_Escaping(t *testing.T) {
	start := time.Date(2025, 7, 24, 9, 0, 0, 0, time.UTC)
	cal := &Calendar{Events: []Event{{
		UID: "x@dispatch", Summary: "a, b; c\\d", Start: start, End: start.Add(time.Hour),
	}}}
	out := cal.Encode()
	if !strings.Contains(out, "SUMMARY:a\\, b\\; c\\\\d") {
		t.Errorf("escaping failed:\n%s", out)
	}
}
