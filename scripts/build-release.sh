#!/usr/bin/env bash
# Builds "batteries-included" per-platform release binaries: the host plus
# the plugins listed in defaults.lock, baked in via go:embed behind the
# "embed" build tag (see cmd/embed.go). NOT run in CI — this is a release
# maintainer's manual/local step.
#
# No feed coordinates are hardcoded here: this script clones the plugin
# index fresh on every run and reads each plugin's Azure Artifacts feed
# (organization/feed/project) and per-platform package name straight out of
# its index manifest (plugins/<name>.yaml), the same file `dongle plugin
# install` resolves against. It then needs `az` credentials for that feed.
#
# Requires: yq, az (logged in to the feed), git, go.
#
# For a plain, plugin-less build use scripts/build.sh (or `go build ./cmd`
# directly).
set -eu
cd "$(dirname "$0")/.."

# Fallback index URL/branch — the same defaults the CLI itself uses
# (internal/index.go's IndexURL/IndexBranch). DONGLE_INDEX_URL and
# DONGLE_INDEX_BRANCH override them here exactly like they do for the CLI,
# so this script and `dongle index refresh` always agree on which index
# they're pointed at.
DEFAULT_INDEX_URL="https://dev.azure.com/[ORG]/[PROJECT]/_git/[REPO]"
INDEX_URL="${DONGLE_INDEX_URL:-$DEFAULT_INDEX_URL}"
INDEX_BRANCH="${DONGLE_INDEX_BRANCH:-main}"

LOCK_FILE="defaults.lock"
EMBED_DIR="cmd/embedded"
TARGETS=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64)

for bin in yq az git go; do
	if ! command -v "$bin" >/dev/null 2>&1; then
		echo "error: $bin is required (see script header)" >&2
		exit 1
	fi
done

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "cloning index $INDEX_URL (branch $INDEX_BRANCH)..."
if ! git clone --depth 1 --branch "$INDEX_BRANCH" "$INDEX_URL" "$TMP/index" >&2; then
	echo "error: could not clone index $INDEX_URL" >&2
	exit 1
fi

VERSION=$(git describe --tags --always --dirty)

# clean_embed wipes any staged plugin payload but keeps the tracked
# .gitkeep, so the dir stays non-empty for go:embed even between
# platforms/runs.
clean_embed() {
	find "$EMBED_DIR" -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
}

# read_defaults prints "name version" pairs from defaults.lock, skipping
# blank lines and comments.
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

		mf="$TMP/index/plugins/${name}.yaml"
		if [ ! -f "$mf" ]; then
			echo "error: no index manifest for plugin '$name' (expected $mf)" >&2
			exit 1
		fi

		org=$(yq -r '.feed.organization // ""' "$mf")
		feed=$(yq -r '.feed.feed // ""' "$mf")
		project=$(yq -r '.feed.project // ""' "$mf")
		pkg=$(yq -r ".platforms[] | select(.selector.os == \"$GOOS\" and .selector.arch == \"$GOARCH\") | .package" "$mf")

		if [ -z "$org" ] || [ -z "$feed" ]; then
			echo "error: $mf is missing feed.organization or feed.feed" >&2
			exit 1
		fi
		if [ -z "$pkg" ]; then
			echo "error: $mf has no platforms[] entry for $GOOS/$GOARCH" >&2
			exit 1
		fi

		ver="${version#v}" # upack versions are bare semver
		echo "  [$name] downloading $pkg@$ver from feed '$feed' (org $org)..."

		dl_dir="$EMBED_DIR/.download"
		rm -rf "$dl_dir"
		mkdir -p "$dl_dir"

		az_args=(artifacts universal download
			--organization "https://dev.azure.com/${org}"
			--feed "$feed"
			--name "$pkg"
			--version "$ver"
			--path "$dl_dir")
		if [ -n "$project" ]; then
			az_args+=(--project "$project" --scope project)
		fi
		if ! az "${az_args[@]}"; then
			echo "error: az download failed for $pkg@$ver ($GOOS/$GOARCH)" >&2
			exit 1
		fi

		downloaded=$(find "$dl_dir" -maxdepth 1 -type f)
		if [ -z "$downloaded" ] || [ "$(printf '%s\n' "$downloaded" | wc -l)" -ne 1 ]; then
			echo "error: package $pkg@$ver did not contain exactly one file" >&2
			exit 1
		fi

		# Canonicalize to the same entrypoint naming `dongle plugin install`
		# uses: <host binary name>-<plugin name>.
		file="dongle-${name}"
		[ "$GOOS" = "windows" ] && file="${file}.exe"
		mv "$downloaded" "$EMBED_DIR/$file"
		rm -rf "$dl_dir"
		chmod 0755 "$EMBED_DIR/$file"

		manifest_entries+=("{\"name\":\"${name}\",\"version\":\"${ver}\",\"file\":\"${file}\",\"entrypoint\":\"${file}\"}")
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
		-ldflags "-s -w -X main.hostVersion=$VERSION" \
		-o "$out" ./cmd

	clean_embed
	echo "  done: ${out}"
done

echo "release build complete: $VERSION"
