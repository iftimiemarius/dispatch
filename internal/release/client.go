// Package release talks to GitHub Releases to discover, download, and verify
// Dispatch release artifacts. It is used by the `dispatch upgrade` command and
// could back other self-update flows.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
)

// Repo identifies the GitHub repository hosting releases.
const Repo = "iftimiemarius/dispatch"

// Asset is a single downloadable file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// Release is the subset of a GitHub release that Dispatch cares about.
type Release struct {
	TagName string  `json:"tag_name"`
	Name    string  `json:"name"`
	HTMLURL string  `json:"html_url"`
	Assets  []Asset `json:"assets"`
}

// Client queries the GitHub Releases API.
type Client struct {
	HTTP    *http.Client
	BaseURL string // defaults to https://api.github.com
}

// NewClient returns a Client with a default HTTP client.
func NewClient() *Client {
	return &Client{
		HTTP:    &http.Client{},
		BaseURL: "https://api.github.com",
	}
}

// Latest fetches the most recent published release.
func (c *Client) Latest(ctx context.Context) (*Release, error) {
	return c.getByURL(ctx, fmt.Sprintf("%s/repos/%s/releases/latest", c.baseURL(), Repo))
}

// ByTag fetches a specific release by its tag (e.g. "v0.1.0").
func (c *Client) ByTag(ctx context.Context, tag string) (*Release, error) {
	return c.getByURL(ctx, fmt.Sprintf("%s/repos/%s/releases/tags/%s", c.baseURL(), Repo, tag))
}

func (c *Client) getByURL(ctx context.Context, url string) (*Release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("github returned %s: %s", resp.Status, string(body))
	}

	var r Release
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}
	return &r, nil
}

func (c *Client) baseURL() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return "https://api.github.com"
}

// Download writes the asset content to dst, returning the number of bytes.
func (c *Client) Download(ctx context.Context, asset Asset, dst io.Writer) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, asset.BrowserDownloadURL, nil)
	if err != nil {
		return 0, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", asset.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download %s: %s", asset.Name, resp.Status)
	}
	return io.Copy(dst, resp.Body)
}

// Platform describes the running OS/arch and the archive kind its assets use.
type Platform struct {
	GOOS   string
	GOARCH string
	Ext    string // "tar.gz" or "zip"
}

// CurrentPlatform returns the Platform matching the running binary.
func CurrentPlatform() Platform {
	p := Platform{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH}
	switch runtime.GOOS {
	case "windows":
		p.Ext = "zip"
	default:
		p.Ext = "tar.gz"
	}
	return p
}
