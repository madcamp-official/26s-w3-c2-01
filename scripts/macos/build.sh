#!/usr/bin/env bash
# Builds a universal (amd64 + arm64) libra binary for macOS and packages it
# as dist/libra-<version>-macos.tar.gz — the artifact attached to GitHub
# Releases and referenced by Formula/libra.rb's precompiled path (if used).
#
# Requires Go 1.25+ and Xcode Command Line Tools (for `lipo`):
#   xcode-select --install
#
# Usage:
#   scripts/macos/build.sh [version]
# `version` defaults to `git describe --tags --always`, falling back to
# "0.0.0-dev" if no tags exist.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$REPO_ROOT"

VERSION="${1:-$(git describe --tags --always 2>/dev/null || true)}"
VERSION="${VERSION:-0.0.0-dev}"

echo "Building libra (macOS universal, version $VERSION)..."
LDFLAGS="-s -w -X github.com/madcamp-official/26s-w3-c2-01/cmd.Version=$VERSION"

mkdir -p dist
GOOS=darwin GOARCH=amd64 go build -ldflags "$LDFLAGS" -o dist/libra-amd64 .
GOOS=darwin GOARCH=arm64 go build -ldflags "$LDFLAGS" -o dist/libra-arm64 .

lipo -create -output dist/libra dist/libra-amd64 dist/libra-arm64
rm dist/libra-amd64 dist/libra-arm64
chmod +x dist/libra

tar -czf "dist/libra-$VERSION-macos.tar.gz" -C dist libra
echo "Done: dist/libra-$VERSION-macos.tar.gz"
