package github

import "testing"

func TestParseRef(t *testing.T) {
	cases := []struct {
		in       string
		wantRepo string
		wantNum  int
		wantErr  bool
	}{
		{"#42", "", 42, false},
		{"42", "", 42, false},
		{"owner/name#42", "owner/name", 42, false},
		{"https://github.com/owner/name/issues/42", "owner/name", 42, false},
		{"https://github.com/owner/name/pull/7", "owner/name", 7, false},
		{"https://github.com/octocat/Hello-World/issues/100", "octocat/Hello-World", 100, false},
		{"", "", 0, true},
		{"owner/name", "", 0, true},     // no number
		{"not-a-number", "", 0, true},
		{"owner/name#abc", "", 0, true},
	}
	for _, c := range cases {
		got, err := ParseRef(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("ParseRef(%q) want error, got %+v", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseRef(%q) unexpected error: %v", c.in, err)
			continue
		}
		if got.Repo != c.wantRepo || got.Number != c.wantNum {
			t.Errorf("ParseRef(%q) = {Repo:%q Number:%d}, want {%q %d}",
				c.in, got.Repo, got.Number, c.wantRepo, c.wantNum)
		}
	}
}

func TestRefString(t *testing.T) {
	if got := (Ref{Number: 42}).String(); got != "#42" {
		t.Errorf("got %q", got)
	}
	if got := (Ref{Repo: "owner/name", Number: 42}).String(); got != "owner/name#42" {
		t.Errorf("got %q", got)
	}
}

func TestNormalizeRepo(t *testing.T) {
	cases := map[string]string{
		"owner/name":                       "owner/name",
		"https://github.com/owner/name":    "owner/name",
		"https://github.com/owner/name/":   "owner/name",
		"  owner/name  ":                   "owner/name",
	}
	for in, want := range cases {
		if got := normalizeRepo(in); got != want {
			t.Errorf("normalizeRepo(%q) = %q, want %q", in, got, want)
		}
	}
}
