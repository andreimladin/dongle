#!/usr/bin/env bash
# The one release build script: cross-compiles "batteries-included"
# binaries for all six supported platforms — the host plus the plugins
# listed in configs/defaults.lock, baked in via go:embed behind the "embed"
# build tag (see internal/bootstrap/bootstrap.go). Release-only; NOT run on
# every PR (see azure-pipelines-ci.yml for that) — this is what
# azure-pipelines-release.yml runs, and it's also runnable by hand for a
# local test build:
#
#   DONGLE_VERSION=1.4.0 ./scripts/build-binaries.sh
#
# configs/ is the single source of truth for build inputs (see
# configs/README.md): index url/branch come from configs/index.env, the
# embedded plugin list from configs/defaults.lock. No feed coordinates are
# hardcoded here: this script clones the plugin index fresh into a temp dir
# on every run — using the same INDEX_URL/INDEX_BRANCH it bakes into the
# binary via -ldflags, so the binary and the clone it resolved plugins
# against always agree — and shells out to tools/resolve (a small
# build-time-only Go helper in this repo — not a dongle subcommand) to read
# each plugin's Azure Artifacts feed (organization/feed/project) and
# per-platform package name straight out of its index manifest
# (plugins/<name>.yaml) — parsed by the exact same internal/index code
# `dongle plugin install` uses, not reimplemented. It then needs `az`
# credentials for the plugin feed (see azure-pipelines-release.yml).
#
# Every defaults.lock plugin must be published (per plugin, per platform) to
# the feed for all six targets below — if tools/resolve can't find a target
# platform in a plugin's manifest, or the az download fails, this script
# fails fast naming the plugin and platform rather than silently shipping a
# binary with that default missing.
#
# hostVersion comes from outside this script: the pipeline sets
# DONGLE_VERSION (required in CI); locally it falls back to `git describe`,
# then "dev".
#
# Requires: az (logged in to the plugin feed), git, go.
#
# For a plain, plugin-less dev build, `go build ./cmd` needs nothing beyond
# the Go toolchain (hostVersion "dev", no index URL baked in).
set -eu
cd "$(dirname "$0")/.."

source configs/index.env # INDEX_URL, INDEX_BRANCH

LOCK_FILE="configs/defaults.lock"
EMBED_DIR="internal/bootstrap/embedded"
TARGETS=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64 windows/arm64)

for bin in az git go; do
	if ! command -v "$bin" >/dev/null 2>&1; then
		echo "error: $bin is required (see script header)" >&2
		exit 1
	fi
done

# Version is provided by the pipeline via DONGLE_VERSION.
# In CI it is REQUIRED; locally it falls back to git, then "dev".
if [ -n "${CI:-}${TF_BUILD:-}${GITHUB_ACTIONS:-}" ] && [ -z "${DONGLE_VERSION:-}" ]; then
	echo "error: DONGLE_VERSION must be set in CI" >&2
	exit 1
fi
# Named HOST_VERSION (not VERSION) because eval-ing the resolve tool's env
# output below sets a VERSION of its own (the plugin's version, per
# invocation) — this is the host's own version, computed once up front.
HOST_VERSION="${DONGLE_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "cloning index $INDEX_URL (branch $INDEX_BRANCH)..."
if ! git clone --depth 1 --branch "$INDEX_BRANCH" "$INDEX_URL" "$TMP/index" >&2; then
	echo "error: could not clone index $INDEX_URL" >&2
	exit 1
fi

echo "building resolve tool..."
if ! go build -o "$TMP/resolve" ./tools/resolve; then
	echo "error: could not build tools/resolve" >&2
	exit 1
fi

# clean_embed wipes any staged plugin payload but keeps the tracked
# .gitkeep, so the dir stays non-empty for go:embed even between
# platforms/runs.
clean_embed() {
	find "$EMBED_DIR" -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
}

# read_defaults prints "name version" pairs from configs/defaults.lock,
# skipping blank lines and comments.
read_defaults() {
	grep -Ev '^[[:space:]]*(#|$)' "$LOCK_FILE"
}

for target in "${TARGETS[@]}"; do
	GOOS="${target%/*}"
	GOARCH="${target#*/}"
	echo "== $GOOS/$GOARCH =="

	clean_embed

	manifest_entries=()
	while read -r name version; do
		[ -n "$name" ] || continue

		echo "  [$name] resolving feed coordinates for $GOOS/$GOARCH..."
		if ! resolved=$("$TMP/resolve" "$name" \
			--version "$version" --os "$GOOS" --arch "$GOARCH" \
			--index "$TMP/index" --format env); then
			echo "error: [$name] has no published build for $GOOS/$GOARCH — cannot embed defaults for this target" >&2
			exit 1
		fi
		eval "$resolved"

		echo "  [$name] downloading $PACKAGE@$VERSION from feed '$FEED' (org $ORG)..."

		dl_dir="$EMBED_DIR/.download"
		rm -rf "$dl_dir"
		mkdir -p "$dl_dir"

		az_args=(artifacts universal download
			--organization "https://dev.azure.com/${ORG}"
			--feed "$FEED"
			--name "$PACKAGE"
			--version "$VERSION"
			--path "$dl_dir")
		if [ -n "$PROJECT" ]; then
			az_args+=(--project "$PROJECT" --scope project)
		fi
		if ! az "${az_args[@]}"; then
			echo "error: az download failed for $PACKAGE@$VERSION ($GOOS/$GOARCH) — plugin [$name] may not be published for this platform" >&2
			exit 1
		fi

		downloaded=$(find "$dl_dir" -maxdepth 1 -type f)
		if [ -z "$downloaded" ] || [ "$(printf '%s\n' "$downloaded" | wc -l)" -ne 1 ]; then
			echo "error: package $PACKAGE@$VERSION did not contain exactly one file" >&2
			exit 1
		fi

		# Canonicalize to the same entrypoint naming `dongle plugin install`
		# uses: <host binary name>-<plugin name>.
		file="dongle-${name}"
		[ "$GOOS" = "windows" ] && file="${file}.exe"
		mv "$downloaded" "$EMBED_DIR/$file"
		rm -rf "$dl_dir"
		chmod 0755 "$EMBED_DIR/$file"

		manifest_entries+=("{\"name\":\"${name}\",\"version\":\"${VERSION}\",\"file\":\"${file}\",\"entrypoint\":\"${file}\"}")
		echo "  [$name] staged as $file"
	done < <(read_defaults)

	{
		printf '[\n'
		last=$((${#manifest_entries[@]} - 1))
		for i in "${!manifest_entries[@]}"; do
			printf '  %s' "${manifest_entries[$i]}"
			[ "$i" -lt "$last" ] && printf ','
			printf '\n'
		done
		printf ']\n'
	} >"$EMBED_DIR/manifest.json"

	out="dist/dongle-${GOOS}-${GOARCH}"
	[ "$GOOS" = "windows" ] && out="${out}.exe"

	echo "  building ${out}..."
	GOOS="$GOOS" GOARCH="$GOARCH" go build -tags embed \
		-ldflags "-s -w \
			-X main.hostVersion=$HOST_VERSION \
			-X main.indexURL=$INDEX_URL \
			-X main.indexBranch=$INDEX_BRANCH" \
		-o "$out" ./cmd

	clean_embed
	echo "  done: ${out}"
done

echo "release build complete: $HOST_VERSION"
