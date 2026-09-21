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

### Two ways to build the host

| command | binary | plugins |
|---|---|---|
| `go build ./cmd` | dev binary, `hostVersion` defaults to `"dev"` | none — install them the normal way |
| `./scripts/build.sh build_target <os> <arch>` | `dist/dongle-<os>-<arch>` for that one platform | embedded defaults from `configs/build.yaml` |

`go build ./cmd` is always available and requires nothing beyond the Go
toolchain — embedding is entirely opt-in and behind a build tag, so a plain
build has no new behavior. **Release always embeds — there is no no-embed
release path and no `--embed` flag; the only no-embed build is the plain
`go build ./cmd` above (CI/dev only, never published).**

`scripts/build.sh` (see "Embedded default plugins" below) is a release
maintainer's manual step, dispatched by its first argument to one of three
functions, run locally one platform at a time or by
`azure-pipelines-release.yml`'s job matrix (see "Pipelines" below) — **not**
run in CI on every PR:

| function | needs | does |
|---|---|---|
| `fetch_embedded <os> <arch>` | `az` (logged in to the plugin feed), `git` | stages `internal/bootstrap/embedded/` for that platform — no compiling |
| `build_binary <os> <arch>` | just Go | compiles `dist/dongle-<os>-<arch>` from whatever is already staged — no feed access |
| `build_target <os> <arch>` | both of the above | `fetch_embedded` then `build_binary`, for a local one-shot build |

Feed-coordinate resolution goes through `tools/resolve-plugin` (see below)
rather than a separate YAML tool.

### Build inputs are injected at build time (`configs/build.yaml`)

`hostVersion`, the index URL, and the index branch are not hardcoded in Go —
they're plain vars in `cmd/root.go` (`protocol`, the host↔plugin contract
version, stays a `const`), stamped in at build time via `-ldflags -X`:

- A plain `go build ./cmd` leaves them at their zero-value defaults:
  `hostVersion` is `"dev"` and no index URL is baked in — set
  `DONGLE_INDEX_URL` (and optionally `DONGLE_INDEX_BRANCH`) at runtime to use
  `dongle index`/`dongle plugin` commands locally.
- `scripts/build.sh`'s `build_binary` stamps all three, with the index
  url/branch read from `configs/build.yaml` (via `tools/readconfig` — see
  "Embedded default plugins" below) — the script itself hardcodes none of it.
  `hostVersion` still comes from outside the script — the pipeline's
  `DONGLE_VERSION` — and is **required** in CI (detected via
  `CI`/`TF_BUILD`/`GITHUB_ACTIONS`); locally it falls back to `git describe`,
  then `"dev"`.

`configs/build.yaml` (see `configs/README.md`) is human-edited and consumed
only by `scripts/build.sh` — nothing in `configs/` is read at runtime or
embedded into the binary. Target platforms are **not** in
`configs/build.yaml` — they live only in `azure-pipelines-release.yml`'s
job matrix (or whatever `<os> <arch>` pair you pass the script by hand).

### Pipelines

- **`azure-pipelines-ci.yml`** — the fast soundness gate: `go build ./...`,
  `go vet ./...`, a `gofmt` check, and `go test ./...`. Runs on every PR and
  every push to `main`/a feature branch. No feed access, no
  cross-compilation.
- **`azure-pipelines-release.yml`** — manual-only (`trigger: none`, `pr:
  none`; queued by hand or the REST API). A `Guard` job fails fast unless
  run against a `release/X.Y.Z` branch, and derives `DONGLE_VERSION` from
  the branch name. A `BuildAndPublish` job then runs as a **matrix over all
  six target platforms**; each leg logs in via an Azure service connection
  with access to both the plugin feed (to download `configs/build.yaml`'s
  embedded defaults) and a separate host feed, runs `./scripts/build.sh
  fetch_embedded $(os) $(arch)` then `./scripts/build.sh build_binary $(os)
  $(arch)`, and publishes that one binary as its own Universal Package to
  the host feed. Feed names and the service connection are parameterized at
  the top of the file with `TODO` placeholders — fill those in before
  running it.
