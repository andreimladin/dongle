#!/usr/bin/env bash
# Plugin install now resolves only from the index, and the feed download
# (internal/plugincmd.downloadArtifact) is still a stub — so `dongle plugin
# install <name>` can't complete end-to-end yet. This demo is trimmed to the
# parts that work today: build, `dongle version`, and (if DONGLE_INDEX_ORG or
# similar is configured) `dongle refresh` / `dongle plugin search`.
set -eu
cd "$(dirname "$0")"

go build -o dist/dongle ./cmd

export DONGLE_DATA_DIR="$PWD/dist/home"
rm -rf "$DONGLE_DATA_DIR"

echo "== version =="
./dist/dongle version

if [ -n "${DONGLE_INDEX_ORG:-}" ]; then
	echo; echo "== refresh =="
	./dist/dongle refresh

	echo; echo "== version (with index cached) =="
	./dist/dongle version

	echo; echo "== plugin search =="
	./dist/dongle plugin search
else
	echo; echo "(DONGLE_INDEX_ORG not set — skipping refresh/plugin search)"
fi
