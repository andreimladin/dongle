# 🔌 dongle

> One CLI. Plug in the rest.

**dongle** is a host CLI that your other CLIs plug into. Each one is a **one-shot
plugin** you plug in, version, and unplug independently — one connector spec, any
language, no background services.

This repo is the hand-built version, assembled step by step. It compiles and
runs end to end via the **local-dir install path**; the **index/feed install
path** is fully wired except for the one Azure-feed download call, which is a
clearly marked stub. Authentication is intentionally **not built yet** — it's the
next area of work.

## Build & run

The host has a single external dependency (`gopkg.in/yaml.v3`, for the index),
so run `go mod tidy` once (needs network), then build:

```sh
go mod tidy
go build ./...      # builds the host (examples/ is a separate module)
sh demo.sh          # local-dir install lifecycle, end to end
```

`demo.sh` isolates state under `./dist` via `DONGLE_DATA_DIR`.

### Three ways to build the host

| command | binary | plugins |
|---|---|---|
| `go build ./cmd` | dev binary, `hostVersion` defaults to `"dev"` | none — install them the normal way |
| `scripts/build.sh` | `dist/dongle`, `hostVersion` stamped from `git describe` | none |
| `scripts/build-release.sh` | `dist/dongle-<os>-<arch>` per platform | embedded defaults from `defaults.lock` |

`go build ./cmd` and `scripts/build.sh` are always available and require
nothing beyond the Go toolchain — embedding is entirely opt-in and behind a
build tag, so plain builds have no new behavior. `scripts/build-release.sh`
is a release maintainer's manual step and is **not** run in CI: it needs
`az` (logged in to the feed) and `git`, on top of the Go toolchain — nothing
else, since feed-coordinate resolution goes through `tools/resolve` (see
below) rather than a separate YAML tool.

### Embedded default plugins (`embed` build tag)

`defaults.lock` (repo root) is the reviewable, diffable list of which
plugins ship baked into a release binary:

```
# name  version
tacho   2.4.0
bell    1.1.0
```

`scripts/build-release.sh` clones the plugin index fresh into a temp dir on
every run (`DONGLE_INDEX_URL`/`DONGLE_INDEX_BRANCH` override it, same as the
CLI), then builds `tools/resolve` — a small build-time-only Go helper in
this repo, not a `dongle` subcommand — into that temp dir. For each
`defaults.lock` entry it shells out to it: `resolve <name> --version <v>
--os <os> --arch <arch> --index <indexdir>` reads `plugins/<name>.yaml`
directly from the freshly cloned checkout (no cache, no network) and prints
the plugin's Azure Artifacts feed coordinates and per-platform package name
as `eval`-able `KEY="value"` lines. Nothing about the feed — organization,
feed name, project, or package naming — is hardcoded in the script itself,
and the manifest is parsed by the exact same `internal/index` code (plus one
purely-additive `LoadFile` helper for reading from an arbitrary path) that
the CLI uses for `dongle plugin install` — not reimplemented. The script
then downloads each plugin's binary for the target platform into
`cmd/embedded/` (git-ignored except for the tracked `cmd/embedded/.gitkeep`
placeholder), writes `cmd/embedded/manifest.json`, and builds with
`-tags embed` so `cmd/embed.go`'s `//go:embed all:embedded` picks the staged
files up into the binary. On first run, `installEmbeddedDefaults()` unpacks
them into the normal plugin store (`plugins/<name>/<version>/<entrypoint>`)
and sets a `defaultsBootstrapped` flag in `state.json` so it never runs
again — from then on those plugins behave exactly like ones installed via
`dongle plugin install`.

A binary built without `-tags embed` (i.e. every binary except the ones
`scripts/build-release.sh` produces) links `cmd/embed_noop.go` instead, whose
`installEmbeddedDefaults()` is a no-op — no embed dependency, no behavior
change, nothing staged.

## What works vs. what's stubbed

Real and testable now:

- Plugin **dispatch** — unknown command → resolve via `state.json` (entrypoint +
  requires recorded there at install time, no per-plugin manifest on disk) →
  compat gate → exec one-shot child, inheriting the terminal.
- **install / uninstall / list** from a local build dir; **search** from the
  index cache.
- **Compatibility gates** (`requires.host` range + `requires.protocol` exact) at
  both install time and dispatch time, from the shared `internal/compat`.
- **Embedded git index**: `dongle index refresh|status`, 24h TTL cache,
  offline-tolerant refresh, `plugin search`, and install-by-name resolution up
  to the download.

Stubbed (the seam is in place):

- `internal/plugincmd.downloadArtifact` calls the real `az artifacts universal
  download`; a REST-based implementation (no `az` dependency) is future work.

