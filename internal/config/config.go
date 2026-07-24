// Package config resolves Dispatch's on-disk paths and (later) loads the
// optional config file. Paths follow the XDG Base Directory Specification
// so the app lives tidily on Linux/macOS.
package config

import (
	"fmt"
	"os"
	"path/filepath"

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
