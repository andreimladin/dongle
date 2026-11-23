#!/usr/bin/env bash
# The release build script:
#
#   ./scripts/build.sh <os> <arch>
#
# Cross-compiles ONE target's host binary to dist/dongle-<os>-<arch>[.exe].
# That's all a release build is: the binary is thin (host only) and embeds
# no plugins and no plugin index — on first run it downloads the index from
# the feed and installs the plugins the index's defaults.yaml lists (see
# internal/builtins.Initialize). So this script needs no feed access and no
# `az`: just Go.
#
# hostVersion comes from outside this script: the pipeline sets
# DONGLE_VERSION (required in CI); locally it falls back to `git describe`,
# then "dev". The index feed identity (org/project/feed/package) comes from
# configs/index.yaml — read through tools/readconfig, so this script
# hardcodes none of it — and is baked in via -ldflags alongside
# hostVersion, so the binary knows where to download its index from.
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -ne 2 ]; then
	echo "usage: $0 <os> <arch>" >&2
	exit 2
fi
goos="$1" goarch="$2"

# Version is provided by the pipeline via DONGLE_VERSION.
# In CI it is REQUIRED; locally it falls back to git, then "dev".
if [ -n "${CI:-}${TF_BUILD:-}${GITHUB_ACTIONS:-}" ] && [ -z "${DONGLE_VERSION:-}" ]; then
	echo "error: DONGLE_VERSION must be set in CI" >&2
	exit 1
fi
version="${DONGLE_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

TOOLBIN=$(mktemp -d)
trap 'rm -rf "$TOOLBIN"' EXIT
go build -o "$TOOLBIN/readconfig" ./tools/readconfig
eval "$("$TOOLBIN/readconfig" --index)" # INDEX_ORG, INDEX_PROJECT, INDEX_FEED, INDEX_PACKAGE

out="dist/dongle-${goos}-${goarch}"
[ "$goos" = "windows" ] && out="${out}.exe"

echo "building ${out} (version $version)..."
GOOS="$goos" GOARCH="$goarch" go build \
	-ldflags "-s -w \
		-X main.hostVersion=$version \
		-X main.indexOrg=$INDEX_ORG \
		-X main.indexProject=$INDEX_PROJECT \
		-X main.indexFeed=$INDEX_FEED \
		-X main.indexPackage=$INDEX_PACKAGE" \
	-o "$out" ./cmd

echo "done: ${out}"
