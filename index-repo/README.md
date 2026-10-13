# dongle plugin index

This repo is the **dongle plugin index**: the catalog of plugins users can
install with `dongle install <name>`. It holds one manifest per plugin,
`plugins/<name>.yaml`, which says where the plugin's executables are
published and which build to use on each platform. The dongle team
releases the index as a versioned package that dongle downloads.

The docs in this repo are for **plugin owners** who want to make their CLI
available through dongle. Docs for people who *use* dongle are on Forge:
<!-- TODO: Forge base URL — replace <TODO-FORGE-BASE-URL> in the links on
     this page, in CONTRIBUTING.md and in docs/. -->
[dongle documentation](<TODO-FORGE-BASE-URL>/welcome).

## Start here

- **[Concepts](<TODO-FORGE-BASE-URL>/explanation/concepts)** (Forge):
  what a plugin, a manifest and the index are. Read this first.
- **[How dongle works](<TODO-FORGE-BASE-URL>/explanation/design)** (Forge):
  what happens when a user runs your plugin. In particular:
  - [Running a plugin](<TODO-FORGE-BASE-URL>/explanation/design#running-a-plugin-one-process-one-shot):
    the host↔plugin contract your executable must honor (arguments,
    inherited terminal, environment variables, exit code);
  - [Compatibility checks](<TODO-FORGE-BASE-URL>/explanation/design#compatibility-checks):
    how `requires.host` and `requires.protocol` are enforced;
  - [Where plugins come from](<TODO-FORGE-BASE-URL>/explanation/design#where-plugins-come-from-feeds):
    how dongle downloads and installs your packages.

## Adding or updating a plugin

Follow [CONTRIBUTING.md](CONTRIBUTING.md): the requirements your plugin
must meet, and every step from publishing your executables to getting your
plugin released.

## Reference

| page | contents |
|---|---|
| [Manifest reference](docs/manifest-reference.md) | every field of `plugins/<name>.yaml`, with a full annotated example. **The single source of truth for manifest fields** |
| [1es-cli-validate-manifest](docs/1es-cli-validate-manifest.md) | the manifest validator CI runs on your PR: usage, every check and error message, exit codes |
| [Command reference](<TODO-FORGE-BASE-URL>/reference) (Forge) | every dongle command, useful when testing your plugin through dongle |

## How the index is released

dongle never reads this repo directly. Manifests are merged into
`develop`; the dongle team then releases the index:

- a release is cut from a `release/X.Y.Z` branch, and `X.Y.Z` becomes the
  index version;
- `azure-pipelines-publish-index.yml` packs the repo's `plugins/` folder,
  plus a `VERSION` file, into `index.tar.gz`;
- that archive is published to the feed as the **`dongle-index`** Universal
  Package, at version `X.Y.Z`.

A merged manifest therefore reaches users only after the next index
release. How users' machines pick up a new index is described in
[How dongle works](<TODO-FORGE-BASE-URL>/explanation/design#the-index-a-released-archive-not-a-live-repo).

## What's in this repo

| path | purpose |
|---|---|
| `plugins/<name>.yaml` | one manifest per plugin |
| `CONTRIBUTING.md` | how to add or update a plugin |
| `PULL_REQUEST_TEMPLATE.md` | checklist for manifest PRs |
| `CODEOWNERS` | who owns each plugin's manifest |
| `docs/` | contributor reference (manifest spec, validator) |
| `azure-pipelines-validate.yml` | validates changed manifests on PRs, and all manifests nightly |
| `azure-pipelines-publish-index.yml` | publishes the index as the `dongle-index` package |
