#!/usr/bin/env bash
# Plugin install now resolves only from the index, and the feed download
# (internal/plugincmd.downloadArtifact) is still a stub — so `dongle plugin
# install <name>` can't complete end-to-end yet. This demo is trimmed to the
# parts that work today: build, `dongle version`, and (if DONGLE_INDEX_URL or
# similar is configured) `dongle plugin search` / `dongle index status`.
set -eu
cd "$(dirname "$0")"

go build -o dist/dongle ./cmd/dongle

export DONGLE_DATA_DIR="$PWD/dist/home"
rm -rf "$DONGLE_DATA_DIR"

echo "== version =="
./dist/dongle version

if [ -n "${DONGLE_INDEX_URL:-}" ]; then
	echo; echo "== index status =="
	./dist/dongle index status

	echo; echo "== plugin search =="
	./dist/dongle plugin search
else
	echo; echo "(DONGLE_INDEX_URL not set — skipping index status/search)"
fi
