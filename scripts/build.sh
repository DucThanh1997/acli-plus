#!/usr/bin/env bash
# Cross-compile acli-plus for common platforms into ./dist.
# Run this once on a machine that has Go; ship ./dist + install.sh to users who
# do not have Go so they can install a prebuilt binary.
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-0.1.0}"
LDFLAGS="-s -w -X acli-plus/internal/cmd.version=${VERSION}"

platforms=(
  "darwin/amd64"
  "darwin/arm64"
  "linux/amd64"
  "linux/arm64"
  "windows/amd64"
)

mkdir -p dist
rm -f dist/acli-plus_*

for platform in "${platforms[@]}"; do
  os="${platform%/*}"
  arch="${platform#*/}"
  out="dist/acli-plus_${os}_${arch}"
  [ "$os" = "windows" ] && out="${out}.exe"
  echo "building ${out}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$out" .
done

echo
echo "Built version ${VERSION}:"
ls -la dist/
echo
echo "To distribute to machines without Go:"
echo "  tar czf acli-plus-dist.tgz dist install.sh"
echo "  # recipient: tar xzf acli-plus-dist.tgz && ./install.sh"
