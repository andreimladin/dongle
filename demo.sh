#!/usr/bin/env bash
# Plugin install now resolves only from the index, and the feed download
# (internal/builtins.downloadArtifact) is still a stub — so `dongle
# install <name>` can't complete end-to-end yet. This demo is trimmed to the
# parts that work today: build, `dongle --version`, and (if INDEX_ORG and
# INDEX_FEED are given) `dongle update` / `dongle search`.
#
# The index feed identity is baked into the binary at build time, the same
# way scripts/build.sh does it — dongle reads no runtime overrides:
#
#   INDEX_ORG=myorg INDEX_FEED=myfeed [INDEX_PROJECT=...] [INDEX_PACKAGE=...] ./demo.sh
set -eu
cd "$(dirname "$0")"

ldflags=""
[ -n "${INDEX_ORG:-}" ] && ldflags="$ldflags -X main.indexOrg=$INDEX_ORG"
[ -n "${INDEX_PROJECT:-}" ] && ldflags="$ldflags -X main.indexProject=$INDEX_PROJECT"
[ -n "${INDEX_FEED:-}" ] && ldflags="$ldflags -X main.indexFeed=$INDEX_FEED"
[ -n "${INDEX_PACKAGE:-}" ] && ldflags="$ldflags -X main.indexPackage=$INDEX_PACKAGE"
go build -ldflags "$ldflags" -o dist/dongle ./cmd

# dongle always keeps its data in ~/.dongle; point HOME at a scratch dir so
# the demo never touches yours. Keep the Azure CLI's login where it was.
export AZURE_CONFIG_DIR="${AZURE_CONFIG_DIR:-$HOME/.azure}"
export HOME="$PWD/dist/home"
rm -rf "$HOME"
mkdir -p "$HOME"

echo "== version =="
./dist/dongle --version

if [ -n "${INDEX_ORG:-}" ] && [ -n "${INDEX_FEED:-}" ]; then
	echo; echo "== update =="
	./dist/dongle update

	echo; echo "== version (with index cached) =="
	./dist/dongle --version

	echo; echo "== search =="
	./dist/dongle search
else
	echo; echo "(INDEX_ORG/INDEX_FEED not set — skipping update/search)"
fi
