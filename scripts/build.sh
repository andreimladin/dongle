#!/usr/bin/env bash
# Plain, versioned build of the host only — no embedded plugins. This is
# what local development and `go build ./cmd` both boil down to; the only
# difference here is that hostVersion gets stamped from git instead of
# defaulting to "dev". See scripts/build-release.sh for the
# batteries-included, per-platform build with embedded defaults.
set -eu
cd "$(dirname "$0")/.."

VERSION=$(git describe --tags --always --dirty)
mkdir -p dist
go build -ldflags "-X main.hostVersion=$VERSION" -o dist/dongle ./cmd

echo "built dist/dongle ($VERSION, no embedded plugins)"
