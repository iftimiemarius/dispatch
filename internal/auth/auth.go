// Package auth stores OAuth tokens for Dispatch's integrations (Outlook).
//
// It prefers the OS keyring (via go-keyring: Secret Service on Linux, Keychain
// on macOS, Credential Manager on Windows) and falls back to a 0600 file at
// the config dir when no keyring is available (common on headless servers).
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

// Service is the keyring service name under which tokens are stored.
const Service = "dispatch"

// Token is a stored OAuth token (access + refresh + expiry). Generic enough
// for any provider; the Graph client fills it in.
type Token struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"` // unix seconds
	Scope        string `json:"scope,omitempty"`
}

// SaveToken stores a token for the given account (e.g. "outlook") using the
// keyring, falling back to an encrypted-on-permissions file if unavailable.
func SaveToken(account string, t Token) error {
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err := keyring.Set(Service, account, string(data)); err == nil {
		return nil
	}
	// Fallback: file at config dir.
	return saveTokenFile(account, data)
}

// LoadToken retrieves a stored token, or ErrNoToken if none exists.
func LoadToken(account string) (Token, error) {
	secret, err := keyring.Get(Service, account)
	if err == nil {
		return parseToken([]byte(secret))
	}
	if err != keyring.ErrNotFound {
		// keyring present but errored — try file fallback.
		if data, ferr := readTokenFile(account); ferr == nil {
			return parseToken(data)
		}
		return Token{}, fmt.Errorf("read token: %w", err)
	}
	// keyring has nothing — try file fallback before giving up.
	if data, ferr := readTokenFile(account); ferr == nil {
		return parseToken(data)
	}
	return Token{}, ErrNoToken
}

// DeleteToken removes a stored token from both keyring and file.
func DeleteToken(account string) error {
	_ = keyring.Delete(Service, account) // ignore not-found
	return deleteTokenFile(account)
}

// ErrNoToken is returned by LoadToken when no token is stored.
var ErrNoToken = fmt.Errorf("no token stored")

func parseToken(data []byte) (Token, error) {
	var t Token
	if err := json.Unmarshal(data, &t); err != nil {
		return Token{}, err
	}
	return t, nil
}

// --- file fallback ---

// tokenDir resolves the fallback storage directory. We mirror the XDG config
// dir so tokens live alongside config.toml.
func tokenDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "dispatch", "tokens")
	return dir, os.MkdirAll(dir, 0o700)
}

func tokenPath(account string) (string, error) {
	dir, err := tokenDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, account+".json"), nil
}

func saveTokenFile(account string, data []byte) error {
	p, err := tokenPath(account)
	if err != nil {
		return err
	}
	// Write 0600: only the owner can read the token.
	return os.WriteFile(p, data, 0o600)
}

func readTokenFile(account string) ([]byte, error) {
	p, err := tokenPath(account)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(p)
}

func deleteTokenFile(account string) error {
	p, err := tokenPath(account)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
