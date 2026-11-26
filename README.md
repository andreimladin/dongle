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

## Documentation

User docs: [`docs/forge/`](docs/forge/README.md). Plugin-owner docs:
[`index-repo/`](index-repo/README.md). This README covers building and
developing dongle itself.

## Build & run

The host has a single external dependency (`gopkg.in/yaml.v3`, for the index),
so run `go mod tidy` once (needs network), then build:

```sh
go mod tidy
go build ./...      # builds the host (examples/ is a separate module)
sh demo.sh          # local-dir install lifecycle, end to end
```

`demo.sh` isolates state under `./dist` by pointing `HOME` at
`./dist/home` (dongle's data dir is always `~/.dongle`). Pass
`INDEX_ORG`/`INDEX_FEED` (and optionally `INDEX_PROJECT`/`INDEX_PACKAGE`) to
have it build with that index feed baked in and run `update`/`search`.

### Commands

`dongle search | install | upgrade | remove | update | support`, `dongle
<plugin> [args...]`, `dongle --version`, `--help`. The full reference
(flags, exit codes, output conventions, environment variables) is in
[docs/forge/reference/index.md](docs/forge/reference/index.md).

### Two ways to build the host

| command | binary |
|---|---|
| `go build ./cmd` | dev binary, `hostVersion` defaults to `"dev"`, no index feed identity baked in |
| `./scripts/build.sh <os> <arch>` | release binary `dist/dongle-<os>-<arch>` for that one platform |

Both produce the same **thin, host-only** binary: nothing — no plugins, no
plugin index — is embedded into it, and there is no build tag. The release
build differs only in the values it stamps in via `-ldflags` (see below).
Plugins and the index arrive at runtime, from the feed, starting with
[first run](#first-run-download-the-index-install-its-defaults).

`scripts/build.sh <os> <arch>` is a release maintainer's step,
run locally one platform at a time or by `azure-pipelines-release.yml`'s
job matrix (see "Pipelines" below) — **not** run in CI on every PR. It just
cross-compiles: it needs Go and nothing else — no `az`, no feed access.

### Build inputs are injected at build time (`configs/index.yaml`)

`hostVersion` and the index feed identity (org/project/feed/package name)
are not hardcoded in Go — they're plain vars in `cmd/root.go` (`protocol`,
the host↔plugin contract version, stays a `const`), stamped in at build
time via `-ldflags -X`:

- A plain `go build ./cmd` leaves them at their zero-value defaults:
  `hostVersion` is `"dev"` and no index org/feed is baked in, so first-run
  initialization is skipped. To use it, `dongle update` and the index-reading
  commands locally, stamp the feed in yourself, e.g. `go build -ldflags "-X
  main.indexOrg=<org> -X main.indexFeed=<feed>" ./cmd` (there are no runtime
  overrides).
- `scripts/build.sh` stamps all five, with the index feed
  identity read from `configs/index.yaml` via `tools/readconfig` — a small
  build-time-only Go helper (reuses `gopkg.in/yaml.v3`, not a `dongle`
  subcommand) whose `--index` flag prints `INDEX_ORG`/`INDEX_PROJECT`/
  `INDEX_FEED`/`INDEX_PACKAGE` as `eval`-able shell assignments — so the
  script itself hardcodes none of it. `hostVersion` still comes from outside
  the script — the pipeline's `DONGLE_VERSION` — and is **required** in CI
  (detected via `CI`/`TF_BUILD`/`GITHUB_ACTIONS`); locally it falls back to
  `git describe`, then `"dev"`.

```yaml
# configs/index.yaml
index:
  organization: "[ORG]" # TODO: set the real Azure DevOps organization
  project: "" # TODO: set for a project-scoped feed; leave empty for org-scoped
  feed: "[FEED]" # TODO: set the real shared feed name
  package: dongle-index
```

`configs/index.yaml` (see `configs/README.md`) is human-edited and consumed
only by `scripts/build.sh` — nothing in `configs/` is read at runtime.
It holds **no plugin list**: which plugins are installed by default is
declared by the index itself (its top-level `defaults.yaml`, see below).
Target platforms are not in it either — they live only in
`azure-pipelines-release.yml`'s job matrix (or whatever `<os> <arch>` pair
you pass the script by hand).

