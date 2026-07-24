package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/iftimiemarius/dispatch/internal/release"
	"github.com/iftimiemarius/dispatch/internal/ui"
	"github.com/spf13/cobra"
)

func newUpgradeCmd() *cobra.Command {
	var (
		checkOnly bool
		force     bool
	)
	cmd := &cobra.Command{
		Use:    "upgrade",
		Short:  "Upgrade dispatch to the latest GitHub release",
		Args:   cobra.NoArgs,
		Annotations: map[string]string{"skip_db": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			client := release.NewClient()

			fmt.Fprintln(out, ui.Dim("checking for the latest release..."))
			rel, err := client.Latest(ctx)
			if err != nil {
				return fmt.Errorf("lookup latest release: %w", err)
			}

			current := normalizeVersion(Version)
			latest := normalizeVersion(rel.TagName)
			fmt.Fprintf(out, "  current: %s\n", ui.Dim(current))
			fmt.Fprintf(out, "  latest:  %s\n", ui.Bold(latest))

			if latest == current && !force {
				fmt.Fprintln(out, ui.Dim("  already up to date."))
				return nil
			}
			if !isUpgrade(current, latest) && !force && latest != "" {
				// current is newer than latest (e.g. local dev build) — nothing to do.
				fmt.Fprintln(out, ui.Dim("  you are ahead of the latest release."))
				return nil
			}

			if checkOnly {
				fmt.Fprintf(out, "  an upgrade is available: %s → %s\n", current, latest)
				return nil
			}

			plat := release.CurrentPlatform()
			asset, err := rel.FindAsset(plat)
			if err != nil {
				return fmt.Errorf("no release asset for %s/%s: %w", plat.GOOS, plat.GOARCH, err)
			}
			fmt.Fprintf(out, "  downloading %s ...\n", asset.Name)

			var archive bytes.Buffer
			if _, err := client.Download(ctx, asset, &archive); err != nil {
				return err
			}

			// Verify checksum if checksums.txt is available.
			if csAsset, ok := rel.FindChecksum(); ok {
				var csBody bytes.Buffer
				if _, err := client.Download(ctx, csAsset, &csBody); err != nil {
					return fmt.Errorf("download checksums: %w", err)
				}
				expected, err := release.ExpectedChecksum(csBody.Bytes(), asset.Name)
				if err != nil {
					return err
				}
				if err := release.VerifyChecksum(archive.Bytes(), expected); err != nil {
					return err
				}
				fmt.Fprintln(out, ui.Dim("  checksum verified."))
			}

			// Extract into a temp dir.
			tmpDir, err := os.MkdirTemp("", "dispatch-upgrade-")
			if err != nil {
				return err
			}
			defer os.RemoveAll(tmpDir)

			extracted, err := release.ExtractBinary(archive.Bytes(), plat, tmpDir)
			if err != nil {
				return err
			}

			// Determine the destination: replace the running binary in place.
			execPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("locate running binary: %w", err)
			}
			execPath, err = filepath.EvalSymlinks(execPath)
			if err != nil {
				return fmt.Errorf("resolve binary path: %w", err)
			}

			return installUpgrade(out, extracted, execPath, latest)
		},
	}
	cmd.Flags().BoolVar(&checkOnly, "check", false, "only report whether an upgrade is available")
	cmd.Flags().BoolVar(&force, "force", false, "install even if already up to date")
	return cmd
}

// installUpgrade replaces the current binary at execPath with the new one.
// On Windows, replacing a running executable directly fails, so the old file is
// renamed aside before the new one is moved in.
func installUpgrade(out io.Writer, newBinary, execPath, version string) error {
	// Writable check — if the target is system-wide and needs sudo, bail with a hint.
	if err := canWrite(execPath); err != nil {
		return fmt.Errorf("cannot write to %s (try running with elevated permissions, or re-run the installer): %w", execPath, err)
	}

	backup := execPath + ".old"
	// Best-effort: move the old binary aside (Windows can't overwrite a running .exe).
	_ = os.Rename(execPath, backup)

	src, err := os.Open(newBinary)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.OpenFile(execPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		// Roll back the rename if we failed to open the destination.
		if _, statErr := os.Stat(backup); statErr == nil {
			_ = os.Rename(backup, execPath)
		}
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}
	if err := os.Chmod(execPath, 0o755); err != nil {
		return err
	}

	// Clean up the backup (best-effort). On Windows the old running file may be
	// locked; a later run will remove it.
	go func() {
		time.Sleep(2 * time.Second)
		_ = os.Remove(backup)
	}()

	fmt.Fprintf(out, "  %s upgraded to %s\n", ui.Bold("✓"), version)
	fmt.Fprintf(out, "  installed to %s\n", ui.Dim(execPath))
	return nil
}

// canWrite reports whether the file (or its dir, if missing) is writable.
func canWrite(path string) error {
	// Try a temp file next to the target.
	dir := filepath.Dir(path)
	probe, err := os.CreateTemp(dir, ".dispatch-probe-*")
	if err != nil {
		return err
	}
	probe.Close()
	os.Remove(probe.Name())
	return nil
}

// normalizeVersion strips a leading 'v' for comparison. Empty stays empty.
func normalizeVersion(v string) string {
	for len(v) > 0 && (v[0] == 'v' || v[0] == 'V') {
		v = v[1:]
	}
	return v
}

// isUpgrade reports whether latest is a higher version than current using
// simple semantic-major.minor.patch comparison. Returns true when in doubt
// (e.g. unparseable) so --force is the explicit override.
func isUpgrade(current, latest string) bool {
	if latest == "" {
		return false
	}
	if current == "" {
		return true
	}
	var cMaj, cMin, cPat, lMaj, lMin, lPat int
	fmt.Sscanf(current, "%d.%d.%d", &cMaj, &cMin, &cPat)
	fmt.Sscanf(latest, "%d.%d.%d", &lMaj, &lMin, &lPat)
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPat > cPat
}
