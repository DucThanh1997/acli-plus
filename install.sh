#!/bin/sh
# Install acli-plus onto your PATH. POSIX sh — safe to run via `curl ... | sh`.
#
# Resolution order:
#   1. A prebuilt binary in ./dist matching this machine (when run from a checkout).
#   2. Download the matching binary from the GitLab release (no Go needed).
#   3. Build from source if run inside the repo and Go is installed.
#
# Environment overrides:
#   ACLI_PLUS_VERSION   version to install (default below)         e.g. 0.1.0
#   ACLI_PLUS_BASE_URL  GitLab project URL that hosts releases
#   ACLI_PLUS_TOKEN     PRIVATE-TOKEN for a private project (optional)
#   INSTALL_DIR         target dir (default /usr/local/bin, else ~/.local/bin)
set -eu

APP="acli-plus"
VERSION="${ACLI_PLUS_VERSION:-0.1.0}"
# GitLab project URL that hosts releases (override with ACLI_PLUS_BASE_URL).
BASE_URL="${ACLI_PLUS_BASE_URL:-https://gitlab.techvify.dev/d14/ai-kit-group/acli-plus}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
TOKEN="${ACLI_PLUS_TOKEN:-}"

# Directory of this script (".", i.e. cwd, when piped through sh).
SELF="${0:-}"
case "$SELF" in
  */*) REPO_DIR=$(CDPATH= cd -- "$(dirname -- "$SELF")" && pwd) ;;
  *)   REPO_DIR="." ;;
esac

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$arch" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "Unsupported architecture: $arch" >&2; exit 1 ;;
esac
asset="${APP}_${os}_${arch}"

tmpbin=""
cleanup() { [ -n "$tmpbin" ] && rm -f "$tmpbin" || true; }
trap cleanup EXIT

download() {
  _url="$1"; _out="$2"
  if command -v curl >/dev/null 2>&1; then
    if [ -n "$TOKEN" ]; then
      curl -fsSL --header "PRIVATE-TOKEN: ${TOKEN}" -o "$_out" "$_url"
    else
      curl -fsSL -o "$_out" "$_url"
    fi
  elif command -v wget >/dev/null 2>&1; then
    if [ -n "$TOKEN" ]; then
      wget -q --header="PRIVATE-TOKEN: ${TOKEN}" -O "$_out" "$_url"
    else
      wget -q -O "$_out" "$_url"
    fi
  else
    echo "Need curl or wget to download." >&2
    return 1
  fi
}

# Resolve a binary into $src.
src=""
if [ -f "$REPO_DIR/dist/$asset" ]; then
  echo "Using local prebuilt: dist/$asset (no download)"
  src="$REPO_DIR/dist/$asset"
else
  url="${BASE_URL}/-/releases/v${VERSION}/downloads/${asset}"
  echo "Downloading ${APP} v${VERSION} for ${os}/${arch}..."
  echo "  $url"
  tmpbin=$(mktemp)
  if download "$url" "$tmpbin"; then
    src="$tmpbin"
  elif [ -f "$REPO_DIR/go.mod" ] && command -v go >/dev/null 2>&1; then
    echo "Download failed; building from source with $(go version | awk '{print $3}')..."
    ( cd "$REPO_DIR" && CGO_ENABLED=0 go build -trimpath -o "$tmpbin" . )
    src="$tmpbin"
  else
    echo "Error: could not download and no source/Go to build from." >&2
    echo "  - check ACLI_PLUS_BASE_URL (currently: $BASE_URL) and ACLI_PLUS_VERSION" >&2
    echo "  - for a private project, set ACLI_PLUS_TOKEN" >&2
    exit 1
  fi
fi

# Install to a directory on PATH.
dest="$INSTALL_DIR/$APP"
if [ -d "$INSTALL_DIR" ] && [ -w "$INSTALL_DIR" ]; then
  cp "$src" "$dest" && chmod 0755 "$dest"
elif command -v sudo >/dev/null 2>&1; then
  echo "Writing to $INSTALL_DIR needs elevated permission (sudo)..."
  sudo mkdir -p "$INSTALL_DIR"
  sudo cp "$src" "$dest"
  sudo chmod 0755 "$dest"
else
  INSTALL_DIR="$HOME/.local/bin"
  dest="$INSTALL_DIR/$APP"
  echo "Falling back to user directory: $INSTALL_DIR"
  mkdir -p "$INSTALL_DIR"
  cp "$src" "$dest" && chmod 0755 "$dest"
fi

echo "Installed: $dest"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "NOTE: $INSTALL_DIR is not on your PATH. Add to your shell profile:"
     echo "  export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
esac

echo "Verifying:"
"$dest" version
echo "Done. Next: $APP setup"
