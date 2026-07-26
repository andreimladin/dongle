#!/usr/bin/env bash
# Walkthrough of the local-dir install path, using a bash plugin so no external
# deps are needed. (The index/feed install path is stubbed at downloadArtifact —
# see internal/plugincmd.)
set -eu
cd "$(dirname "$0")"

go build -o dist/dongle ./cmd/dongle

# Stage a release dir: a bash "plugin" + its plugin.json.
mkdir -p dist/rel
cat > dist/rel/dongle-hello <<'PLUGIN'
#!/usr/bin/env bash
echo "hello from $DONGLE_PLUGIN_NAME v$DONGLE_VERSION (protocol $DONGLE_PROTOCOL)"
echo "args: $*"
PLUGIN
chmod +x dist/rel/dongle-hello
cat > dist/rel/plugin.json <<'MANIFEST'
{ "name": "hello", "version": "1.0.0", "description": "say hi",
  "entrypoint": "dongle-hello",
  "requires": { "host": ">=0.1.0", "protocol": "v1" } }
MANIFEST

export DONGLE_DATA_DIR="$PWD/dist/home"
rm -rf "$DONGLE_DATA_DIR"

echo "== version =="
./dist/dongle version

echo; echo "== install from local dir =="
./dist/dongle plugin install ./dist/rel

echo; echo "== list installed =="
./dist/dongle plugin list

echo; echo "== run the plugin (host injects the context contract) =="
./dist/dongle hello there --loud

echo; echo "== uninstall =="
./dist/dongle plugin uninstall hello
./dist/dongle plugin list
