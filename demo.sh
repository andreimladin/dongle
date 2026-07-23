#!/usr/bin/env bash
# Walkthrough of the local-dir install path + credential broker, using a bash
# plugin so no external deps are needed. (The index/feed install path is stubbed
# at downloadArtifact — see internal/plugincmd. The cobra plugin lives in
# examples/dongle-deploy and needs its own `go mod tidy`.)
set -eu
cd "$(dirname "$0")"

go build -o dist/dongle ./cmd/dongle

# Stage a release dir: a bash "plugin" + its plugin.json.
mkdir -p dist/rel
cat > dist/rel/dongle-hello <<'EOF'
#!/usr/bin/env bash
echo "hello from $DONGLE_PLUGIN_NAME v$DONGLE_VERSION (invocation $DONGLE_INVOCATION_ID)"
echo "args: $*"
if [ -n "${DONGLE_CREDENTIALS_FILE:-}" ]; then
  echo "brokered creds file: $DONGLE_CREDENTIALS_FILE"
  cat "$DONGLE_CREDENTIALS_FILE"; echo
fi
EOF
chmod +x dist/rel/dongle-hello
cat > dist/rel/plugin.json <<'EOF'
{ "name": "hello", "version": "1.0.0", "description": "say hi",
  "entrypoint": "dongle-hello",
  "requires": { "host": ">=0.1.0", "protocol": "v1" },
  "auth": [ { "provider": "platform-oidc", "injectAs": "file" } ] }
EOF

export DONGLE_DATA_DIR="$PWD/dist/home"
rm -rf "$DONGLE_DATA_DIR"

echo "== version =="
./dist/dongle version

echo; echo "== install from local dir =="
./dist/dongle plugin install ./dist/rel

echo; echo "== list installed =="
./dist/dongle plugin list

echo; echo "== run BEFORE login (broker fails: no credential) =="
./dist/dongle hello there || echo "(exit $? — as expected)"

echo; echo "== login, then run (host brokers the credential into the plugin) =="
./dist/dongle login platform-oidc
./dist/dongle hello there --loud

echo; echo "== uninstall =="
./dist/dongle plugin uninstall hello
./dist/dongle plugin list