### Pipelines

- **`azure-pipelines-ci.yml`** — the fast soundness gate: `go build ./...`,
  `go vet ./...`, a `gofmt` check, and `go test ./...`. Runs on every PR and
  every push to `main`/a feature branch. No feed access, no
  cross-compilation.
- **`azure-pipelines-release.yml`** — manual-only (`trigger: none`, `pr:
  none`; queued by hand or the REST API). A `Guard` job fails fast unless
  run against a `release/X.Y.Z` branch, and derives `DONGLE_VERSION` from
  the branch name. A `BuildAndPublish` job then runs as a **matrix over all
  six target platforms**; each leg runs `./scripts/build.sh $(os) $(arch)`
  and publishes that one binary as its own Universal Package to the host
  feed (the only step that needs the Azure service connection).
  Feed name and service connection are parameterized at the top of the
  file with `TODO` placeholders — fill those in before running it.
- **`azure-pipelines-tooling.yml`** — publishes `tools/validate-manifest`
  (see below) to the feed. Same `Guard`-derives-`DONGLE_VERSION` shape as
  the release pipeline, but triggered off `tooling/X.Y.Z` branches and
  matrixed only over the platforms the tool itself needs to run on
  (`linux/amd64` for CI, `darwin/arm64` for local use) — not all six host
  targets.
- **`azure-pipelines-test-linux.yml`** — smoke-tests a published linux
  release by installing it the way the Forge getting-started guide says,
  then running it for the first time. Since first run downloads the index
  and default plugins, its service connection needs read access to those
  feeds too.

All actual build logic lives in `scripts/build.sh`; the pipelines only
orchestrate it, so a future GitHub Actions migration is a wrapper rewrite,
not a rewrite of the build itself.

### First run: download the index, install its defaults

The binary ships with nothing, so the first time it runs (before any
command, `--help` included) `internal/builtins.Initialize` sets it up from the
feed:

1. downloads the latest `dongle-index` package from the feed (the same
   `internal/index.Fetch` path `dongle update` uses) into a staging dir;
2. reads the index's top-level **`defaults.yaml`** — the single source of
   truth for which plugins are installed by default, maintained in the index
   repo (see its README) and shipped inside the index archive next to
   `VERSION` and `plugins/`:

   ```yaml
   defaults:
     - tacho
     - bell
   ```

3. installs each listed plugin through the normal install path (compat
   check, download from the feed, place under
   `plugins/<name>/<version>/<entrypoint>`, register in `state.json`);
4. swaps the downloaded index in as the cache and sets a
   `defaultsBootstrapped` flag in `state.json`, so this runs once.

From then on those plugins behave exactly like ones installed via `dongle
install`. Progress goes to stderr (a spinner on a terminal, plain lines
otherwise); later runs print nothing:

```
Downloading plugin index...
✓ Downloaded plugin index 2026.11.17.1
Installing tacho...
✓ Installed tacho 1.2.0
Installing bell...
✓ Installed bell 1.0.0
✓ Initialization complete.
```

**First run requires feed access** — there is no offline first run. If the
feed can't be reached (or a default plugin can't be downloaded), dongle
exits non-zero with:

```
error: couldn't reach the feed to initialize dongle — check your connection and that you're signed in (az login)
  cause: <the az error>
• Nothing was changed; run dongle again to retry.
```

and leaves no half-initialized state: default plugins already placed by
that run are removed again, the downloaded index is discarded, and the
`defaultsBootstrapped` flag stays unset, so the next run retries from
scratch. A default that can never be installed on this machine — not in the
index, incompatible with this host's version/protocol, or no build for this
os/arch — is skipped with a warning instead, since retrying wouldn't help.
Every invocation triggers it on first run, `dongle --help` and a bare
`dongle` included, so until it succeeds even help needs the feed.