- **`azure-pipelines-tooling.yml`** — publishes `tools/validate-manifest`
  (see below) to the feed. Same `Guard`-derives-`DONGLE_VERSION` shape as
  the release pipeline, but triggered off `tooling/X.Y.Z` branches and
  matrixed only over the platforms the tool itself needs to run on
  (`linux/amd64` for CI, `darwin/arm64` for local use) — not all six host
  targets. Feed name and service connection are separate `TODO`
  placeholders from the release pipeline's, since this only needs feed
  WRITE, not plugin-feed READ.

All actual build logic lives in `scripts/build.sh`; both pipelines only
orchestrate it, so a future GitHub Actions migration is a wrapper rewrite,
not a rewrite of the build itself.

### Embedded default plugins (`embed` build tag)

`configs/build.yaml` is the single, reviewable, diffable source of truth for
which plugins (at which exact versions) ship baked into a release build,
and which index they're resolved against:

```yaml
index:
  url: https://dev.azure.com/[ORG]/[PROJECT]/_git/[REPO] # TODO: set the real index repo URL
  branch: main

embedded:
  - { name: tacho, version: 2.4.0 }
  - { name: bell, version: 1.1.0 }
```

`scripts/build.sh` reads it through `tools/readconfig` — a small
build-time-only Go helper in this repo (reuses `gopkg.in/yaml.v3`, no `yq`
dependency), not a `dongle` subcommand — instead of hardcoding any of it:
`readconfig --index` prints `INDEX_URL`/`INDEX_BRANCH` as `eval`-able shell
assignments, `readconfig --embedded` prints one `name:version` line per
plugin. `scripts/build.sh` builds `readconfig` and `tools/resolve-plugin`
once, up front, to temp binaries rather than `go run` on every call.

`fetch_embedded <os> <arch>` clones the plugin index fresh into a temp dir
on every run, using the same `INDEX_URL`/`INDEX_BRANCH` that `build_binary`
also bakes into the binary via `-ldflags`. For each `embedded` entry it
shells out to `tools/resolve-plugin` — a small build-time-only Go helper,
not a `dongle` subcommand: `resolve-plugin <name> <version> <os> <arch>
--index <indexdir>` reads `plugins/<name>.yaml` directly from the freshly
cloned checkout (no cache, no network) and prints the plugin's Azure
Artifacts feed coordinates and per-platform package name as `eval`-able
`KEY="value"` lines — or fails, naming the plugin and platform, if that
plugin has no published build for `<os>/<arch>`. Nothing about the feed —
organization, feed name, project, or package naming — is hardcoded in the
script itself, and the manifest is parsed by the exact same `internal/index`
code (plus one purely-additive `LoadFile` helper for reading from an
arbitrary path) that the CLI uses for `dongle plugin install` — not
reimplemented. It then downloads each plugin's binary into
`internal/bootstrap/embedded/` (git-ignored except for the tracked
`internal/bootstrap/embedded/.gitkeep` placeholder) and writes
`internal/bootstrap/embedded/manifest.json`.

`build_binary <os> <arch>` then builds with `-tags embed` so
`internal/bootstrap/bootstrap.go`'s `//go:embed all:embedded` picks up
whatever `fetch_embedded` staged — no feed/`az` access in this function at
all, it only compiles. On first run, `bootstrap.InstallDefaults()` unpacks
the embedded plugins into the normal plugin store
(`plugins/<name>/<version>/<entrypoint>`) and sets a `defaultsBootstrapped`
flag in `state.json` so it never runs again — from then on those plugins
behave exactly like ones installed via `dongle plugin install`.

The embedding mechanism itself — the `//go:embed` directive, the staged
`embedded/` payload, and both the real and no-op `InstallDefaults`
implementations — lives entirely in `internal/bootstrap`, since `//go:embed`
paths are relative to the source file and can't reach outside a package
with `../`. `cmd/` only calls `bootstrap.InstallDefaults()`; it holds no
embedding logic of its own.

A binary built without `-tags embed` (i.e. anything but `build_binary`'s
output) links `internal/bootstrap/noop.go` instead, whose
`InstallDefaults()` is a no-op — no embed dependency, no behavior change,
nothing staged.

Local usage for one platform:

```sh
./scripts/build.sh build_target darwin arm64
```

