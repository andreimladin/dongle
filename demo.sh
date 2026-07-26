#!/usr/bin/env bash
# Walkthrough of the local-dir install path, using a bash plugin so no external
# deps are needed.
#
# This repo stops at "5a": the credential STORE (login/logout) exists, but
# credential INJECTION into the plugin process is the next step and isn't wired
# in yet. So `login` fills the store, but plugins don't yet receive credentials.
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

echo; echo "== credential store: login fills it, logout clears it =="
./dist/dongle login platform-oidc
cat "$DONGLE_DATA_DIR/credentials.json"; echo
./dist/dongle logout platform-oidc

echo; echo "== uninstall =="
./dist/dongle plugin uninstall hello
./dist/dongle plugin list
