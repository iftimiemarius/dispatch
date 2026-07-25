// Package config resolves Dispatch's on-disk paths and (later) loads the
// optional config file. Paths follow the XDG Base Directory Specification
// so the app lives tidily on Linux/macOS.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/adrg/xdg"
)

// AppName is the directory name used under XDG data/config roots.
const AppName = "dispatch"

// Paths holds the resolved filesystem locations Dispatch uses.
type Paths struct {
	// DataDir holds the database and the default ICS export.
	DataDir string
	// ConfigDir holds the optional config file.
	ConfigDir string
	// DBPath is the SQLite database file.
	DBPath string
	// ConfigFile is the optional user config (TOML, added in a later milestone).
	ConfigFile string
	// DefaultICS is the default export target for calendar export.
	DefaultICS string
}

// Resolve returns the paths Dispatch will use. XDG env vars
// (XDG_DATA_HOME, XDG_CONFIG_HOME) take precedence when set. Missing
// directories are created so a first run "just works".
func Resolve() (*Paths, error) {
	dataDir, err := xdg.DataFile(AppName)
	if err != nil {
		return nil, fmt.Errorf("resolve data dir: %w", err)
	}
	configDir, err := xdg.ConfigFile(AppName)
	if err != nil {
		return nil, fmt.Errorf("resolve config dir: %w", err)
	}

	for _, dir := range []string{dataDir, configDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	return &Paths{
		DataDir:    dataDir,
		ConfigDir:  configDir,
		DBPath:     filepath.Join(dataDir, "dispatch.db"),
		ConfigFile: filepath.Join(configDir, "config.toml"),
		DefaultICS: filepath.Join(dataDir, "calendar.ics"),
	}, nil
}

// Config is the user-tunable settings loaded from the TOML config file. Most
// users never need one; sections only matter when enabling an integration
// (e.g. Outlook OAuth).
type Config struct {
	Outlook OutlookConfig `toml:"outlook"`
	GitHub  GitHubConfig  `toml:"github"`
}

// OutlookConfig holds Microsoft Graph OAuth settings. Populated from the Azure
// app registration the user creates.
type OutlookConfig struct {
	// ClientID is the Application (client) ID from Azure.
	ClientID string `toml:"client_id"`
	// Tenant: "consumers" (personal accounts), "common", "organizations", or a
	// specific tenant GUID. Defaults to "consumers".
	Tenant string `toml:"tenant"`
	// RedirectPort is the localhost port the OAuth callback server listens on.
	// Must match the redirect URI registered in Azure. Defaults to 8484.
	RedirectPort int `toml:"redirect_port"`
}

// GitHubConfig is optional. By default Dispatch shells out to the `gh` CLI for
// auth; a token here overrides that.
type GitHubConfig struct {
	Token string `toml:"token"`
}

// Load reads the TOML config file at path. A missing file is not an error — it
// returns a Config with defaults applied.
func Load(path string) (*Config, error) {
	c := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	if err := toml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	// Apply defaults after load so zero-value fields are filled.
	if c.Outlook.Tenant == "" {
		c.Outlook.Tenant = "consumers"
	}
	if c.Outlook.RedirectPort == 0 {
		c.Outlook.RedirectPort = 8484
	}
	return c, nil
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Outlook: OutlookConfig{
			Tenant:       "consumers",
			RedirectPort: 8484,
		},
	}
}

// OutlookEnabled reports whether Outlook integration is configured (client id
// present).
func (c *Config) OutlookEnabled() bool {
	return c.Outlook.ClientID != ""
}
