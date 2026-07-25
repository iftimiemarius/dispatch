// Package graph is a minimal Microsoft Graph client for calendar events.
//
// It implements the OAuth 2.0 Authorization Code flow with PKCE (no client
// secret required for public/native apps) and a small set of calendar REST
// calls. Tokens are stored via the internal/auth package (OS keyring).
package graph

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/iftimiemarius/dispatch/internal/auth"
	"golang.org/x/oauth2"
)

// account is the keyring account name under which the Outlook token is stored.
const account = "outlook"

// Scope is the Microsoft Graph delegated permission for calendar read/write.
const Scope = "Calendars.ReadWrite offline_access"

// Authenticator runs the OAuth flow and stores the resulting token.
type Authenticator struct {
	Config     *oauth2.Config
	RedirectPort int
}

// NewAuthenticator builds an OAuth2 config from the provided settings. The
// tenant selects the audience:
//
//	"consumers"    — personal Microsoft accounts only (login.live.com)
//	"common"       — any org + personal accounts
//	"organizations" — work/school accounts only
//	"<tenant-guid>" — a single specific tenant
//
// We construct the v2.0 endpoint URLs explicitly rather than relying on
// endpoints.Microsoft, whose contents vary across x/oauth2 versions (some map
// to the legacy login.live.com endpoint, which is /consumers-only).
func NewAuthenticator(clientID, tenant string, redirectPort int) *Authenticator {
	if tenant == "" {
		tenant = "common"
	}
	if redirectPort == 0 {
		redirectPort = 8484
	}
	endpoint := microsoftEndpoint(tenant)

	return &Authenticator{
		Config: &oauth2.Config{
			ClientID:    clientID,
			Endpoint:    endpoint,
			RedirectURL: fmt.Sprintf("http://localhost:%d/callback", redirectPort),
			Scopes:      strings.Fields(Scope),
		},
		RedirectPort: redirectPort,
	}
}

// microsoftEndpoint returns the v2.0 OAuth2 endpoints for the given tenant.
//
//	"consumers"     → login.live.com (personal accounts only)
//	"common"        → .../common (any org + personal)
//	"organizations" → .../organizations (work/school only)
//	"<guid>"        → .../<guid> (single tenant)
func microsoftEndpoint(tenant string) oauth2.Endpoint {
	if tenant == "consumers" {
		// The personal-only legacy endpoint.
		return oauth2.Endpoint{
			AuthURL:  "https://login.live.com/oauth20_authorize.srf",
			TokenURL: "https://login.live.com/oauth20_token.srf",
		}
	}
	base := "https://login.microsoftonline.com/" + tenant
	return oauth2.Endpoint{
		AuthURL:  base + "/oauth2/v2.0/authorize",
		TokenURL: base + "/oauth2/v2.0/token",
	}
}
func (a *Authenticator) LoginURL(state, verifier string) string {
	challenge := pkceChallenge(verifier)
	return a.Config.AuthCodeURL(state,
		oauth2.SetAuthURLParam("code_challenge", challenge),
		oauth2.SetAuthURLParam("code_challenge_method", "S256"),
		oauth2.AccessTypeOffline,
	)
}

// Exchange swaps an authorization code for tokens, storing them in the keyring.
// It runs a local HTTP server on RedirectPort to receive the redirect.
//
// We send client_id explicitly (public-client PKCE: no secret) so that Azure
// recognizes the request as coming from a registered public app. The token
// request is built and sent manually so we control exactly which params ship
// (the oauth2 library has historically been finicky with MSFT public clients).
func (a *Authenticator) Exchange(ctx context.Context, code, verifier string) error {
	form := url.Values{
		"client_id":     {a.Config.ClientID},
		"code":          {code},
		"code_verifier": {verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {a.Config.RedirectURL},
		"scope":         {strings.Join(a.Config.Scopes, " ")},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		a.Config.Endpoint.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("token exchange: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token exchange: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var raw struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("decode token response: %w", err)
	}
	tok := &oauth2.Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(raw.ExpiresIn) * time.Second),
	}
	return SaveToken(tok)
}

// --- PKCE helpers ---

// pkceVerifier generates a random 43-128 char code_verifier (RFC 7636).
func pkceVerifier() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// pkceChallenge returns the S256 code_challenge for a verifier.
func pkceChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// randState returns a short random string for CSRF protection.
func randState() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// --- token storage bridge (oauth2.Token → auth.Token) ---

// SaveToken converts an oauth2 token to the auth storage format and persists it.
func SaveToken(t *oauth2.Token) error {
	at := auth.Token{
		AccessToken:  t.AccessToken,
		RefreshToken: t.RefreshToken,
		Scope:        "",
	}
	if t.Expiry != (time.Time{}) {
		at.ExpiresAt = t.Expiry.Unix()
	}
	return auth.SaveToken(account, at)
}

// ErrNotAuthenticated is returned when no Outlook token is stored.
var ErrNotAuthenticated = errors.New("not authenticated — run `dispatch auth login outlook`")

// tokenSource returns an oauth2.TokenSource that auto-refreshes, backed by the
// keyring. It returns ErrNotAuthenticated if no token exists.
func tokenSource(ctx context.Context, cfg *oauth2.Config) (oauth2.TokenSource, error) {
	raw, err := auth.LoadToken(account)
	if err != nil {
		return nil, ErrNotAuthenticated
	}
	stored := &oauth2.Token{
		AccessToken:  raw.AccessToken,
		RefreshToken: raw.RefreshToken,
	}
	if raw.ExpiresAt != 0 {
		stored.Expiry = time.Unix(raw.ExpiresAt, 0)
	}
	ts := &refreshingSource{
		cfg:     cfg,
		current: stored,
		mu:      &sync.Mutex{},
	}
	return oauth2.ReuseTokenSource(nil, ts), nil
}

// refreshingSource wraps oauth2.Config.TokenSource but persists refreshed
// tokens back to the keyring so the new refresh token survives.
type refreshingSource struct {
	cfg     *oauth2.Config
	current *oauth2.Token
	mu      *sync.Mutex
}

func (r *refreshingSource) Token() (*oauth2.Token, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// If the current token is still valid, return it.
	if r.current.Valid() {
		return r.current, nil
	}
	if r.current.RefreshToken == "" {
		return nil, ErrNotAuthenticated
	}
	// Refresh via the configured endpoint.
	ts := r.cfg.TokenSource(context.Background(), &oauth2.Token{RefreshToken: r.current.RefreshToken})
	t, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("refresh token: %w", err)
	}
	// Persist the refreshed token.
	if err := SaveToken(t); err != nil {
		// Non-fatal: we still have an in-memory token to return.
		_ = err
	}
	r.current = t
	return t, nil
}

// HTTPClient returns an *http.Client that injects a valid (auto-refreshing)
// access token into every request.
func HTTPClient(ctx context.Context, cfg *oauth2.Config) (*http.Client, error) {
	ts, err := tokenSource(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, ts), nil
}

// Logout removes the stored Outlook token.
func Logout() error {
	return auth.DeleteToken(account)
}

// HasToken reports whether an Outlook token is currently stored.
func HasToken() bool {
	_, err := auth.LoadToken(account)
	return err == nil
}

// PKCEVerifier is exported for the login command (which must keep the verifier
// to pass to Exchange).
func PKCEVerifier() (string, error) { return pkceVerifier() }

// RandState is exported for the login command's CSRF state.
func RandState() (string, error) { return randState() }
