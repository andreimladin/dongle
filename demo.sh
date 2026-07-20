#!/usr/bin/env bash
# End-to-end walkthrough of the exec-based plugin flow.
# Requires Go 1.21+. Uses a throwaway data dir so it won't touch your real one.
set -eu
cd "$(dirname "$0")"

# 1. Build the host and the sample plugin (zero external deps).
go build -o dist/dongle ./cmd/dongle
go build -o dist/dongle-deploy ./plugins/deploy

# 2. Stage a "release" dir for the plugin = binary + its manifest.
mkdir -p dist/deploy-release
cp dist/dongle-deploy dist/deploy-release/
cp plugins/deploy/plugin.json dist/deploy-release/

# 3. Isolate state in ./dist so this demo is self-contained.
export DONGLE_DATA_DIR="$PWD/dist/dongle-home"
rm -rf "$DONGLE_DATA_DIR"

echo "== version =="
./dist/dongle version

echo; echo "== install the deploy plugin from a local build dir =="
./dist/dongle plugin install ./dist/deploy-release

echo; echo "== list installed plugins =="
./dist/dongle plugin list

echo; echo "== run BEFORE login (expected to fail: no brokered credential) =="
./dist/dongle deploy run billing || echo "(exit $?  — as expected)"

echo; echo "== log in to the two providers the plugin declares =="
./dist/dongle login platform-oidc
./dist/dongle login registry-token

echo; echo "== run AFTER login (host brokers creds into the one-shot plugin) =="
./dist/dongle deploy run billing

echo; echo "== uninstall =="
./dist/dongle plugin uninstall deploy
./dist/dongle plugin list
