#!/usr/bin/env bash
# Builds "batteries-included" per-platform release binaries: the host plus
# the plugins listed in defaults.lock, baked in via go:embed behind the
# "embed" build tag (see cmd/embed.go). NOT run in CI — this is a release
# maintainer's manual/local step, since it needs `az` credentials for the
# shared Azure Artifacts feed.
#
# For a plain, plugin-less build use scripts/build.sh (or `go build ./cmd`
# directly).
set -eu
cd "$(dirname "$0")/.."

# --- Azure Artifacts feed config -------------------------------------------
# TODO: fill these in for your organization's feed before running this
# script. See README's "Publishing a plugin" section for the naming
# convention these must match.
AZ_ORG="TODO-org"     # e.g. "my-company"
AZ_FEED="TODO-feed"   # e.g. "dongle-plugins"
AZ_PROJECT=""         # TODO: set only if the feed is project-scoped
# ----------------------------------------------------------------------------

LOCK_FILE="defaults.lock"
EMBED_DIR="cmd/embedded"
TARGETS=(darwin/arm64 darwin/amd64 linux/amd64 linux/arm64 windows/amd64)

if ! command -v az >/dev/null 2>&1; then
	echo "error: the Azure CLI (az) is required to download plugin artifacts" >&2
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

		file="dongle-${name}"
		[ "$GOOS" = "windows" ] && file="${file}.exe"
		package="dongle-${name}_${version}_${GOOS}_${GOARCH}"

		echo "  downloading ${package}@${version}..."
		dl_dir="$EMBED_DIR/.download"
		rm -rf "$dl_dir"
		mkdir -p "$dl_dir"

		az_args=(artifacts universal download
			--organization "https://dev.azure.com/${AZ_ORG}"
			--feed "$AZ_FEED"
			--name "$package"
			--version "$version"
			--path "$dl_dir")
		if [ -n "$AZ_PROJECT" ]; then
			az_args+=(--project "$AZ_PROJECT" --scope project)
		fi
		if ! az "${az_args[@]}"; then
			echo "error: az download failed for ${package}@${version} (${GOOS}/${GOARCH})" >&2
			exit 1
		fi

		downloaded=$(find "$dl_dir" -maxdepth 1 -type f)
		if [ -z "$downloaded" ] || [ "$(printf '%s\n' "$downloaded" | wc -l)" -ne 1 ]; then
			echo "error: package ${package}@${version} did not contain exactly one file" >&2
			exit 1
		fi
		mv "$downloaded" "$EMBED_DIR/$file"
		rm -rf "$dl_dir"
		chmod 0755 "$EMBED_DIR/$file"

		entrypoint="$file"
		manifest_entries+=("{\"name\":\"${name}\",\"version\":\"${version}\",\"file\":\"${file}\",\"entrypoint\":\"${entrypoint}\"}")
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
