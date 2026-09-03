#!/usr/bin/env bash
# Plain, versioned build of the host only — no embedded plugins. This is
# what local development and `go build ./cmd` both boil down to; the only
# difference here is that hostVersion gets stamped from the pipeline/git
# instead of defaulting to "dev". Index url/branch are left empty/default
# here (this is a plain build) — set DONGLE_INDEX_URL/DONGLE_INDEX_BRANCH at
# runtime to use `dongle index`/`dongle plugin` commands. See
# scripts/build-release.sh for the batteries-included, per-platform build
# with embedded defaults and injected index coordinates.
set -eu
cd "$(dirname "$0")/.."

# Version is provided by the pipeline via DONGLE_VERSION.
# In CI it is REQUIRED; locally it falls back to git, then "dev".
if [ -n "${CI:-}${TF_BUILD:-}${GITHUB_ACTIONS:-}" ] && [ -z "${DONGLE_VERSION:-}" ]; then
	echo "error: DONGLE_VERSION must be set in CI" >&2
	exit 1
fi
VERSION="${DONGLE_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

mkdir -p dist
go build -ldflags "-X main.hostVersion=$VERSION" -o dist/dongle ./cmd

echo "built dist/dongle ($VERSION, no embedded plugins)"
