package release

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRelease_FindAsset(t *testing.T) {
	r := &Release{TagName: "v0.1.0", Assets: []Asset{
		{Name: "dispatch_0.1.0_linux_amd64.tar.gz"},
		{Name: "dispatch_0.1.0_darwin_arm64.tar.gz"},
		{Name: "dispatch_0.1.0_windows_amd64.zip"},
		{Name: "checksums.txt"},
	}}
	cases := []struct {
		plat Platform
		want string
	}{
		{Platform{GOOS: "linux", GOARCH: "amd64", Ext: "tar.gz"}, "dispatch_0.1.0_linux_amd64.tar.gz"},
		{Platform{GOOS: "darwin", GOARCH: "arm64", Ext: "tar.gz"}, "dispatch_0.1.0_darwin_arm64.tar.gz"},
		{Platform{GOOS: "windows", GOARCH: "amd64", Ext: "zip"}, "dispatch_0.1.0_windows_amd64.zip"},
		{Platform{GOOS: "linux", GOARCH: "arm64", Ext: "tar.gz"}, ""}, // not present
	}
	for _, c := range cases {
		got, err := r.FindAsset(c.plat)
		if c.want == "" {
			if err != ErrAssetNotFound {
				t.Errorf("FindAsset(%+v) want ErrAssetNotFound, got %v", c.plat, err)
			}
			continue
		}
		if err != nil || got.Name != c.want {
			t.Errorf("FindAsset(%+v) = %v, %v; want %s", c.plat, got, err, c.want)
		}
	}
}

func TestExpectedChecksum(t *testing.T) {
	body := []byte("abc123  dispatch_0.1.0_linux_amd64.tar.gz\ndef456  dispatch_0.1.0_windows_amd64.zip\n")
	got, err := ExpectedChecksum(body, "dispatch_0.1.0_linux_amd64.tar.gz")
	if err != nil || got != "abc123" {
		t.Fatalf("got %q, %v", got, err)
	}
	// star-prefixed names should still match.
	body2 := []byte("abc123  *dispatch_0.1.0_linux_amd64.tar.gz\n")
	got2, err := ExpectedChecksum(body2, "dispatch_0.1.0_linux_amd64.tar.gz")
	if err != nil || got2 != "abc123" {
		t.Fatalf("star-prefixed: got %q, %v", got2, err)
	}
	if _, err := ExpectedChecksum(body, "missing.tar.gz"); err == nil {
		t.Fatal("want error for missing entry")
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("hello world")
	sum := sha256.Sum256(data)
	good := hex.EncodeToString(sum[:])
	if err := VerifyChecksum(data, good); err != nil {
		t.Fatalf("verify good: %v", err)
	}
	if err := VerifyChecksum(data, "000000"); err == nil {
		t.Fatal("want mismatch error")
	}
}

func TestExtractBinary_TarGz(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("posix extraction test on non-windows")
	}
	data := makeTarGz(t, "dispatch", []byte("#!/bin/sh\nfake"))
	dir := t.TempDir()
	path, err := ExtractBinary(data, Platform{GOOS: "linux", GOARCH: "amd64", Ext: "tar.gz"}, dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if filepath.Base(path) != "dispatch" {
		t.Fatalf("name = %s", filepath.Base(path))
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte("#!/bin/sh\nfake")) {
		t.Fatalf("content mismatch: %q", got)
	}
}

func TestExtractBinary_Zip(t *testing.T) {
	data := makeZip(t, "dispatch.exe", []byte("fakeexe"))
	dir := t.TempDir()
	path, err := ExtractBinary(data, Platform{GOOS: "windows", GOARCH: "amd64", Ext: "zip"}, dir)
	if err != nil {
		t.Fatalf("extract: %v", err)
	}
	if filepath.Base(path) != "dispatch.exe" {
		t.Fatalf("name = %s", filepath.Base(path))
	}
}

func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	tw.Write(content)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func makeZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	f.Write(content)
	w.Close()
	return buf.Bytes()
}
