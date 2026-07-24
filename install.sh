#!/bin/sh
# Dispatch installer — POSIX sh.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/iftimiemarius/dispatch/main/install.sh | sh
#
#   # or, to a specific dir:
#   curl -fsSL .../install.sh | sh -s -- --install-dir ~/bin
#
# Downloads the latest Dispatch release for your OS/arch from GitHub Releases,
# verifies the SHA-256 checksum, and installs the binary to ~/.local/bin
# (override with DISPATCH_INSTALL_DIR or --install-dir).

set -eu

REPO="iftimiemarius/dispatch"
INSTALL_DIR="${DISPATCH_INSTALL_DIR:-${HOME}/.local/bin}"
VERSION="${DISPATCH_VERSION:-}"   # empty = latest
# Internal/testing override: point the metadata + download requests elsewhere.
API_BASE="${DISPATCH_API_BASE:-https://api.github.com}"

# --- helpers ----------------------------------------------------------------

info()  { printf '   • %s\n' "$*"; }
warn()  { printf '   ! %s\n' "$*" >&2; }
fatal() { printf '   ✗ %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || fatal "required command not found: $1"
}

# Detect OS.
case "$(uname -s)" in
  Linux*)  GOOS=linux ;;
  Darwin*) GOOS=darwin ;;
  FreeBSD*) GOOS=freebsd ;;
  *) fatal "unsupported OS: $(uname -s)" ;;
esac

# Detect arch (normalize common aliases).
case "$(uname -m)" in
  x86_64|amd64)    GOARCH=amd64 ;;
  aarch64|arm64)   GOARCH=arm64 ;;
  armv7l|armhf)    GOARCH=arm ;;
  *) fatal "unsupported architecture: $(uname -m)" ;;
esac

# Pick archive extension per OS.
case "$GOOS" in
  linux|freebsd) EXT=tar.gz ;;
  darwin)        EXT=tar.gz ;;
esac

# --- dependencies -----------------------------------------------------------

need uname
need curl
# tar/sha256 are checked below; fallback to shasum on macOS.

CHECKSUM_CMD=""
if command -v sha256sum >/dev/null 2>&1; then
  CHECKSUM_CMD=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  CHECKSUM_CMD="shasum -a 256"
else
  fatal "neither sha256sum nor shasum is available"
fi
need tar

printf '\n  Dispatch installer\n  -------------------\n'
info "platform: ${GOOS}/${GOARCH}"
info "install dir: ${INSTALL_DIR}"
[ -n "$VERSION" ] && info "version: ${VERSION}" || info "version: latest"

# --- resolve the release to install -----------------------------------------

API="${API_BASE}/repos/${REPO}/releases"
if [ -n "$VERSION" ]; then
  RELEASE_URL="${API}/tags/${VERSION}"
else
  RELEASE_URL="${API}/latest"
fi

info "fetching release metadata..."
RELEASE_JSON=$(curl -fsSL -H "Accept: application/vnd.github+json" "$RELEASE_URL") \
  || fatal "failed to fetch release metadata from ${RELEASE_URL}"

# Extract the published version tag for display.
RELEASE_TAG=$(printf '%s\n' "$RELEASE_JSON" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"([^"]+)".*/\1/')
[ -n "$RELEASE_TAG" ] || fatal "could not determine release tag"
info "release: ${RELEASE_TAG}"

# --- find the matching asset + checksums file -------------------------------

ASSET_NAME="dispatch_${RELEASE_TAG#v}_${GOOS}_${GOARCH}.${EXT}"
# Some users may receive an archive named without the leading v stripped; try both.
ASSET_NAME_ALT="dispatch_${RELEASE_TAG}_${GOOS}_${GOARCH}.${EXT}"

# GitHub assets URLs appear as "browser_download_url": "...".
find_url() {
  printf '%s\n' "$RELEASE_JSON" \
    | grep '"browser_download_url"' \
    | grep -oE 'https?://[^"]+' \
    | grep -F "/$1" \
    | head -1
}

ASSET_URL=$(find_url "$ASSET_NAME")
if [ -z "$ASSET_URL" ]; then
  ASSET_URL=$(find_url "$ASSET_NAME_ALT")
fi
[ -n "$ASSET_URL" ] || fatal "no release asset matched ${ASSET_NAME} for ${GOOS}/${GOARCH}"

CHECKSUM_URL=$(find_url "checksums.txt")
[ -n "$CHECKSUM_URL" ] || fatal "checksums.txt asset not found in release"

info "asset: ${ASSET_NAME}"

# --- download, verify, extract ----------------------------------------------

TMPDIR=$(mktemp -d 2>/dev/null || mktemp -d -t dispatch)
trap 'rm -rf "$TMPDIR"' EXIT INT TERM

ARCHIVE="${TMPDIR}/${ASSET_NAME}"
CHECKSUMS="${TMPDIR}/checksums.txt"

info "downloading archive..."
curl -fsSL -o "$ARCHIVE" "$ASSET_URL" || fatal "archive download failed"

info "downloading checksums..."
curl -fsSL -o "$CHECKSUMS" "$CHECKSUM_URL" || fatal "checksums download failed"

# Verify: the expected hash for our file from checksums.txt must match.
EXPECTED=$(grep -F "$ASSET_NAME" "$CHECKSUMS" | awk '{print $1}')
[ -n "$EXPECTED" ] || EXPECTED=$(grep -F "$ASSET_NAME_ALT" "$CHECKSUMS" | awk '{print $1}')
[ -n "$EXPECTED" ] || fatal "no checksum entry for ${ASSET_NAME}"

ACTUAL=$($CHECKSUM_CMD "$ARCHIVE" | awk '{print $1}')
if [ "$ACTUAL" != "$EXPECTED" ]; then
  fatal "checksum mismatch for ${ASSET_NAME}:
   expected: ${EXPECTED}
   actual:   ${ACTUAL}"
fi
info "checksum verified"

# Ensure install dir exists.
mkdir -p "$INSTALL_DIR" || fatal "could not create ${INSTALL_DIR}"

info "extracting..."
# Extract into a subdir so we can move just the binary cleanly.
EXTRACT="${TMPDIR}/out"
mkdir -p "$EXTRACT"
tar -xzf "$ARCHIVE" -C "$EXTRACT" || fatal "extraction failed"

BINARY="${EXTRACT}/dispatch"
[ -f "$BINARY" ] || BINARY=$(find "$EXTRACT" -type f -name dispatch | head -1)
[ -f "$BINARY" ] || fatal "dispatch binary not found in archive"

TARGET="${INSTALL_DIR}/dispatch"
mv -f "$BINARY" "$TARGET"
chmod 0755 "$TARGET"

printf '\n  ✓ Dispatch %s installed to %s\n\n' "$RELEASE_TAG" "$TARGET"

# --- PATH hint --------------------------------------------------------------

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *)
    printf '  Note: %s is not on your PATH.\n' "$INSTALL_DIR"
    printf '  Add it to your shell profile, e.g.:\n\n'
    printf '    echo "export PATH=\\"\\$PATH:%s\\"" >> ~/.bashrc && source ~/.bashrc\n\n' "$INSTALL_DIR"
    ;;
esac

printf '  Run: dispatch version\n\n'
