#!/usr/bin/env bash
# fetch-embedded.sh <os> <arch>
#
# Stages the embedded-defaults payload for ONE target platform into
# internal/bootstrap/embedded/, ready for scripts/build-binary.sh to
# `-tags embed` into a binary. Release-only; needs `az` credentials for the
# plugin feed (see azure-pipelines-release.yml, which runs this once per
# matrix leg).
#
# configs/build.yaml (see configs/README.md) is the single source of truth
# for the index coordinates and the embedded plugin list — this script
# hardcodes neither: it shells out to tools/readconfig for both. It clones
# the plugin index fresh into a temp dir every run, using the same
# INDEX_URL/INDEX_BRANCH that scripts/build-binary.sh also bakes into the
# binary via -ldflags, so the binary and the clone plugins were resolved
# against always agree. For each embedded plugin it shells out to
# tools/resolve-plugin (also build-time-only, not a dongle subcommand) to
# read that plugin's Azure Artifacts feed coordinates and per-platform
# package name straight out of its index manifest (plugins/<name>.yaml) —
# parsed by the exact same internal/index code `dongle plugin install`
# uses, not reimplemented — then downloads it via `az artifacts universal
# download`.
#
# Fails fast, naming the plugin and platform, if a plugin has no published
# build for <os>/<arch> rather than silently staging a binary with that
# default missing.
#
# Requires: az (logged in to the plugin feed), git, go.
#
# Usage: ./scripts/fetch-embedded.sh darwin arm64
set -euo pipefail
cd "$(dirname "$0")/.."

if [ $# -ne 2 ]; then
	echo "usage: $0 <os> <arch>" >&2
	exit 2
fi
GOOS="$1"
GOARCH="$2"

EMBED_DIR="internal/bootstrap/embedded"

for bin in az git go; do
	if ! command -v "$bin" >/dev/null 2>&1; then
		echo "error: $bin is required (see script header)" >&2
		exit 1
	fi
done

eval "$(go run ./tools/readconfig --index)" # INDEX_URL, INDEX_BRANCH

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

echo "cloning index $INDEX_URL (branch $INDEX_BRANCH)..."
if ! git clone --depth 1 --branch "$INDEX_BRANCH" "$INDEX_URL" "$TMP/index" >&2; then
	echo "error: could not clone index $INDEX_URL" >&2
	exit 1
fi

# clean_embed wipes any previously staged plugin payload but keeps the
# tracked .gitkeep, so the dir stays non-empty for go:embed.
clean_embed() {
	find "$EMBED_DIR" -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
}
clean_embed

echo "== staging embedded defaults for $GOOS/$GOARCH =="

manifest_entries=()
while IFS=: read -r name version; do
	[ -n "$name" ] || continue

	echo "  [$name] resolving feed coordinates for $GOOS/$GOARCH..."
	if ! resolved=$(go run ./tools/resolve-plugin "$name" "$version" "$GOOS" "$GOARCH" --index "$TMP/index"); then
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
done < <(go run ./tools/readconfig --embedded)

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

echo "done: embedded defaults staged for $GOOS/$GOARCH"
