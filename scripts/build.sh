#!/usr/bin/env bash
# The one release build script, function-based and dispatched by its first
# argument:
#
#   ./scripts/build.sh fetch_embedded <os> <arch>   # feed access, no compiling
#   ./scripts/build.sh build_binary   <os> <arch>   # compiles, no feed access
#   ./scripts/build.sh build_target   <os> <arch>   # fetch_embedded + build_binary
#
# Release ALWAYS embeds — there is no no-embed release path and no --embed
# flag. The only no-embed build is a plain `go build ./cmd` (used by CI/dev,
# never published; see azure-pipelines-ci.yml).
#
# fetch_embedded stages the embedded-defaults payload for ONE target
# platform into internal/bootstrap/embedded/, ready for build_binary to
# `-tags embed` into a binary. Needs `az` credentials for the shared feed
# (see azure-pipelines-release.yml, which runs this once per matrix leg).
# configs/build.yaml (see its own header comment) is the single source of
# truth for the index feed identity and the embedded plugin list — this
# script hardcodes neither: it shells out to tools/readconfig for both. It
# downloads the latest "dongle-index" Universal Package fresh into a temp
# dir every run (via `az artifacts universal download --version "*"`, then
# extracts it — stdlib `tar`), using the same INDEX_ORG/INDEX_PROJECT/
# INDEX_FEED/INDEX_PACKAGE that build_binary also bakes into the binary via
# -ldflags, so the binary and the index its embedded plugins were resolved
# against always agree. That same downloaded archive is ALSO copied verbatim
# into internal/bootstrap/embedded/index.tar.gz (its VERSION file's value
# recorded in manifest.json's "index" field), so build_binary's -tags embed
# bakes a seed catalog into the binary too — internal/index extracts it into
# the cache on a first run with nothing cached yet, so a released dongle has
# a working plugin index from its very first run, fully offline (see
# internal/index.EnsureFresh's embedded-seed precedence). For each embedded
# plugin name it shells out to tools/resolve-plugin (also build-time-only,
# not a dongle subcommand) to read that plugin's Azure Artifacts feed
# coordinates, per-platform package name, and version straight out of its
# index manifest (plugins/<name>.yaml) — parsed by the exact same
# internal/index code `dongle plugin install` uses, not reimplemented — then
# downloads it via `az artifacts universal download`. Fails fast, naming the
# plugin and platform, if a plugin has no published build for <os>/<arch>
# rather than silently staging a binary with that default missing.
#
# build_binary compiles ONE target's binary from whatever is already staged
# in internal/bootstrap/embedded/ (run fetch_embedded first, or use
# build_target). No feed access here: this function only builds, it never
# touches `az` or the plugin index. hostVersion comes from outside this
# script: the pipeline sets DONGLE_VERSION (required in CI); locally it
# falls back to `git describe`, then "dev". The index feed identity comes
# from configs/build.yaml (via tools/readconfig) and is baked in via
# -ldflags alongside hostVersion, so the resulting binary points at the
# same feed its embedded defaults were actually resolved against.
#
# build_target is pure composition (fetch_embedded then build_binary) for a
# local one-shot build; build_binary itself never calls fetch_embedded.
#
# tools/readconfig and tools/resolve-plugin are built once, up front, to
# temp binaries rather than `go run` on every invocation inside the loop.
#
# Requires: az (logged in to the shared feed) for fetch_embedded; just Go
# for build_binary. git is no longer needed for either (fetch_embedded
# downloads the index from the feed, not a git clone) — build_binary's
# local hostVersion fallback still shells out to `git describe` if
# DONGLE_VERSION isn't set, but that's unrelated to the index and optional
# (falls back further to "dev" if git isn't available either).
set -euo pipefail
cd "$(dirname "$0")/.."

EMBED_DIR="internal/bootstrap/embedded"

TOOLBIN=$(mktemp -d)
FETCH_TMP="" # set by fetch_embedded; cleaned up by cleanup() below regardless of how the script exits
cleanup() {
	# Capture and re-assert the real exit status: an EXIT trap's own exit
	# status otherwise silently replaces the script's actual one (e.g. a
	# false `[ -n "$FETCH_TMP" ]` test here would turn `exit 2` into
	# exit 1).
	local status=$?
	rm -rf "$TOOLBIN"
	if [ -n "$FETCH_TMP" ]; then
		rm -rf "$FETCH_TMP"
	fi
	exit "$status"
}
trap cleanup EXIT