### The index cache after first run

`internal/index.EnsureCache` makes sure *some* index is cached before a
command reads it (normally the one first run downloaded; with nothing
cached — a dev build that skipped first run, or a deleted cache — it
downloads the latest). `dongle update` always checks the feed's latest
index version (ignoring the 1h TTL below) and downloads it if newer,
replacing the cache on success; on failure the existing cache stays in use.

`search`, `install` and `upgrade` check the feed for a **newer index
version** than the cached one — but only once the cache is more than **1
hour** old (`internal/index.CheckTTL`), counted from the last time it was
downloaded from, or confirmed current against, the feed. Within that hour
the cache is used as-is: no feed call, no prompt. The check is a metadata
query only — the index archive is downloaded only after you agree:

- **Newer index, interactive** (stdin is a terminal): `install`/`upgrade`
  ask `A newer plugin index is available (<old> -> <new>). Update the index
  first? [y/N]`. Yes downloads the new index into the cache, prints its
  plugins (as `dongle update` does) and then runs against it; no uses the
  cached index and downloads nothing.
- **Newer index, `search`, or non-interactive** (piped/CI): never prompts
  or hangs — the cached index is used and a note says a newer one is
  available.
  `--sync` (check regardless of the TTL and update first) and `--no-sync`
  (use the cache, don't even check) make the choice up front.
- **No newer index**, or the feed can't be reached: proceeds on the cache.

Either way the command says which happened (updated / used cache / already
the latest).

## What works vs. what's stubbed

Real and testable now:

- Plugin **dispatch** — unknown command → resolve via `state.json` (entrypoint +
  requires recorded there at install time, no per-plugin manifest on disk) →
  compat gate → exec one-shot child, inheriting the terminal.
- **install / remove** from the index (installed plugins are listed by
  `dongle --version`); **search** from the
  index cache.
- **upgrade**: `dongle upgrade <name>` (or bare `dongle upgrade` for every
  installed plugin) compares an installed plugin's
  `ActiveVersion` against the version the index currently declares and, if
  the index is ahead, runs the same install path to fetch and place it —
  refusing to downgrade if the installed version is somehow newer than the
  index. The old version directory is left on disk (only the active pointer
  in `state.json` moves), so rollback stays possible.
- **Compatibility gates** (`requires.host` range + `requires.protocol` exact) at
  both install time and dispatch time, from the shared `internal/compat`.
- **Feed-archive index**: `dongle update`, `dongle --version` (shows the
  cached index version), version check before any download, offline-tolerant refresh, `dongle
  search`, and install-by-name resolution up to the download.

Stubbed (the seam is in place):

- `internal/builtins.downloadArtifact` calls the real `az artifacts universal
  download`; a REST-based implementation (no `az` dependency) is future work.

Not built yet (future work): authentication (a credential store, `login`, and
brokering credentials into plugins).

## Layout

```
azure-pipelines-ci.yml       PR/push soundness gate: build, vet, gofmt, test
azure-pipelines-release.yml  manual-only: Guard job (release/X.Y.Z + version)
                        then a BuildAndPublish job matrixed over the six
                        target platforms, publishing to the host feed
azure-pipelines-tooling.yml  publishes tools/validate-manifest to the feed;
                        same Guard shape as release, triggered off
                        tooling/X.Y.Z, matrixed only over the validator's
                        own runner platforms (not all six host targets)
configs/                build input consumed by scripts/build.sh
                        (index.yaml: index feed identity only — no plugin
                        list, no target platforms; defaults live in the
                        index's defaults.yaml, target platforms in the
                        release pipeline's matrix) — human-edited, not read
                        at runtime
scripts/build.sh        the release build script: build.sh <os> <arch>
                        cross-compiles one thin binary (no feed access)
cmd/                    host entry (cobra): main.go, root.go (root command +
                        plugin dispatch fall-through, plus the
                        hostVersion/indexOrg/indexProject/indexFeed/
                        indexPackage vars + protocol const, injected via
                        -ldflags — calls builtins.Initialize() for first
                        run), plugins.go (search/
                        install/remove/upgrade), update.go, support.go
internal/compat/       semver + host/protocol gate (single source of truth)
internal/state/        installed-plugin registry (entrypoint + requires) + on-disk paths
internal/dispatch/     resolve -> compat -> exec
internal/builtins/     the builtin commands, one file each: search.go
                        (search/support), install.go, remove.go,
                        upgrade.go, update.go, version.go (--version),
                        initialize.go (first run: download index, install
                        its defaults); built on
                        internal/index/state/ui
internal/ui/           TTY-aware output: aligned tables, color, status
                        lines, spinner, y/N prompt
internal/index/        feed-archive catalog: downloads + extracts the
                        versioned dongle-index package, Prepare/Update
                        (ensure cache, 1h TTL, version check, update
                        prompt, index summary), lookups, the index's
                        defaults.yaml list
tools/readconfig/       build-time-only helper: reads configs/index.yaml,
                        prints the index feed identity for scripts/build.sh
                        (not a dongle subcommand)
tools/validate-manifest/  standalone tool (not a dongle subcommand): validates
                        plugins/<name>.yaml manifests using the exact same
                        internal/index types/parsing dongle itself uses, plus
                        a check that all 6 mandatory os/arch pairs are declared and
                        a feed existence check per declared platform. Built
                        for the index repo's own PR pipeline to download from
                        the feed and run against changed/all manifests — see
                        azure-pipelines-tooling.yml (publishes it here) and
                        the generated azure-pipelines-validate.yml (consumes
                        it, in the separate index repo).
examples/dongle-deploy/  sample cobra plugin (its own module)
examples/index/          sample index-repo layout: a manifest (Azure feed
                        coordinates) + defaults.yaml
index-repo/              staged files for the separate plugin index repo
                        (see its own README.md): defaults.yaml (plugins
                        installed on first run), manifest validation +
                        publish-to-feed pipelines, contributor docs
```

## User and plugin-owner documentation

Docs are split by audience, with no content duplicated between the two:

- **dongle users** (and anything both audiences need):
  [`docs/forge/`](docs/forge/README.md), published to Forge (TODO: Forge
  base URL). This includes how dongle works (dispatch, the host↔plugin
  contract, compatibility and the three version axes, feeds, the index
  archive, first run): [explanation/design.md](docs/forge/explanation/design.md).
- **plugin owners**: the index repo's docs, staged in `index-repo/`, which
  link to Forge for shared content:
  - onboarding a plugin: [CONTRIBUTING.md](index-repo/CONTRIBUTING.md)
  - the manifest spec: the "Manifest reference" section of the index repo's
    [README.md](index-repo/README.md#manifest-reference)

`index-repo/` holds the files staged for the separate index repo (its
`README.md`, `CONTRIBUTING.md`, PR template, `CODEOWNERS` and pipelines);
copy them to that repo's root.

## Index access

How the index is released, cached and refreshed is described in
[docs/forge/explanation/design.md](docs/forge/explanation/design.md#the-index-a-released-archive-not-a-live-repo).

The index feed identity comes only from the values injected at build time
from `configs/index.yaml`'s `index:` section; nothing overrides it at runtime
(there are deliberately no environment variables for it). A plain `go build
./cmd` has no index org/feed baked in at all, so first-run initialization,
`dongle update` and the index-reading commands need at least
`-X main.indexOrg=...` and `-X main.indexFeed=...` in `-ldflags`.

## Before you publish this repo

- Replace `andreimladin` with your GitHub/module path everywhere:
  `grep -rl andreimladin . | xargs sed -i 's/andreimladin/<you>/g'`
- Set `organization`, `project`, and `feed` under `index:` in
  `configs/index.yaml`.
- Fill in the `LICENSE` year/name.

## License

MIT — see [LICENSE](LICENSE).
