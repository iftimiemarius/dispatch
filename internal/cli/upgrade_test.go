package cli

import "testing"

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v0.1.0":  "0.1.0",
		"V0.1.0":  "0.1.0",
		"0.1.0":   "0.1.0",
		"":        "",
		"v":       "",
	}
	for in, want := range cases {
		if got := normalizeVersion(in); got != want {
			t.Errorf("normalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsUpgrade(t *testing.T) {
	cases := []struct {
		current, latest string
		want            bool
	}{
		{"0.1.0", "0.2.0", true},
		{"0.1.0", "1.0.0", true},
		{"0.1.0", "0.1.1", true},
		{"0.2.0", "0.1.0", false},
		{"0.1.0", "0.1.0", false},
		{"", "0.1.0", true},
		{"0.1.0", "", false},
	}
	for _, c := range cases {
		if got := isUpgrade(c.current, c.latest); got != c.want {
			t.Errorf("isUpgrade(%q, %q) = %v, want %v", c.current, c.latest, got, c.want)
		}
	}
}