or the two steps separately:

```sh
./scripts/build.sh fetch_embedded darwin arm64
./scripts/build.sh build_binary darwin arm64
```

## What works vs. what's stubbed

Real and testable now:

- Plugin **dispatch** — unknown command → resolve via `state.json` (entrypoint +
  requires recorded there at install time, no per-plugin manifest on disk) →
  compat gate → exec one-shot child, inheriting the terminal.
- **install / uninstall / list** from a local build dir; **search** from the
  index cache.
- **update**: `dongle plugin update <name>` compares an installed plugin's
  `ActiveVersion` against the version the index currently declares and, if
  the index is ahead, runs the same install path to fetch and place it —
  refusing to downgrade if the installed version is somehow newer than the
  index. The old version directory is left on disk (only the active pointer
  in `state.json` moves), so rollback stays possible.
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
azure-pipelines-ci.yml       PR/push soundness gate: build, vet, gofmt, test
azure-pipelines-release.yml  manual-only: Guard job (release/X.Y.Z + version)
                        then a BuildAndPublish job matrixed over the six
                        target platforms, publishing to the host feed
azure-pipelines-tooling.yml  publishes tools/validate-manifest to the feed;
                        same Guard shape as release, triggered off
                        tooling/X.Y.Z, matrixed only over the validator's
                        own runner platforms (not all six host targets)
configs/                build input consumed by scripts/build.sh
                        (build.yaml: index url/branch, embedded plugins —
                        no target platforms, those live in the release
                        pipeline's matrix) — human-edited, not read at
                        runtime, not embedded
scripts/build.sh        the one release build script: fetch_embedded,
                        build_binary, build_target, dispatched by first arg
                        — see "Embedded default plugins" below
cmd/                    host entry (cobra): main.go, root.go (root command +
                        plugin dispatch fall-through, plus the
                        hostVersion/indexURL/indexBranch vars + protocol
                        const, injected via -ldflags — calls
                        bootstrap.InstallDefaults(), holds no embedding
                        logic), plugin.go, index.go
internal/bootstrap/    embedded default plugins (see below): bootstrap.go /
                        noop.go, plus the staged embedded/ payload
internal/compat/       semver + host/protocol gate (single source of truth)
internal/state/        installed-plugin registry (entrypoint + requires) + on-disk paths
internal/dispatch/     resolve -> compat -> exec
internal/plugincmd/    plugin list/search/install/uninstall (+ index resolver)
internal/index/        embedded git catalog: clone/TTL-pull cache, lookups
tools/resolve-plugin/   build-time-only helper: manifest -> feed coordinates
                        for one plugin/platform (not a dongle subcommand) —
                        see "Embedded default plugins" above
tools/readconfig/       build-time-only helper: reads configs/build.yaml,
                        prints the index coords or embedded-plugin list for
                        scripts/build.sh (not a dongle subcommand)
tools/validate-manifest/  standalone tool (not a dongle subcommand): validates
                        plugins/<name>.yaml manifests using the exact same
                        internal/index types/parsing dongle itself uses, plus
                        a feed existence check per declared platform. Built
                        for the index repo's own PR pipeline to download from
                        the feed and run against changed/all manifests — see
                        azure-pipelines-tooling.yml (publishes it here) and
                        the generated azure-pipelines-validate.yml (consumes
                        it, in the separate index repo).
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

Set the index URL/branch in `configs/build.yaml`'s `index:` section — it's
injected into the binary at build time (see "Build inputs are injected at
build time" above). `DONGLE_INDEX_URL` overrides it for dev.

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
`DONGLE_INDEX_BRANCH` pins a different branch (both default to the values
injected at build time from `configs/build.yaml`'s `index:` section; a
plain `go build ./cmd` has no index URL baked in at all, so one of these
overrides is required to use `dongle index`/`dongle plugin` commands).

## Before you publish this repo

- Replace `andreimladin` with your GitHub/module path everywhere:
  `grep -rl andreimladin . | xargs sed -i 's/andreimladin/<you>/g'`
- Set `index.url` and `index.branch` in `configs/build.yaml`.
- Fill in the `LICENSE` year/name.

## License

MIT — see [LICENSE](LICENSE).
