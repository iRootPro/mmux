#!/usr/bin/env sh
set -eu

REPO="${MMUX_REPO:-iRootPro/mmux}"
VERSION="${MMUX_VERSION:-latest}"
INSTALL_DIR="${MMUX_INSTALL_DIR:-/usr/local/bin}"
BIN_NAME="${MMUX_BIN:-mmux}"
TMP_DIR="${TMPDIR:-/tmp}/mmux-install-$$"

log() { printf '%s\n' "$*" >&2; }
fatal() { log "error: $*"; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fatal "required command not found: $1"; }

need uname
need tar
need curl

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  [ -n "$VERSION" ] || fatal "could not resolve latest release"
fi

case "$(uname -s)" in
  Linux) os="linux" ;;
  Darwin) os="darwin" ;;
  *) fatal "unsupported OS: $(uname -s)" ;;
esac

case "$(uname -m)" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) fatal "unsupported architecture: $(uname -m)" ;;
esac

mkdir -p "$TMP_DIR"
trap 'rm -rf "$TMP_DIR"' EXIT INT TERM

# New assets use mmux_* and contain a mmux binary. v0.1.0 also has legacy
# band-tui_* assets, so keep a fallback for macOS/older releases.
asset="mmux_${VERSION}_${os}_${arch}.tar.gz"
url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"
archive="$TMP_DIR/$asset"

log "Downloading $url"
if ! curl -fL --progress-bar -o "$archive" "$url"; then
  legacy="band-tui_${VERSION}_${os}_${arch}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${VERSION}/${legacy}"
  archive="$TMP_DIR/$legacy"
  log "Falling back to $url"
  curl -fL --progress-bar -o "$archive" "$url" || fatal "download failed"
fi

tar -xzf "$archive" -C "$TMP_DIR"

binary="$(find "$TMP_DIR" -type f \( -name mmux -o -name band-tui \) -perm -111 | head -n 1 || true)"
[ -n "$binary" ] || binary="$(find "$TMP_DIR" -type f \( -name mmux -o -name band-tui \) | head -n 1 || true)"
[ -n "$binary" ] || fatal "binary not found in archive"
chmod +x "$binary"

target="$INSTALL_DIR/$BIN_NAME"
if [ -w "$INSTALL_DIR" ]; then
  install -m 0755 "$binary" "$target"
else
  log "Installing to $target with sudo"
  sudo install -m 0755 "$binary" "$target"
fi

log "Installed: $target"
"$target" --help >/dev/null 2>&1 || true
printf 'mmux installed successfully: %s\n' "$target"
