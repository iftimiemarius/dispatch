package timeparse

import (
	"testing"
	"time"
)

func ref(t *testing.T) time.Time {
	t.Helper()
	// Wed 2025-06-04 10:00 local
	ref, err := time.ParseInLocation("2006-01-02 15:04", "2025-06-04 10:00", time.Local)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestParse_RelativeOffsets(t *testing.T) {
	r := ref(t)
	cases := []struct {
		in   string
		want time.Time
	}{
		{"+2h", r.Add(2 * time.Hour)},
		{"2h", r.Add(2 * time.Hour)},
		{"3d", r.Add(3 * 24 * time.Hour)},
		{"+1w", r.Add(7 * 24 * time.Hour)},
		{"30m", r.Add(30 * time.Minute)},
	}
	for _, c := range cases {
		got, err := Parse(c.in, r)
		if err != nil {
			t.Errorf("Parse(%q) err: %v", c.in, err)
			continue
		}
		if !got.Equal(c.want) {
			t.Errorf("Parse(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParse_DayWords(t *testing.T) {
	r := ref(t) // Wed 2025-06-04 10:00
	cases := []struct {
		in   string
		want time.Time
	}{
		{"today", r},
		{"tomorrow", r.AddDate(0, 0, 1)},
		{"tom", r.AddDate(0, 0, 1)},
		{"mon", r.AddDate(0, 0, 5)},  // next Monday (5 days from Wed)
		{"fri", r.AddDate(0, 0, 2)},  // this Friday (2 days)
		{"next mon", r.AddDate(0, 0, 12)}, // next week Monday
	}
	for _, c := range cases {
		got, err := Parse(c.in, r)
		if err != nil {
			t.Errorf("Parse(%q) err: %v", c.in, err)
			continue
		}
		// compare date only for day-word cases (clock defaults to midnight).
		if got.Location() != c.want.Location() {
		}
		if !sameDate(got, c.want) {
			t.Errorf("Parse(%q) date = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParse_WithClock(t *testing.T) {
	r := ref(t) // Wed 10:00
	cases := []struct {
		in      string
		wantH   int
		wantMin int
	}{
		{"tomorrow 9am", 9, 0},
		{"today 9am", 9, 0},     // already passed 9am today → rolls to tomorrow
		{"fri 14:30", 14, 30},
		{"9pm", 21, 0},
		{"12am", 0, 0}, // midnight
		{"12pm", 12, 0},
	}
	for _, c := range cases {
		got, err := Parse(c.in, r)
		if err != nil {
			t.Errorf("Parse(%q) err: %v", c.in, err)
			continue
		}
		if got.Hour() != c.wantH || got.Minute() != c.wantMin {
			t.Errorf("Parse(%q) = %02d:%02d, want %02d:%02d", c.in, got.Hour(), got.Minute(), c.wantH, c.wantMin)
		}
	}
}

func TestParse_ISO(t *testing.T) {
	r := ref(t)
	got, err := Parse("2025-12-01 14:30", r)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got.Month() != time.December || got.Day() != 1 || got.Hour() != 14 || got.Minute() != 30 {
		t.Fatalf("got %v", got)
	}
}

func TestParse_Today9am_rollsToTomorrow(t *testing.T) {
	r := ref(t) // Wed 10:00
	got, err := Parse("today 9am", r)
	if err != nil {
		t.Fatal(err)
	}
	// 9am has passed (it's 10am), so it should be tomorrow 9am.
	if !sameDate(got, r.AddDate(0, 0, 1)) {
		t.Fatalf("expected tomorrow, got %v", got)
	}
}

func sameDate(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}
