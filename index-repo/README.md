# dongle plugin index

This repo is the **dongle plugin index**: the catalog of every plugin that
users can install through `dongle`. It explains what the index is, how
`dongle` uses it, how index versions are released, and the full manifest
format.

To add or update a plugin, see **[CONTRIBUTING.md](CONTRIBUTING.md)**.

**Contents**

- [What this repo is](#what-this-repo-is)
- [Repository layout](#repository-layout)
- [How dongle uses the index](#how-dongle-uses-the-index)
- [How an index version is released](#how-an-index-version-is-released)
- [Plugin and package requirements](#plugin-and-package-requirements)
- [Manifest file reference](#manifest-file-reference)
- [CI validation](#ci-validation)
- [Governance](#governance)
- [Common mistakes](#common-mistakes)

---

## What this repo is

`dongle` is a host CLI that other CLIs plug into. A user runs
`dongle install <name>`, and from then on `dongle <name> [args...]` runs that
plugin. The plugin runs as a short-lived child process that inherits the
user's terminal, with credentials and context passed in by `dongle`.

This repo tells `dongle` **which plugins exist and where to download each
one**. It holds one YAML manifest per plugin:

```
plugins/
  deploy.yaml      # registers `dongle deploy`
  <name>.yaml      # registers `dongle <name>`
```

The repo contains **only manifests**: no plugin source code and no binaries.
Each plugin's executables are stored as Universal Packages in an Azure
Artifacts feed, and the manifest says which feed that is and which package
to download for each operating system and CPU architecture.

## Repository layout

| path | purpose |
|---|---|
| `plugins/<name>.yaml` | one manifest per plugin (see [Manifest file reference](#manifest-file-reference)) |
| `CONTRIBUTING.md` | how to onboard or update a plugin |
| `PULL_REQUEST_TEMPLATE.md` | checklist for manifest PRs |
| `CODEOWNERS` | review ownership |
| `azure-pipelines-validate.yml` | CI: validates manifests on every PR, plus a nightly full scan (see [CI validation](#ci-validation)) |
| `azure-pipelines-publish-index.yml` | release: publishes a new index version (see [How an index version is released](#how-an-index-version-is-released)) |

## How dongle uses the index

`dongle` never clones or reads this git repo. It works from a released
**index package** instead:

1. The index is published to the shared Azure Artifacts feed as a Universal
   Package named **`dongle-index`**. Each version is an `index.tar.gz`
   archive holding the `plugins/` directory and a `VERSION` file.
2. `dongle` downloads the latest `dongle-index`, extracts it into its local
   data directory, and caches it for a fixed time. When the cache expires,
   `dongle` checks for a newer index version again.
3. Users can refresh the cache immediately with **`dongle update`**.
4. Commands read manifests from the cached copy:

   | command | what it uses from the manifest |
   |---|---|
   | `dongle search` | every plugin's `name`, `version`, `shortDescription` |
   | `dongle install <name>` | `requires` (compatibility check), `feed` + the `platforms` entry for the user's OS/arch (download), `version` |
   | `dongle upgrade [name]` | compares the installed version with the index's `version`, then installs it if the index is newer (never downgrades) |
   | `dongle support <name>` | the `support` block |
   | `dongle <name> ...` | re-checks `requires` before every run |
   | `dongle --version` | shows the host, index, and installed plugin versions |

At install time, `dongle` downloads the package for the user's platform,
takes the single file inside it, and installs it as `dongle-<name>`. The
file's own name inside the package doesn't matter.

**What this means for contributors:** a merged manifest is invisible to
users until the next index version is released and their cache refreshes.

## How an index version is released

Releases are done by the **dongle team**, by hand. Contributors don't
release; they ask the dongle team to (see [CONTRIBUTING.md](CONTRIBUTING.md)).

1. The dongle team creates a branch named **`release/X.Y.Z`** that contains
   the merged manifests to release.
2. They manually queue **`azure-pipelines-publish-index.yml`** against that
   branch. The pipeline never runs automatically: it has no push or PR
   triggers.
3. The pipeline:
   - checks that the branch name is exactly `release/X.Y.Z` and takes
     **`X.Y.Z` as the index version** (no git tags, no auto-increment);
   - writes `X.Y.Z` into a `VERSION` file and archives `VERSION` plus
     `plugins/` into `index.tar.gz`;
   - publishes `index.tar.gz` to the shared feed as `dongle-index` at version
     `X.Y.Z`.
4. The pipeline never writes to git. It only reads the branch it runs on.

Once published, users get the new index the next time their cache expires,
or immediately with `dongle update`.

---

## Plugin and package requirements

### The plugin

- **It must be a single, standalone executable.** `dongle` runs exactly one
  file per plugin.
- **It must not depend on other tools or runtimes being installed** on the
  user's machine (interpreters, frameworks, other CLIs, shared libraries
  that aren't part of the OS). If your plugin needs something like that,
  contact the dongle team before onboarding.
- **It must run on macOS, Windows, or both.** Ship a separate executable
  for each OS/architecture you support:

  | platform | `selector.os` | `selector.arch` |
  |---|---|---|
  | macOS, Apple silicon | `darwin` | `arm64` |
  | macOS, Intel | `darwin` | `amd64` |
  | Windows, x64 | `windows` | `amd64` |
  | Windows, ARM | `windows` | `arm64` |

  You don't need to cover every platform. A user on a platform you don't
  list gets a clear error: `<name> <version> has no build for <os>/<arch>`.
- **It must follow the dongle host↔plugin contract, protocol `v1`:**
  arguments are everything after the plugin name, stdin/stdout/stderr and
  the terminal are inherited, the exit code is passed back to the user
  unchanged, and context and credentials arrive as `DONGLE_*` environment
  variables.

### The packages

Each platform's executable is published as its **own Universal Package** in
an Azure Artifacts feed, **before** the manifest PR is opened. CI downloads
every package a manifest references and fails if any is missing.

- **Exactly one file per package**: the executable. If a downloaded package
  has zero files or more than one, `dongle install` fails with
  `package <name>@<version> contains N files, expected exactly 1`.
- **Package version = manifest `version` without a leading `v`.** For example,
  `version: v2.3.1` means every package must exist at `2.3.1`.
- **All of a plugin's platform packages share one version**, because they
  are all fetched at the manifest's single `version`.
- **Package names are up to you.** The recommended convention is
  `<packageName>_<version>_<os>_<arch>`, e.g.
  `dongle-deploy_2.3.1_windows_amd64`. dongle doesn't parse the name.
- **The index repo's CI build identity needs Reader access on the feed**, or
  the package-existence check fails with 401/403. A new feed needs this set
  up once by the dongle team.

---

## Manifest file reference

A manifest is a YAML file at `plugins/<name>.yaml`. It has exactly seven
top-level keys: `name`, `version`, `shortDescription`, `requires`, `feed`,
`platforms`, `support`. This section and the `validate-manifest` checks
described in [CI validation](#ci-validation) are the whole definition of the
format.

- **Parsing is strict.** Any key not listed here, at any nesting level,
  fails validation (`field <x> not found in type ...`). This is almost always
  a typo, so fix the key name.
- **All values are strings**, except `platforms`, which is a list. Quote
  values YAML might misread, especially constraints starting with `>`, e.g.
  `host: ">=0.1.0"`.

### Field summary

| field | required | type | example |
|---|---|---|---|
| `name` | **required** | string | `deploy` |
| `version` | **required** | string (semver) | `v2.3.1` |
| `shortDescription` | optional | string | `Deploy services to the platform` |
| `requires` | optional | mapping | |
| `requires.host` | optional | string (constraint) | `">=0.1.0"` |
| `requires.protocol` | optional (strongly recommended) | string | `"v1"` |
| `feed` | **required** | mapping | |
| `feed.organization` | **required** | string | `acme` |
| `feed.project` | optional (only for project-scoped feeds) | string | `acme-platform` |
| `feed.feed` | **required** | string | `dongle-plugins` |
| `feed.packageType` | **required** | string | `upack` |
| `feed.packageName` | **required** | string | `dongle-deploy` |
| `platforms` | **required** (at least 1 entry) | list of mappings | |
| `platforms[].selector` | **required** per entry | mapping | `{ os: darwin, arch: arm64 }` |
| `platforms[].selector.os` | **required** per entry | string | `darwin` |
| `platforms[].selector.arch` | **required** per entry | string | `arm64` |
| `platforms[].package` | **required** per entry | string | `dongle-deploy_2.3.1_darwin_arm64` |
| `support` | **required** | mapping | |
| `support.documentation` | **required** | string (URL) | `https://docs.acme.internal/dongle-deploy` |
| `support.channel` | **required** | string (URL) | `https://acme.slack.com/archives/C0DEPLOY` |
| `support.contact` | optional | string | `deploy-team@acme.example.com` |

### `name` (required, string)

The plugin's name and its command: `name: deploy` registers `dongle deploy`.
It **must equal the filename without `.yaml`**, or validation fails with
`name "X" does not match filename "Y.yaml" (expected name: Y)`. Use
lowercase letters, digits, and hyphens. The name must not collide with an
existing plugin or a dongle builtin command (`search`, `install`, `remove`,
`upgrade`, `update`, `support`). Also avoid `help` and anything starting
with `-`.

### `version` (required, string)

The plugin's own version. It must be valid semver, `MAJOR.MINOR.PATCH`, with
optional `-prerelease` and `+build` suffixes and an optional leading `v`.
The validator's exact rule:

```
^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-.]+)?(\+[0-9A-Za-z-.]+)?$
```

- Valid: `2.3.1`, `v2.3.1`, `1.0.0-rc.1`, `1.0.0+build.5`
- Invalid: `2.3`, `v2`, `2.3.1.4`, `02.3.1`, `version-2.3.1`

Packages are downloaded at this version with the `v` removed.
`dongle upgrade` compares it with the installed version. Bump it every time
you publish new executables.

### `shortDescription` (optional, string)

A one-line summary shown by `dongle search`. If you leave it out, that
column is blank.

### `requires` (optional, mapping)

The compatibility gate. dongle checks it at install time **and** before
every run. If the check fails, the plugin is not installed or run, and the
user sees the reason.

- **`requires.host`**: the dongle host versions the plugin works with.
  Only three forms are supported:

  | form | meaning |
  |---|---|
  | `""` or omitted | any host version |
  | `">=x.y.z"` | host version x.y.z or newer |
  | `"x.y.z"` | exactly host version x.y.z |

  `^`, `~`, `<`, and `||` ranges are **not supported**. If set, the validator
  checks that the value parses.
- **`requires.protocol`**: the host↔plugin contract version the plugin
  speaks. The current host speaks **`v1`**, and the value must match exactly.
  An empty value means "any protocol", which is rarely what you want. **The
  validator does not check this value**, so a typo like `"1"` or `"V1"`
  passes CI but makes the plugin impossible to install.

### `feed` (required, mapping)

The Azure Artifacts feed holding the plugin's packages. Both `dongle install`
and CI's existence check use **the feed this manifest names**.

- **`feed.organization`** (required): the Azure DevOps organization name
  only, not a URL. dongle builds `https://dev.azure.com/<organization>` from
  it.
- **`feed.project`** (optional): set this **only for a project-scoped feed**,
  and **leave the line out entirely** for an org-scoped feed. Getting this
  wrong either way makes downloads fail.
- **`feed.feed`** (required): the feed name.
- **`feed.packageType`** (required): always **`upack`** (Universal Package),
  the only type dongle downloads. The validator only checks that it's
  non-empty.
- **`feed.packageName`** (required): the plugin's base package name, a label
  for all its per-platform packages, conventionally `dongle-<name>`. It must
  be non-empty, but nothing is downloaded by this name. The
  `platforms[].package` values are what get fetched.

### `platforms` (required, list, at least one entry)

One entry per OS/architecture you published a package for. Each entry has:

- **`selector.os`** and **`selector.arch`** (required): Go `GOOS`/`GOARCH`
  spelling, see the platform table in [The plugin](#the-plugin). dongle
  picks the entry whose values **exactly** equal the user's platform
  (case-sensitive). The validator only checks that they're non-empty, so
  `macos`, `win`, `x86_64`, or `Darwin` pass CI but never match anyone.
- **`package`** (required): the exact Universal Package name published for
  this platform. It is downloaded at the manifest's `version`.

List each OS/arch pair once. If a pair appears twice, only the first entry
is used. An empty list fails with
`platforms: at least one platform entry is required`. The inline form
`selector: { os: darwin, arch: arm64 }` and the block form (with `os:` and
`arch:` on their own lines) are equivalent.

### `support` (required, mapping)

Shown by `dongle support <name>`.

- **`support.documentation`** (required): URL of the plugin's documentation.
- **`support.channel`** (required): URL of the plugin's support channel (a
  chat channel, issue tracker, etc.).
- **`support.contact`** (optional): an email address or handle.

### Full annotated example

A complete, valid `plugins/deploy.yaml`:

```yaml
# plugins/deploy.yaml
#
# The filename stem ("deploy") MUST equal `name` below.

# REQUIRED. Plugin name = command name: registers `dongle deploy`.
name: deploy

# REQUIRED. Plugin version, valid semver; leading "v" optional.
# Packages are fetched at this version with the "v" stripped (2.3.1).
version: v2.3.1

# OPTIONAL. One line shown by `dongle search`.
shortDescription: Deploy services to the platform

# OPTIONAL (recommended). Compatibility gate, checked at install and at
# every run.
requires:
  # Host version constraint: "" (any), ">=x.y.z", or exact "x.y.z".
  # No ^, ~, <, or ||. Quote it.
  host: ">=0.1.0"
  # Host<->plugin contract version; must exactly match the host's ("v1").
  protocol: "v1"

# REQUIRED. The Azure Artifacts feed the packages are published to.
feed:
  # REQUIRED. Azure DevOps organization name (not a URL).
  organization: acme
  # OPTIONAL. Only for a project-scoped feed; delete this line for an
  # org-scoped feed.
  project: acme-platform
  # REQUIRED. Feed name.
  feed: dongle-plugins
  # REQUIRED. Always "upack" (Universal Package).
  packageType: upack
  # REQUIRED. Base package name for this plugin (a label, not downloaded).
  packageName: dongle-deploy

# REQUIRED. At least one entry; one per os/arch you published.
platforms:
  # selector.os / selector.arch: Go GOOS / GOARCH spelling, exact match.
  # package: the Universal Package name for this os/arch, containing
  # exactly one file (the executable), published at version 2.3.1.
  - selector: { os: darwin, arch: arm64 }
    package: dongle-deploy_2.3.1_darwin_arm64
  - selector: { os: windows, arch: amd64 }
    package: dongle-deploy_2.3.1_windows_amd64

# REQUIRED. Shown by `dongle support deploy`.
support:
  # REQUIRED. Documentation URL.
  documentation: https://docs.acme.internal/dongle-deploy
  # REQUIRED. Support channel URL.
  channel: https://acme.slack.com/archives/C0DEPLOY
  # OPTIONAL. Direct contact.
  contact: deploy-team@acme.example.com
```

---

## CI validation

### When it runs

`azure-pipelines-validate.yml` runs in two modes:

- **On every PR to `develop`** (or `main`) that touches `plugins/*.yaml`.
  It validates the manifests the PR **added, modified, copied, or renamed**.
  Deleted manifests aren't validated, and files ending in `.yml` aren't
  picked up.
- **Nightly (06:00 UTC) on `develop`, plus on demand.** A full scan of every
  `plugins/*.yaml`, to catch drift, for example a package deleted from a
  feed after its manifest was merged.

The pipeline downloads a pinned version of the `validate-manifest` tool and
runs it on the manifests. The tool uses the same manifest parsing as
`dongle install`, so "passes CI" and "dongle can read it" can't drift apart.

### What it checks

Two layers per manifest. If layer 1 fails, layer 2 is skipped for that
manifest.

**Layer 1: correctness.** All errors are reported at once:

| check | error message |
|---|---|
| valid YAML with **no unknown fields** | `invalid manifest at <path>: yaml: unmarshal errors: ... field <x> not found in type ...` |
| `name` present and equals the filename stem | `name is required` / `name "<name>" does not match filename "<file>" (expected name: <stem>)` |
| `version` present and valid semver | `version is required` / `version "<v>" is not a valid semver` |
| `requires.host`, if set, is `>=x.y.z` or `x.y.z` | `requires.host "<c>": invalid semver "<...>"` |
| `feed.organization`, `feed.feed`, `feed.packageType`, `feed.packageName` present | `feed.<field> is required` |
| `support.documentation`, `support.channel` present | `support.<field> is required` |
| at least one `platforms` entry | `platforms: at least one platform entry is required` |
| each entry has `selector.os`, `selector.arch`, `package` | `platforms[<i>].<field> is required` (`<i>` counts from 0) |

**Layer 2: package existence.** For **each** `platforms` entry, the
validator downloads that entry's `package` at the manifest's `version`
(without the `v`) from **the feed the manifest names**. It uses the same
download command as `dongle install`, into a throwaway directory. A
successful download proves the package exists and is downloadable. Each
platform is reported pass or fail.

### What it does not check

**CI validates the manifest and that the packages exist. It does not
validate the plugin itself.** No step runs, inspects, or tests your
executable. In particular, CI does **not** check:

- that the executable runs, or runs on the OS/arch its selector claims;
- that it has no extra dependencies;
- that it follows the host↔plugin contract;
- that each package contains exactly one file (`dongle install` enforces
  this, not CI);
- the value of `requires.protocol` (must be `"v1"`) or `feed.packageType`
  (must be `upack`);
- that `selector.os`/`selector.arch` are real Go platform names;
- that `support` URLs work.

Getting these right is the plugin owner's responsibility.

### Reading the output

The output is in the "Validate manifests" step of the pipeline run linked
from the PR. When everything passes:

```
== plugins/deploy.yaml ==
  correctness: OK
  darwin/arm64 dongle-deploy_2.3.1_darwin_arm64@2.3.1: OK
  windows/amd64 dongle-deploy_2.3.1_windows_amd64@2.3.1: OK

1/1 manifests passed
```

A correctness failure:

```
== plugins/deploy.yaml ==
  correctness: FAIL
    error: name "deploy-tool" does not match filename "deploy.yaml" (expected name: deploy)
    error: support.channel is required

0/1 manifests passed
```

An existence failure:

```
== plugins/deploy.yaml ==
  correctness: OK
  darwin/arm64 dongle-deploy_2.3.1_darwin_arm64@2.3.1: OK
  windows/amd64 dongle-deploy_2.3.1_windows_amd64@2.3.1: FAIL — az artifacts universal download: exit status 1: <error text>

0/1 manifests passed
```

The check passes only if **every** changed manifest passes **both** layers.
For a brand-new manifest file, the pipeline also logs a
`NEW PLUGIN MANIFEST: plugins/<name>.yaml ...` warning (and, where
configured, posts it as a PR comment). This warning is informational and
doesn't fail the build.

### Fixing common failures

| symptom | cause | fix |
|---|---|---|
| existence `FAIL`, package or version not found | package not published yet, or published under a different name or version | publish it, or correct `platforms[].package` / `version` to match the feed exactly (version without `v`) |
| existence `FAIL` with 401 / 403 | CI's build identity can't read the feed | ask the dongle team to grant it Reader on the feed |
| existence `FAIL`, feed or project not found | wrong `feed.organization` / `feed.feed`, or `feed.project` wrong for the feed's scope | fix the `feed` block |
| `name ... does not match filename` | filename and `name` differ | rename the file or fix `name` so they match exactly, including case |
| `... is not a valid semver` | e.g. `2.3`, `v2`, `2.3.1.0` | use `MAJOR.MINOR.PATCH` |
| `field <x> not found in type ...` | misspelled or unsupported key | fix the key; only the fields in [Field summary](#field-summary) exist |
| `<field> is required` | a mandatory field is missing or empty | add it |
| `requires.host ...: invalid semver` | unsupported constraint form | use `""`, `">=x.y.z"`, or `"x.y.z"` |
| pipeline didn't run | PR doesn't target `develop`, or the file isn't `plugins/<name>.yaml` | target `develop`; use the `.yaml` extension |

Push a fix to the same PR branch and CI runs again.

---

## Governance

- Every manifest PR targets **`develop`**.
- A PR can merge only when **CI passes** and it has **2 approvals from the
  dongle team**.
- Releasing a new index version is done **only by the dongle team** (see
  [How an index version is released](#how-an-index-version-is-released)).
- Changes outside `plugins/` (pipelines, `CODEOWNERS`, docs) are made and
  reviewed by the dongle team.

## Common mistakes

- **Manifest PR opened before the packages are published.** CI fails
  existence. Publish every listed platform first, or leave unpublished
  platforms out of `platforms`.
- **Version typo or mismatch.** Check `version` and the package names (they
  often embed the version) against what's actually in the feed.
- **Filename doesn't match `name`**, or the file uses `.yml`.
- **Missing `support.documentation` or `support.channel`.**
- **Misspelled keys.** For example `shortdescription`, `packagetype`,
  `documentaion`, `platform`, `selectors`.
- **Non-Go platform names** like `macos`, `win`, `x86_64`, `aarch64`. They
  pass CI but never match a user.
- **`requires.protocol` not `"v1"`.** It passes CI, but users can't install
  the plugin.
- **More than one file in a package.** It passes CI, but `dongle install`
  fails.
- **"Merged, but `dongle search` doesn't show it."** Merging doesn't release.
  Ask the dongle team for a new index version, then run `dongle update`.
