// Package github wraps the `gh` CLI to read issues, PRs, and their metadata.
//
// Dispatch reuses `gh`'s authentication (the user's existing `gh auth login`)
// rather than managing its own tokens. Every call shells out to `gh api` and
// parses JSON, so no Go GitHub SDK is needed. If `gh` is absent or
// unauthenticated, calls return ErrGhUnavailable so the CLI can degrade
// gracefully.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrGhUnavailable is returned when the `gh` CLI is missing or not
// authenticated. Callers should surface a friendly install hint.
var ErrGhUnavailable = errors.New("gh CLI not available or not authenticated (run `gh auth login`)")

// Client wraps the gh CLI.
type Client struct {
	ghPath string // resolved path to gh; empty means "look up in PATH"
}

// NewClient returns a Client. It does not verify gh is installed — that check
// happens on first use so construction never fails.
func NewClient() *Client {
	return &Client{}
}

// Available reports whether gh is installed and authenticated.
func (c *Client) Available(ctx context.Context) error {
	gh, err := c.path()
	if err != nil {
		return ErrGhUnavailable
	}
	out, err := exec.CommandContext(ctx, gh, "auth", "status").CombinedOutput()
	if err != nil {
		return fmt.Errorf("%w: %s", ErrGhUnavailable, strings.TrimSpace(string(out)))
	}
	return nil
}

// path returns the gh executable path, looking it up once and caching it.
func (c *Client) path() (string, error) {
	if c.ghPath != "" {
		return c.ghPath, nil
	}
	p, err := exec.LookPath("gh")
	if err != nil {
		return "", err
	}
	c.ghPath = p
	return p, nil
}

// Issue is the subset of an issue/PR Dispatch displays.
type Issue struct {
	Number  int    `json:"number"`
	Title   string `json:"title"`
	State   string `json:"state"` // "open" | "closed"
	HTMLURL string `json:"html_url"`
	// IsPR reports whether this is a pull request (issues and PRs share the
	// issues endpoint in the GitHub API).
	IsPR bool `json:"-"`
	Body string `json:"body,omitempty"`
}

// PR is the subset of a pull request with merge/CI detail.
type PR struct {
	Number    int    `json:"number"`
	Title     string `json:"title"`
	State     string `json:"state"`
	HTMLURL   string `json:"html_url"`
	Draft     bool   `json:"draft"`
	HeadRef   string `json:"-"`        // populated from Head.Ref below
	Body      string `json:"body,omitempty"`
	Mergeable *bool  `json:"mergeable,omitempty"`
	Head      *struct {
		Ref string `json:"ref"`
	} `json:"head,omitempty"`
}

// Ref returns the PR head branch name, or "" if absent.
func (p PR) Ref() string {
	if p.Head != nil {
		return p.Head.Ref
	}
	return p.HeadRef
}

// api runs `gh api <args>` and unmarshals the JSON output into dst.
func (c *Client) api(ctx context.Context, dst any, args ...string) error {
	gh, err := c.path()
	if err != nil {
		return ErrGhUnavailable
	}
	full := append([]string{"api", "-H", "Accept: application/vnd.github+json"}, args...)
	out, err := exec.CommandContext(ctx, gh, full...).Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return fmt.Errorf("gh api: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return fmt.Errorf("gh api: %w", err)
	}
	if dst == nil {
		return nil
	}
	return json.Unmarshal(out, dst)
}

// GetIssue fetches a single issue or PR by number from owner/name.
func (c *Client) GetIssue(ctx context.Context, repo string, number int) (*Issue, error) {
	repo = normalizeRepo(repo)
	var raw struct {
		Number       int    `json:"number"`
		Title        string `json:"title"`
		State        string `json:"state"`
		HTMLURL      string `json:"html_url"`
		Body         string `json:"body"`
		PullRequest  *struct{} `json:"pull_request,omitempty"`
	}
	if err := c.api(ctx, &raw, "repos/"+repo+"/issues/"+itoa(number)); err != nil {
		return nil, err
	}
	return &Issue{
		Number: raw.Number, Title: raw.Title, State: raw.State,
		HTMLURL: raw.HTMLURL, IsPR: raw.PullRequest != nil, Body: raw.Body,
	}, nil
}

// ListOpenIssues returns open issues for the repo (max 30 by default).
func (c *Client) ListOpenIssues(ctx context.Context, repo string) ([]Issue, error) {
	repo = normalizeRepo(repo)
	var raw []struct {
		Number      int      `json:"number"`
		Title       string   `json:"title"`
		State       string   `json:"state"`
		HTMLURL     string   `json:"html_url"`
		PullRequest *struct{} `json:"pull_request,omitempty"`
	}
	if err := c.api(ctx, &raw, "repos/"+repo+"/issues?state=open&sort=created&direction=desc"); err != nil {
		return nil, err
	}
	out := make([]Issue, 0, len(raw))
	for _, r := range raw {
		out = append(out, Issue{
			Number: r.Number, Title: r.Title, State: r.State,
			HTMLURL: r.HTMLURL, IsPR: r.PullRequest != nil,
		})
	}
	return out, nil
}

// ListOpenPRs returns open pull requests for the repo.
func (c *Client) ListOpenPRs(ctx context.Context, repo string) ([]PR, error) {
	repo = normalizeRepo(repo)
	var prs []PR
	if err := c.api(ctx, &prs, "repos/"+repo+"/pulls?state=open&sort=created&direction=desc"); err != nil {
		return nil, err
	}
	return prs, nil
}

// PRChecks returns the CI status summary for a PR (counts of pass/fail/pending).
type CheckSummary struct {
	Total   int
	Passing int
	Failing int
	Pending int
}

// PRChecks summarizes the latest CI runs for a PR.
func (c *Client) PRChecks(ctx context.Context, repo string, prNumber int) (CheckSummary, error) {
	repo = normalizeRepo(repo)
	var resp struct {
		States []struct {
			Status string `json:"status"` // COMPLETED, IN_PROGRESS, QUEUED
			Conclusion string `json:"conclusion"` // SUCCESS, FAILURE, NEUTRAL...
		} `json:"statuses"`
	}
	if err := c.api(ctx, &resp, "repos/"+repo+"/commits/"+itoa(prNumber)+"/check-runs"); err != nil {
		// check-runs lives under a different path; fall back gracefully.
		return CheckSummary{}, nil
	}
	var s CheckSummary
	s.Total = len(resp.States)
	for _, st := range resp.States {
		switch {
		case st.Conclusion == "SUCCESS":
			s.Passing++
		case st.Conclusion == "FAILURE":
			s.Failing++
		case st.Status != "COMPLETED":
			s.Pending++
		}
	}
	return s, nil
}

// normalizeRepo ensures a repo ref is "owner/name". It strips a leading
// "https://github.com/" if present.
func normalizeRepo(repo string) string {
	repo = strings.TrimSpace(repo)
	repo = strings.TrimPrefix(repo, "https://github.com/")
	repo = strings.TrimPrefix(repo, "http://github.com/")
	repo = strings.TrimSuffix(repo, "/")
	return repo
}

// NormalizeRepoArg is the exported form of normalizeRepo for use by the CLI.
func NormalizeRepoArg(repo string) string { return normalizeRepo(repo) }

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