build_tools() {
	go build -o "$TOOLBIN/readconfig" ./tools/readconfig
	go build -o "$TOOLBIN/resolve-plugin" ./tools/resolve-plugin
}

# clean_embed wipes any previously staged plugin payload but keeps the
# tracked .gitkeep, so the dir stays non-empty for go:embed.
clean_embed() {
	find "$EMBED_DIR" -mindepth 1 ! -name '.gitkeep' -exec rm -rf {} +
}

fetch_embedded() {
	if [ $# -ne 2 ]; then
		echo "usage: $0 fetch_embedded <os> <arch>" >&2
		exit 2
	fi
	local goos="$1" goarch="$2"

	if ! command -v az >/dev/null 2>&1; then
		echo "error: az is required (see script header)" >&2
		exit 1
	fi

	eval "$("$TOOLBIN/readconfig" --index)" # INDEX_ORG, INDEX_PROJECT, INDEX_FEED, INDEX_PACKAGE

	FETCH_TMP=$(mktemp -d)
	local tmp="$FETCH_TMP"

	echo "downloading index $INDEX_PACKAGE from feed '$INDEX_FEED' (org $INDEX_ORG)..."
	local index_dl_dir="$tmp/download"
	mkdir -p "$index_dl_dir"
	local index_az_args=(artifacts universal download
		--organization "https://dev.azure.com/${INDEX_ORG}"
		--feed "$INDEX_FEED"
		--name "$INDEX_PACKAGE"
		--version "*"
		--path "$index_dl_dir")
	if [ -n "$INDEX_PROJECT" ]; then
		index_az_args+=(--project "$INDEX_PROJECT" --scope project)
	fi
	if ! az "${index_az_args[@]}"; then
		echo "error: could not download index package $INDEX_PACKAGE from feed $INDEX_FEED" >&2
		exit 1
	fi

	local index_archive
	index_archive=$(find "$index_dl_dir" -maxdepth 1 -type f)
	if [ -z "$index_archive" ] || [ "$(printf '%s\n' "$index_archive" | wc -l)" -ne 1 ]; then
		echo "error: index package $INDEX_PACKAGE did not contain exactly one file" >&2
		exit 1
	fi

	mkdir -p "$tmp/index"
	tar -xzf "$index_archive" -C "$tmp/index"

	local index_version=""
	if [ -f "$tmp/index/VERSION" ]; then
		index_version=$(tr -d '[:space:]' <"$tmp/index/VERSION")
	fi
	if [ -z "$index_version" ]; then
		echo "error: downloaded index archive $index_archive has no VERSION file" >&2
		exit 1
	fi

	clean_embed

	# Stage the index archive itself alongside the plugin defaults so
	# build_binary's -tags embed picks it up too (see
	# internal/bootstrap.EmbeddedIndex): a released dongle then has a
	# working plugin catalog from its very first run, fully offline (see
	# internal/index's embedded-seed precedence). manifest.json's "index"
	# field records the version so internal/index and `dongle version` can
	# tell it apart from one later fetched from the feed.
	cp "$index_archive" "$EMBED_DIR/index.tar.gz"
	echo "  staged embedded index (version $index_version)"

	echo "== staging embedded defaults for $goos/$goarch =="

	local manifest_entries=()
	local name resolved
	while IFS= read -r name; do
		[ -n "$name" ] || continue

		echo "  [$name] resolving feed coordinates + version for $goos/$goarch..."
		if ! resolved=$("$TOOLBIN/resolve-plugin" "$name" "$goos" "$goarch" --index "$tmp/index"); then
			echo "error: [$name] has no published build for $goos/$goarch — cannot embed defaults for this target" >&2
			exit 1
		fi
		eval "$resolved"

		echo "  [$name] downloading $PACKAGE@$VERSION from feed '$FEED' (org $ORG)..."

		local dl_dir="$EMBED_DIR/.download"
		rm -rf "$dl_dir"
		mkdir -p "$dl_dir"

		local az_args=(artifacts universal download
			--organization "https://dev.azure.com/${ORG}"
			--feed "$FEED"
			--name "$PACKAGE"
			--version "$VERSION"
			--path "$dl_dir")
		if [ -n "$PROJECT" ]; then
			az_args+=(--project "$PROJECT" --scope project)
		fi
		if ! az "${az_args[@]}"; then
			echo "error: az download failed for $PACKAGE@$VERSION ($goos/$goarch) — plugin [$name] may not be published for this platform" >&2
			exit 1
		fi

		local downloaded
		downloaded=$(find "$dl_dir" -maxdepth 1 -type f)
		if [ -z "$downloaded" ] || [ "$(printf '%s\n' "$downloaded" | wc -l)" -ne 1 ]; then
			echo "error: package $PACKAGE@$VERSION did not contain exactly one file" >&2
			exit 1
		fi

		# Canonicalize to the same entrypoint naming `dongle plugin install`
		# uses: <host binary name>-<plugin name>.
		local file="dongle-${name}"
		[ "$goos" = "windows" ] && file="${file}.exe"
		mv "$downloaded" "$EMBED_DIR/$file"
		rm -rf "$dl_dir"
		chmod 0755 "$EMBED_DIR/$file"

		manifest_entries+=("{\"name\":\"${name}\",\"version\":\"${VERSION}\",\"file\":\"${file}\",\"entrypoint\":\"${file}\"}")
		echo "  [$name] staged as $file (version $VERSION)"
	done < <("$TOOLBIN/readconfig" --embedded)

	{
		printf '{\n'
		printf '  "plugins": [\n'
		local last=$((${#manifest_entries[@]} - 1))
		local i
		for i in "${!manifest_entries[@]}"; do
			printf '    %s' "${manifest_entries[$i]}"
			[ "$i" -lt "$last" ] && printf ','
			printf '\n'
		done
		printf '  ],\n'
		printf '  "index": {"version": "%s"}\n' "$index_version"
		printf '}\n'
	} >"$EMBED_DIR/manifest.json"

	rm -rf "$tmp"
	FETCH_TMP=""

	echo "done: embedded defaults staged for $goos/$goarch"
}

build_binary() {
	if [ $# -ne 2 ]; then
		echo "usage: $0 build_binary <os> <arch>" >&2
		exit 2
	fi
	local goos="$1" goarch="$2"

	# Version is provided by the pipeline via DONGLE_VERSION.
	# In CI it is REQUIRED; locally it falls back to git, then "dev".
	if [ -n "${CI:-}${TF_BUILD:-}${GITHUB_ACTIONS:-}" ] && [ -z "${DONGLE_VERSION:-}" ]; then
		echo "error: DONGLE_VERSION must be set in CI" >&2
		exit 1
	fi
	local version="${DONGLE_VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"

	eval "$("$TOOLBIN/readconfig" --index)" # INDEX_ORG, INDEX_PROJECT, INDEX_FEED, INDEX_PACKAGE

	local out="dist/dongle-${goos}-${goarch}"
	[ "$goos" = "windows" ] && out="${out}.exe"

	echo "building ${out} (version $version)..."
	GOOS="$goos" GOARCH="$goarch" go build -tags embed \
		-ldflags "-s -w \
			-X main.hostVersion=$version \
			-X main.indexOrg=$INDEX_ORG \
			-X main.indexProject=$INDEX_PROJECT \
			-X main.indexFeed=$INDEX_FEED \
			-X main.indexPackage=$INDEX_PACKAGE" \
		-o "$out" ./cmd

	echo "done: ${out}"
}

# build_target composes fetch_embedded + build_binary for a local one-shot
# build. build_binary itself must never call fetch_embedded.
build_target() {
	if [ $# -ne 2 ]; then
		echo "usage: $0 build_target <os> <arch>" >&2
		exit 2
	fi
	fetch_embedded "$@"
	build_binary "$@"
}

if [ $# -lt 1 ]; then
	echo "usage: $0 <fetch_embedded|build_binary|build_target> <os> <arch>" >&2
	exit 2
fi
cmd="$1"
shift
case "$cmd" in
fetch_embedded | build_binary | build_target) ;;
*)
	echo "error: unknown command '$cmd' (want fetch_embedded, build_binary, or build_target)" >&2
	echo "usage: $0 <fetch_embedded|build_binary|build_target> <os> <arch>" >&2
	exit 2
	;;
esac

build_tools
"$cmd" "$@"
