package release

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ErrAssetNotFound is returned when no release asset matches the platform.
var ErrAssetNotFound = errors.New("no release asset matches this platform")

// FindAsset returns the release asset for the given platform, e.g.
// "dispatch_0.1.0_linux_amd64.tar.gz". It matches by suffix so it is robust to
// version-number differences (with or without a leading 'v').
func (r *Release) FindAsset(p Platform) (Asset, error) {
	suffix := fmt.Sprintf("_%s_%s.%s", p.GOOS, p.GOARCH, p.Ext)
	for _, a := range r.Assets {
		if strings.HasSuffix(a.Name, suffix) {
			return a, nil
		}
	}
	return Asset{}, ErrAssetNotFound
}

// FindChecksum returns the checksums.txt asset, if present.
func (r *Release) FindChecksum() (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == "checksums.txt" {
			return a, true
		}
	}
	return Asset{}, false
}

// VerifyChecksum computes the sha256 of data and compares it against the
// expected hex digest.
func VerifyChecksum(data []byte, expectedHex string) error {
	sum := sha256.Sum256(data)
	got := hex.EncodeToString(sum[:])
	if got != strings.ToLower(strings.TrimSpace(expectedHex)) {
		return fmt.Errorf("checksum mismatch: want %s, got %s", expectedHex, got)
	}
	return nil
}

// ExpectedChecksum extracts the sha256 digest for assetName from a checksums.txt
// body. The file is GoReleaser's standard "<hex>  <filename>" format.
func ExpectedChecksum(checksumsBody []byte, assetName string) (string, error) {
	for _, line := range strings.Split(string(checksumsBody), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// "<hex>  <name>" or "<hex> <name>"; name may be prefixed with '*'.
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == assetName {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s", assetName)
}

// ExtractBinary reads an archive (tar.gz or zip) from data and writes the
// dispatch( dispatch.exe) binary it contains to outDir, returning its path.
func ExtractBinary(data []byte, p Platform, outDir string) (string, error) {
	switch p.Ext {
	case "zip":
		return extractZip(data, outDir)
	default:
		return extractTarGz(data, outDir)
	}
}

func extractTarGz(data []byte, outDir string) (string, error) {
	gz, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("open gzip: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if base := filepath.Base(hdr.Name); isBinary(base) && hdr.Typeflag == tar.TypeReg {
			return writeExtracted(outDir, base, tr, hdr.Mode)
		}
	}
	return "", errors.New("dispatch binary not found in archive")
}

func extractZip(data []byte, outDir string) (string, error) {
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		if base := filepath.Base(f.Name); isBinary(base) {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()
			return writeExtracted(outDir, base, rc, int64(f.Mode()))
		}
	}
	return "", errors.New("dispatch.exe not found in archive")
}

// isBinary reports whether name is the dispatch executable (any OS).
func isBinary(name string) bool {
	low := strings.ToLower(name)
	return low == "dispatch" || low == "dispatch.exe"
}

// writeExtracted streams an archive entry into outDir/<name> and returns the
// path. It also sets the executable bit on POSIX.
func writeExtracted(outDir, name string, r io.Reader, mode int64) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(outDir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, r); err != nil {
		return "", err
	}
	if mode != 0 {
		_ = os.Chmod(path, os.FileMode(mode))
	}
	return path, nil
}
