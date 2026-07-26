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

- Plugin **dispatch** — unknown command → resolve via `state.json` → load
  `plugin.json` → compat gate → exec one-shot child, inheriting the terminal.
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
internal/manifest/     plugin.json (runtime manifest) loader
internal/state/        installed-plugin registry + on-disk paths
internal/dispatch/     resolve -> compat -> exec
internal/plugincmd/    plugin list/search/install/uninstall (+ index resolver)
internal/index/        embedded git catalog: clone/TTL-pull cache, lookups
pkg/pluginsdk/         helpers for Go plugin authors
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
the plugin name and shipping a `plugin.json`; its whole subcommand tree keeps
working because dispatch hands args straight to cobra. See
`examples/dongle-deploy`.

## Three version axes

Bound by each manifest's `requires`: the host's semver (`hostVersion` in
`cmd/dongle/main.go`), each plugin's semver, and a slow-moving protocol version.
Host and protocol are separate constants so they release independently.

## Publishing a plugin (git index + shared Azure feed)

1. Build per-`os/arch` tarballs (binary + `plugin.json`); record each `sha256`.
2. Publish to the shared Azure Artifacts feed as a Universal Package (one package
   per plugin): `az artifacts universal publish --feed dongle-plugins --name
   dongle-deploy --version 2.3.1 --path ./dist`.
3. PR `plugins/<name>.yaml` to the index repo (version + feed coords +
   checksums). See `examples/index/plugins/deploy.yaml`.

Set the embedded index URL in `internal/index/index.go` (`IndexURL`).
`DONGLE_INDEX_URL` overrides it for dev.

## Before you publish this repo

- Replace `andreimladin` with your GitHub/module path everywhere:
  `grep -rl andreimladin . | xargs sed -i 's/andreimladin/<you>/g'`
- Set `IndexURL` in `internal/index/index.go`.
- Fill in the `LICENSE` year/name.

## License

MIT — see [LICENSE](LICENSE).
