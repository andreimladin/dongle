# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`dongle` is a **proof-of-concept** host CLI that other CLIs plug into as one-shot
subprocess plugins. The host (`cmd/dongle`) execs `dongle-<name>` as a short-lived
child process, brokering credentials in so each plugin never implements its own
login flow. There is no gRPC and no persistent daemon — every plugin invocation is
a single `exec` that inherits the terminal and exits.

Stdlib-only, zero external dependencies (see `go.mod` — no `require` block).

## Commands

```sh
go build ./...                                   # build everything (host + sample plugin)
go build -o dist/dongle ./cmd/dongle              # build just the host
go build -o dist/dongle-deploy ./plugins/deploy   # build just the sample plugin
go vet ./...                                      # static checks
sh demo.sh                                        # full end-to-end walkthrough
```

There are no `*_test.go` files in the repo yet; `go test ./...` currently reports
no tests to run.

`demo.sh` builds the host and the sample `deploy` plugin, stages a plugin
"release" dir (binary + `plugin.json`), then runs through
install → run-before-login (expected failure) → login → run → uninstall. It sets
`DONGLE_DATA_DIR=$PWD/dist/dongle-home` so it never touches a real
`~/.local/share/dongle`. Use the same env var when manually exercising the CLI to
avoid polluting your real plugin state.

## Architecture

### Request flow

`cmd/dongle/main.go` has a hand-rolled builtin switch (`version`, `login`,
`logout`, `plugin`). Anything else is treated as a plugin name and handed to
`internal/dispatch.Run`, which:

1. Loads the installed-plugin registry (`internal/state`) and looks up the
   plugin's active version.
2. Loads that version's `plugin.json` (`internal/manifest`).
3. Checks compatibility: the plugin's `requires.host` semver constraint against
   `hostVersion`, and `requires.protocol` against the protocol this host speaks.
   Both constants are defined in `cmd/dongle/main.go`.
4. Brokers credentials via `internal/auth.Prepare`, which reads each
   `auth[]` entry in the manifest and either sets an env var (`injectAs:"env"`)
   or writes a per-invocation 0600 temp JSON file (`injectAs:"file"`, the
   default) referenced by `DONGLE_CREDENTIALS_FILE`.
5. `exec.Command`s the plugin binary with stdin/stdout/stderr inherited, the
   stable `DONGLE_*` context env vars, and the brokered credential env/file, then
   propagates its exit code. Credential files are deleted via `defer
   inj.Cleanup()` regardless of outcome.

### The connector spec (host↔plugin contract)

This is the interface every plugin — in any language — must honor. It's
described in the README and implemented on the plugin side by
`pkg/pluginsdk` (Go convenience only; not required):

- **argv**: everything after the plugin name.
- **env**: `DONGLE_VERSION`, `DONGLE_PROTOCOL`, `DONGLE_PLUGIN_NAME`,
  `DONGLE_INVOCATION_ID`, `DONGLE_OUTPUT`, plus any `injectAs:"env"` credentials
  (custom var name from `plugin.json`, or `DONGLE_TOKEN_<provider>` by default).
- **`DONGLE_CREDENTIALS_FILE`**: path to a 0600 JSON file `{"tokens":{...}}` for
  `injectAs:"file"` credentials; deleted by the host after the plugin exits —
  don't rely on it persisting.
- **stdin/stdout/stderr/TTY**: inherited directly from the host process.
- **exit code**: propagated verbatim to the host's caller.

When adding or changing anything in `internal/dispatch` or `internal/auth`,
keep this contract in sync with the README and `pkg/pluginsdk`.

### Package layout

| package | responsibility |
|---|---|
| `cmd/dongle` | host entry point; builtin command switch; owns `hostVersion`/`protocol` constants |
| `internal/manifest` | `plugin.json` schema (`Manifest`, `Requires`, `AuthEntry`, `Command`) + `Load` |
| `internal/state` | installed-plugin registry (`state.json`) + on-disk path helpers (XDG layout) |
| `internal/auth` | credential store (`credentials.json`), `Login`/`Logout`, and `Prepare` (the injection broker) |
| `internal/dispatch` | resolves a command name to a plugin, compat-checks, brokers auth, execs — the core request path |
| `internal/plugincmd` | `dongle plugin list/install/uninstall` |
| `pkg/pluginsdk` | optional helper library for Go-authored plugins (`LoadContext`, `Token`, `Main`) |
| `plugins/deploy` | sample plugin demonstrating both `injectAs` modes, using `pluginsdk` |

### On-disk layout (XDG, overridable via `DONGLE_DATA_DIR`)

```
$XDG_DATA_HOME/dongle/
  plugins/<name>/<version>/{<entrypoint binary>, plugin.json}
  state.json           # internal/state — installed-plugin registry
  credentials.json     # internal/auth  — stored tokens, 0600
```

### Three version axes

Every plugin binds to: the host's own semver (`hostVersion` in
`cmd/dongle/main.go`), the plugin's own semver (`plugin.json`'s `version`), and a
slow-moving protocol version (`protocol` in `cmd/dongle/main.go` /
`requires.protocol` in the manifest). `internal/dispatch` checks both
`requires.host` and `requires.protocol` before exec-ing a plugin.
`internal/dispatch.satisfiesHost` only supports `""` (any), `>=x.y.z`, and exact
`x.y.z` constraints — no `^`/`~`/`<`/`||` ranges.

## Known scaffold swap points

This is intentionally a skeleton; several pieces are stubbed and documented as
such at the call site. Don't over-engineer around them unless asked — they're
meant to be swapped wholesale:

- **Manifests are JSON, not YAML** (`internal/manifest`) — the design intent is
  YAML; JSON was chosen to avoid an external dependency.
- **Credential store is a plaintext JSON file** (`internal/auth`), not an OS
  keychain. `Login` mints a fake `demo-token-*` string rather than doing a real
  OAuth/token flow.
- **Builtins are a hand-rolled switch** in `cmd/dongle/main.go`, not cobra.
- **`plugin install` copies from a local directory** (`internal/plugincmd.install`)
  rather than resolving against a real package index/registry.
- **No mid-run token refresh** — credentials are brokered once at exec time.

## Conventions

- Every exported package has a doc comment on the package itself explaining its
  role in the host↔plugin contract — read those first when touching a package.
- Error handling in `cmd`/`internal` follows a consistent pattern: print `error:
  <context>: <err>` to stderr and return a nonzero int exit code (not `panic` or
  `log.Fatal`), since `main` propagates these ints via `os.Exit`.
- The module path (`github.com/andreimladin/dongle`) is a placeholder per the
  README; don't assume it's the final published path.
