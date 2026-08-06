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

- `internal/plugincmd.downloadArtifact` → implement the **Azure feed** pull
  (`az artifacts universal download` first, REST later). `verifySHA256` and
  `untar` around it are already the real implementations.

Not built yet (future work): authentication (a credential store, `login`, and
brokering credentials into plugins).

## Layout

```
cmd/dongle/            host entry; builtin switch + plugin dispatch
internal/compat/       semver + host/protocol gate (single source of truth)
internal/state/        installed-plugin registry (entrypoint + requires) + on-disk paths
internal/dispatch/     resolve -> compat -> exec
internal/plugincmd/    plugin list/search/install/uninstall (+ index resolver)
internal/index/        embedded git catalog: clone/TTL-pull cache, lookups
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
in `cmd/dongle/main.go`), each plugin's semver, and a slow-moving protocol
version. Host and protocol are separate constants so they release
independently.

## Publishing a plugin (git index + shared Azure feed)

1. Build per-`os/arch` tarballs (just the binary); record each `sha256`.
2. Publish to the shared Azure Artifacts feed as a Universal Package (one package
   per plugin): `az artifacts universal publish --feed dongle-plugins --name
   dongle-deploy --version 2.3.1 --path ./dist`.
3. PR `plugins/<name>.yaml` to the index repo (version + feed coords +
   checksums). See `examples/index/plugins/deploy.yaml`.

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