Not built yet (future work): authentication (a credential store, `login`, and
brokering credentials into plugins).

## Layout

```
cmd/                    host entry (cobra): main.go, root.go (root command + plugin
                        dispatch fall-through), plugin.go, index.go, embed.go /
                        embed_noop.go (embedded default plugins, see below)
internal/compat/       semver + host/protocol gate (single source of truth)
internal/state/        installed-plugin registry (entrypoint + requires) + on-disk paths
internal/dispatch/     resolve -> compat -> exec
internal/plugincmd/    plugin list/search/install/uninstall (+ index resolver)
internal/index/        embedded git catalog: clone/TTL-pull cache, lookups
tools/resolve/          build-time-only helper for scripts/build-release.sh
                        (not a dongle subcommand) — see "Embedded default
                        plugins" above
examples/dongle-deploy/  sample cobra plugin (its own module)
examples/index/          sample index-repo manifest (Azure feed coordinates)
```

## The host↔plugin contract (language-agnostic)

The host execs `dongle-<name> <args...>` with:

- **argv** — everything after the plugin name.
- **env** — `DONGLE_VERSION`, `DONGLE_PROTOCOL`, `DONGLE_PLUGIN_NAME`.
- **stdin/stdout/stderr/TTY inherited** — prompts and colors just work.
- **exit code** — propagated.

An existing **cobra** CLI becomes a plugin by setting its root command's `Use` to
the plugin name and adding a manifest to the index (naming its per-platform
binary and `requires`); its whole subcommand tree keeps working because
dispatch hands args straight to cobra. It needs nothing from dongle — the
context arrives as plain env vars. See `examples/dongle-deploy`.

## Three version axes

Bound by each plugin's manifest `requires`: the host's semver (`hostVersion`
in `cmd/root.go`), each plugin's semver, and a slow-moving protocol
version. Host and protocol are separate constants so they release
independently.

## Publishing a plugin (git index + shared Azure feed)

1. Build the plugin binary for each `os/arch` you support.
2. Publish each platform's binary as its own Universal Package to the shared
   Azure Artifacts feed: `az artifacts universal publish --feed dongle-plugins
   --name dongle-deploy_2.3.1_linux_amd64 --version 2.3.1 --path ./dist/linux_amd64`.
   The package name is a plugin-publisher convention — dongle doesn't parse
   it — but a name that encodes plugin/version/os/arch keeps feed browsing
   sane. Each package must contain exactly one file: the binary itself.
3. PR `plugins/<name>.yaml` to the index repo (version + feed coords + each
   platform's `selector` and the Universal Package `name` you published it
   under as `package`). See `examples/index/plugins/deploy.yaml`. At install
   time dongle downloads that package, takes the single file inside it, and
   installs it under a canonical entrypoint (`dongle-<name>`) — the file's own
   name inside the package doesn't matter.

If the Azure Artifacts feed is scoped to a project (rather than organization-wide),
set `feed.project` in the index manifest — `downloadArtifact` passes `--project`
and `--scope project` to `az artifacts universal download` when it's set.
Org-scoped feeds omit `feed.project` entirely.

Set the embedded index URL in `internal/index/index.go` (`IndexURL`).
`DONGLE_INDEX_URL` overrides it for dev.

## Index access

The plugin index is a private Azure DevOps git repo, cloned over HTTPS.
`dongle` does not manage credentials for it — it shells out to `git`, which
authenticates using whatever git auth is already set up on your machine, via
Git Credential Manager (GCM).

One-time setup:

1. Install GCM — it's bundled with Git for Windows; on macOS run `brew install
   --cask git-credential-manager`; on Linux see the [GCM install
   docs](https://github.com/git-ecosystem/git-credential-manager/blob/main/docs/install.md).
2. Run `dongle index refresh` (or any `git clone`/`git pull` of the index
   URL). The first time, GCM opens a browser to sign in with your company
   Entra ID. After that, GCM caches the token and refreshes it silently —
   nothing for you to rotate or maintain.

Dev overrides: `DONGLE_INDEX_URL` points at a different index repo,
`DONGLE_INDEX_BRANCH` pins a different branch (both default to the embedded
`IndexURL`/`IndexBranch` in `internal/index/index.go`).

## Before you publish this repo

- Replace `andreimladin` with your GitHub/module path everywhere:
  `grep -rl andreimladin . | xargs sed -i 's/andreimladin/<you>/g'`
- Set `IndexURL` and `IndexBranch` in `internal/index/index.go`.
- Fill in the `LICENSE` year/name.

## License

MIT — see [LICENSE](LICENSE).
