#!/usr/bin/env bash
# build-binary.sh <os> <arch>
#
# Compiles ONE target's "batteries-included" binary from whatever is
# already staged in internal/bootstrap/embedded/ (see
# scripts/fetch-embedded.sh — run that first). No feed access here: this
# script only builds, it never touches `az` or the plugin index.
#
# hostVersion comes from outside this script: the pipeline sets
# DONGLE_VERSION (required in CI); locally it falls back to `git describe`,
# then "dev". Index url/branch come from configs/build.yaml (via
# tools/readconfig) and are baked in via -ldflags alongside hostVersion, so
# the resulting binary matches the index coordinates its embedded defaults
# were actually resolved against.
#
# Usage: ./scripts/build-binary.sh darwin arm64
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -ne 2 ]; then
	echo "usage: $0 <os> <arch>" >&2
	exit 2
fi
GOOS="$1"
GOARCH="$2"

# Version is provided by the pipeline via DONGLE_VERSION.
# In CI it is REQUIRED; locally it falls back to git, then "dev".
if [ -n "${CI:-}${TF_BUILD:-}${GITHUB_ACTIONS:-}" ] && [ -z "${DONGLE_VERSION:-}" ]; then
	echo "error: DONGLE_VERSION must be set in CI" >&2
	exit 1
fi
VERSION="${DONGLE_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

eval "$(go run ./tools/readconfig --index)" # INDEX_URL, INDEX_BRANCH

out="dist/dongle-${GOOS}-${GOARCH}"
[ "$GOOS" = "windows" ] && out="${out}.exe"

echo "building ${out} (version $VERSION)..."
GOOS="$GOOS" GOARCH="$GOARCH" go build -tags embed \
	-ldflags "-s -w \
		-X main.hostVersion=$VERSION \
		-X main.indexURL=$INDEX_URL \
		-X main.indexBranch=$INDEX_BRANCH" \
	-o "$out" ./cmd

echo "done: ${out}"
