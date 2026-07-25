package github

import (
	"fmt"
	"strconv"
	"strings"
)

// Ref is a parsed GitHub reference: a repo (optional) and an issue/PR number.
type Ref struct {
	Repo   string // "owner/name", or "" if not specified
	Number int
}

// ParseRef parses a user-supplied reference into a Ref. Accepted forms:
//
//	#42                          — number only; repo resolved by caller
//	42                           — same
//	owner/name#42                — repo + number
//	https://github.com/o/n/issues/42
//	https://github.com/o/n/pull/42
//
// Returns an error if no number can be derived.
func ParseRef(s string) (Ref, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}, fmt.Errorf("empty reference")
	}

	// URL form.
	if strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://") {
		return parseURLRef(s)
	}

	// Strip a leading '#'.
	s = strings.TrimPrefix(s, "#")

	// "owner/name#42" — split on the last '#'.
	if idx := strings.LastIndex(s, "#"); idx >= 0 {
		repo := strings.TrimSpace(s[:idx])
		num, err := strconv.Atoi(strings.TrimSpace(s[idx+1:]))
		if err != nil {
			return Ref{}, fmt.Errorf("invalid number in %q", s)
		}
		return Ref{Repo: normalizeRepo(repo), Number: num}, nil
	}

	// "owner/name" alone is not a valid task link (no number).
	// Bare number.
	num, err := strconv.Atoi(s)
	if err != nil {
		return Ref{}, fmt.Errorf("invalid reference %q (use #42, owner/name#42, or a URL)", s)
	}
	return Ref{Number: num}, nil
}

// parseURLRef extracts repo + number from a github.com URL.
func parseURLRef(s string) (Ref, error) {
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	// "github.com/owner/name/issues/42" or ".../pull/42"
	s = strings.TrimPrefix(s, "github.com/")
	parts := strings.Split(s, "/")
	// expect: [owner, name, kind, number]
	if len(parts) < 4 {
		return Ref{}, fmt.Errorf("could not parse GitHub URL %q", s)
	}
	repo := parts[0] + "/" + parts[1]
	num, err := strconv.Atoi(parts[3])
	if err != nil {
		return Ref{}, fmt.Errorf("invalid issue number in URL %q", parts[3])
	}
	return Ref{Repo: repo, Number: num}, nil
}

// String renders a Ref as "owner/name#N" or "#N" when repo is empty.
func (r Ref) String() string {
	if r.Repo == "" {
		return fmt.Sprintf("#%d", r.Number)
	}
	return fmt.Sprintf("%s#%d", r.Repo, r.Number)
}
