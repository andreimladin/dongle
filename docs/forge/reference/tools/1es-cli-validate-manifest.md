# 1es-cli-validate-manifest

The manifest validator. The index repo's CI runs it on every manifest PR,
and plugin owners can run it locally. For a step-by-step walkthrough, see
[Validate your manifest](../../how-to/plug-in-your-cli/validate-your-manifest.md).
For the manifest fields themselves, see the
[Manifest reference](../manifest-reference.md).

<!-- TODO: naming — the tool's source is tools/validate-manifest in the dongle
     repo and it's currently published as validate-manifest-<os>-<arch>. If
     the published packages are renamed to 1es-cli-validate-manifest-<os>-<arch>,
     update the names on this page. -->

## Purpose

Validates `plugins/<name>.yaml` manifests with **the same parsing code
`dongle install` uses**, so a manifest that passes is one dongle can read.
It's a build/CI tool, not a dongle command.

## Availability

| source | platforms |
|---|---|
| Universal Package `validate-manifest-linux-amd64` | Linux x64 (used by CI) |
| Universal Package `validate-manifest-darwin-arm64` | macOS on Apple silicon |
| `go run ./tools/validate-manifest` from the dongle repo | any OS with Go (including Windows) |

The packages are published to the tooling feed <!-- TODO: tooling feed name -->
`<TODO-tooling-feed>`, at versions matching the dongle repo's
`tooling/X.Y.Z` branches. The index repo's CI pins one version
(`VALIDATE_MANIFEST_VERSION` in its `azure-pipelines-validate.yml`).

Download:

```sh
az artifacts universal download \
  --organization https://dev.azure.com/<TODO-org> \
  --feed <TODO-tooling-feed> \
  --name validate-manifest-darwin-arm64 \
  --version <version> \
  --path ./bin
chmod +x ./bin/validate-manifest-darwin-arm64
```

## Usage

```
validate-manifest <file>...
validate-manifest --dir <plugins-dir>
validate-manifest -h | --help
```

| argument / flag | meaning |
|---|---|
| `<file>...` | one or more manifest files to validate |
| `--dir <plugins-dir>` | validate every `*.yaml` directly inside the directory (not recursive), in sorted order. Takes exactly one directory, and must be the first argument |
| `-h`, `--help` | print usage and exit `0` |

With no arguments, it prints usage and exits `2`.

## Requirements

The package-existence check calls `az artifacts universal download`, so you
need:

- the **Azure CLI** on `PATH`, with the `azure-devops` extension;
- an authenticated session: `az login`, or `AZURE_DEVOPS_EXT_PAT` set in
  the environment (CI uses this). The tool never runs `az login` itself;
- **read access to every feed** the manifests name.

Without `az`, the tool prints `warning: az CLI not found on PATH — package-existence
checks will fail closed for every manifest`, and every manifest fails.

## What it checks

Each manifest goes through two layers. If layer 1 fails, layer 2 is skipped
for that manifest.

### Layer 1: correctness

All errors are reported together:

| check | error message |
|---|---|
| valid YAML, **no unknown fields** at any level | `invalid manifest at <path>: yaml: unmarshal errors: ... field <x> not found in type ...` |
| `name` present | `name is required` |
| `name` equals the filename stem | `name "<name>" does not match filename "<file>" (expected name: <stem>)` |
| `version` present | `version is required` |
| `version` is valid semver | `version "<v>" is not a valid semver` |
| `requires.host`, if set, is `>=x.y.z` or `x.y.z` | `requires.host "<c>": invalid semver "<...>"` |
| `feed.organization` present | `feed.organization is required` |
| `feed.feed` present | `feed.feed is required` |
| `feed.packageType` present | `feed.packageType is required` |
| `feed.packageName` present | `feed.packageName is required` |
| `support.documentation` present | `support.documentation is required` |
| `support.channel` present | `support.channel is required` |
| at least one `platforms` entry | `platforms: at least one platform entry is required` |
| each entry has `selector.os` | `platforms[<i>].selector.os is required` |
| each entry has `selector.arch` | `platforms[<i>].selector.arch is required` |
| each entry has `package` | `platforms[<i>].package is required` |

`<i>` counts from 0.

### Layer 2: package existence

For **each** `platforms` entry, the tool downloads `package` at `version`
(without the leading `v`) from the feed the manifest names
(`feed.organization`, `feed.feed`, plus `feed.project` with project scope if
set), into a temporary folder that's deleted right away. A successful
download proves the package exists and can be downloaded. Each platform is
reported separately.

### Not checked

- the values of `requires.protocol` (must be `"v1"`), `feed.packageType`
  (must be `upack`), and `selector.os`/`selector.arch` (must be real Go
  platform names);
- that a package contains exactly one file, or that the executable runs
  or is self-contained;
- the platform policy (macOS and/or Windows required), which is checked in
  review;
- `support` URLs;
- duplicate platform entries.

## Output

For each file:

```
== <path> ==
  correctness: OK | FAIL
    error: <message>                       # one per problem, when FAIL
  <os>/<arch> <package>@<version>: OK      # one per platform, when correctness is OK
  <os>/<arch> <package>@<version>: FAIL — az artifacts universal download: <az error>
```

followed by a final line `<passed>/<total> manifests passed`.

## Exit codes

| code | meaning |
|---|---|
| `0` | every manifest passed both layers |
| `1` | at least one manifest failed, or package existence couldn't be confirmed (including no `az`), or `--dir` couldn't be read |
| `2` | usage error: no arguments, or `--dir` without exactly one directory |

## In the index repo's CI

`azure-pipelines-validate.yml` in the index repo downloads the pinned
`validate-manifest-linux-amd64` and runs it:

- **On PRs to `develop` (and `main`) that change `plugins/*.yaml`**: it runs
  `validate-manifest <changed files>` on the manifests the PR added,
  modified, copied or renamed (deleted files are skipped). It also logs a
  `NEW PLUGIN MANIFEST: plugins/<name>.yaml …` warning for brand-new files
  (and posts it as a PR comment where configured). The warning doesn't fail
  the build.
- **Nightly at 06:00 UTC on `develop`, and on demand**: it runs
  `validate-manifest --dir plugins/` over every manifest, to catch drift
  such as a package deleted from a feed after its manifest was merged.

CI authenticates with the pipeline's own token (`AZURE_DEVOPS_EXT_PAT`), so
the index repo's build identity needs **Reader** access on every feed the
manifests name.
