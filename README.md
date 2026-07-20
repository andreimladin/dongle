# 🔌 dongle

> One CLI. Plug in the rest.

**dongle** is a host CLI that your other CLIs plug into. Each one is a
**one-shot plugin** you plug in, version, and unplug independently — one
connector spec, any language, no background services. The host is the single
front door for authentication, so it feels like one tool even when each plugin
authenticates differently.

> ⚠️ Proof of concept. Stdlib-only, no external dependencies.

```sh
dongle plugin install deploy     # plug a CLI in
dongle deploy run billing        # use it (host brokers its credentials)
dongle plugin uninstall deploy   # unplug it
```

## Run it

```sh
go build ./...     # builds host + sample plugin, no `go get` needed
sh demo.sh         # full walkthrough: build → install → login → run → unplug
```

`demo.sh` isolates state under `./dist` via `DONGLE_DATA_DIR`, so it won't touch
your real `~/.local/share/dongle`.

## How it works

The host execs `dongle-<name> <args...>` as a short-lived child. Everything the
plugin needs arrives through a language-agnostic **connector spec**:

- **argv** — everything after the plugin name.
- **env** — `DONGLE_VERSION`, `DONGLE_PROTOCOL`, `DONGLE_PLUGIN_NAME`,
  `DONGLE_INVOCATION_ID`, `DONGLE_OUTPUT`, plus any `injectAs:"env"` credentials.
- **`DONGLE_CREDENTIALS_FILE`** — path to a 0600 JSON file `{"tokens":{...}}` for
  `injectAs:"file"` credentials; deleted after the plugin exits.
- **stdin/stdout/stderr/TTY inherited** — prompts and colors just work.
- **exit code** — propagated by the host.

Any language that can read env + a file and return an exit code is a valid
plugin. No gRPC, no persistent services.

## Vocabulary

| concept | in dongle |
|---|---|
| a plugin | a **device** you plug in |
| install / uninstall | **plug in** / **unplug** |
| host↔plugin contract | the **connector spec** |
| installed set | **what's plugged in** |

## Layout

```
cmd/dongle/          host entry; builtin command switch
internal/manifest/   plugin.json schema + loader
internal/state/      installed-plugin registry + on-disk paths
internal/auth/       credential store, login, injection (the broker)
internal/dispatch/   resolver: compat check -> auth -> exec (the core)
internal/plugincmd/  `dongle plugin list|install|uninstall`
pkg/pluginsdk/       helpers for Go plugin authors
plugins/deploy/      sample plugin + its manifest
```

## Three version axes

Bound by each manifest's `requires`: the host's semver, each plugin's semver,
and a slow-moving **protocol version**. The host checks both `requires.host` and
`requires.protocol` before it execs a plugin.

## This is a skeleton — the swap points

1. **Manifests → YAML**: import `gopkg.in/yaml.v3`, swap the `json.Unmarshal` in
   `internal/manifest`.
2. **Builtins → cobra**: replace the switch in `cmd/dongle/main.go`.
3. **Credential store → OS keychain**: replace `readStore`/`save` in
   `internal/auth` with Keychain / Credential Manager / libsecret; replace
   `Login`'s stub with a real flow.
4. **install → real registry**: point `internal/plugincmd.install` at a
   krew-style index, download the per-platform artifact, verify its sha256.
5. **semver**: `internal/dispatch` handles `>=`, `<=`, `>`, `<`, `^`, `~`, and
   exact match; drop in `github.com/Masterminds/semver` if you need `||`
   combinators or pre-release-aware ordering.
6. **mid-run token refresh** (optional): host opens a local unix socket, passes
   its path via env, plugin requests fresh scoped tokens as a short-lived
   client. Still one-shot, still local.

## Publishing this

The module path is a placeholder. Before you push, replace `andreimladin` with your
GitHub username everywhere:

```sh
grep -rl 'andreimladin' . | xargs sed -i 's/andreimladin/<your-github-username>/g'
```

(It builds locally either way — Go only fetches remote modules for *external*
imports, and there are none.)

## License

MIT — see [LICENSE](LICENSE).
