# Contributing to the dongle plugin index

This is the single, complete guide for adding a plugin to the dongle plugin
index or updating one. You should not need any other document. If this guide
and the `validate-manifest` tool ever disagree, the tool wins: its behavior
is described below and is what CI enforces. Please report the mismatch so
this file can be fixed.

**Contents**

1. [Overview](#1-overview)
2. [Prerequisites](#2-prerequisites)
3. [Step-by-step contribution flow](#3-step-by-step-contribution-flow)
4. [Manifest file reference](#4-manifest-file-reference)
5. [How CI validation works](#5-how-ci-validation-works)
6. [Common mistakes and troubleshooting](#6-common-mistakes-and-troubleshooting)

---

## 1. Overview

### What this repo is

This repo is the **dongle plugin index**. `dongle` is a host CLI that other
CLIs plug into: a user runs `dongle install <name>`, and from then on
`dongle <name> [args...]` runs that plugin. The index is the catalog that
tells `dongle` which plugins exist and where to download each one.

The index is a directory of manifests, one YAML file per plugin:

```
plugins/
  deploy.yaml      # registers `dongle deploy`
  <name>.yaml      # registers `dongle <name>`
```

The repo holds **only manifests**. It contains no plugin source code and no
binaries. Your plugin's binaries live as Universal Packages in an Azure
Artifacts feed, and your manifest tells `dongle` where that feed is and which
package to download for each operating system and CPU architecture.

### What contributing means

Contributing means **adding or updating one file, `plugins/<name>.yaml`**,
so that your plugin becomes installable (or upgradable) through `dongle`.
There are two kinds of contribution:

| kind | what changes | who approves |
|---|---|---|
| **New plugin** | a new `plugins/<name>.yaml` file, plus a `CODEOWNERS` line | central/platform team |
| **Update** (version bump, new platform, feed change) | edits to an existing `plugins/<name>.yaml` | that plugin's owner(s) listed in `CODEOWNERS` |

### How your manifest reaches users

`dongle` never reads this git repo directly. The path from a merged manifest
to a user's machine is:

1. Your PR merges into `main`.
2. A maintainer publishes a new **index version**. They do this by running
   the `azure-pipelines-publish-index.yml` pipeline by hand against a
   `release/X.Y.Z` branch. The pipeline packs `plugins/` into `index.tar.gz`
   and publishes it to the shared feed as the `dongle-index` package, at
   version `X.Y.Z`. Merging to `main` does **not** publish anything by itself.
3. Each user's `dongle` downloads the latest `dongle-index` and caches it for
   a while. Users can refresh it immediately with `dongle update`.
4. `dongle install <name>` reads your manifest from the cached index, checks
   compatibility, and downloads the package for the user's platform from
   your feed. `dongle upgrade <name>` installs the newer version once the
   index declares one. It never downgrades.

So a merged manifest becomes visible to users once the next index version is
published and their cache refreshes.

---

## 2. Prerequisites

Have all of the following ready before you open a PR.

### 2.1 Tools and access

- **Git**, and permission to push a branch to this repo and open a pull
  request against `main`. If you can't push here, ask the platform team (the
  owners on the `*` line of `CODEOWNERS`) for contributor access.
- **Azure CLI (`az`)** with the Azure DevOps extension:

  ```sh
  az extension add --name azure-devops
  ```

- **Authentication to Azure DevOps**, either through an interactive session:

  ```sh
  az login
  ```

  or through a personal access token in the environment. `az artifacts`
  commands read it automatically:

  ```sh
  export AZURE_DEVOPS_EXT_PAT=<your-PAT-with-Packaging-read-write-scope>
  ```

- **Feed permissions.** You need write access (Contributor) on the Azure
  Artifacts feed you will publish your plugin to. Separately, the CI build
  identity of this index repo must have **Reader** access on that same feed,
  or CI's package-existence check fails with 401/403 errors. A new feed needs
  this permission set up once. Ask the platform team if you're unsure.

### 2.2 A built plugin

Build your plugin as **one standalone executable per platform** you want to
support. Platforms use Go's `GOOS`/`GOARCH` names, for example
`darwin/arm64`, `linux/amd64`, `windows/amd64`. You don't need to cover every
platform. Ship the ones you support and add more later.

Your plugin must follow dongle's host↔plugin contract (protocol `v1`): it is
run as a short-lived child process with the user's terminal inherited, its
arguments are everything after the plugin name, and its exit code is passed
back to the user unchanged. Credentials and context are provided through
`DONGLE_*` environment variables. That contract belongs to the dongle host,
not to this repo. This guide only covers getting the plugin listed.

### 2.3 Packages published to the feed, before the PR

**Publish your packages first, then open the manifest PR.** CI downloads
every package your manifest references and fails if any is missing, so the
packages must already be in the feed.

Publish **each platform's binary as its own Universal Package**:

```sh
# Org-scoped feed
az artifacts universal publish \
  --organization https://dev.azure.com/acme \
  --feed dongle-plugins \
  --name dongle-deploy_2.3.1_linux_amd64 \
  --version 2.3.1 \
  --path ./dist/linux_amd64

# Project-scoped feed: add --project and --scope project
az artifacts universal publish \
  --organization https://dev.azure.com/acme \
  --project acme-platform --scope project \
  --feed dongle-plugins \
  --name dongle-deploy_2.3.1_darwin_arm64 \
  --version 2.3.1 \
  --path ./dist/darwin_arm64
```

Rules for each package:

- **Exactly one file.** The directory you pass to `--path` must contain only
  the plugin executable. If `dongle install` downloads a package with zero
  files or more than one, it fails with `package <name>@<version> contains N
  files, expected exactly 1`. The file's name inside the package doesn't
  matter, because dongle renames it to `dongle-<plugin name>` at install time.
- **Version is bare semver.** `--version` must be the manifest's `version`
  with any leading `v` removed. For example, manifest `version: v2.3.1` means
  publish `--version 2.3.1`.
- **Name is up to you.** You choose `--name` (it becomes the manifest's
  `platforms[].package` value). The recommended convention is
  `<packageName>_<version>_<os>_<arch>`, e.g.
  `dongle-deploy_2.3.1_linux_amd64`. dongle never parses this name, so it's
  only a convention, but it keeps the feed easy to browse.
- **All platforms share one version.** Every package referenced by one
  manifest is fetched at the manifest's single `version`, so publish every
  platform at the same version.

Write down for the manifest: the organization, the project (only for a
project-scoped feed), the feed name, and the package name of each platform.

---

## 3. Step-by-step contribution flow

### a. Create a branch off `main`

```sh
git clone <this-repo-clone-url> dongle-index   # skip if you already have it
cd dongle-index
git checkout main
git pull origin main
git checkout -b plugin/<name>-<version>         # e.g. plugin/deploy-2.3.1
```

### b. Add or update `plugins/<name>.yaml`

The **filename stem must be exactly the plugin name**. `plugins/deploy.yaml`
must contain `name: deploy`, and it registers the command `dongle deploy`.

- **New plugin:** create `plugins/<name>.yaml`. Start from the full annotated
  example in [§4.3](#43-full-annotated-example).
  - Pick a name that is not already a file in `plugins/` and is not a dongle
    builtin command: `search`, `install`, `remove`, `upgrade`, `update`,
    `support`. Also avoid `help` and anything starting with `-`.
  - In the same branch, add an ownership line to `CODEOWNERS`, below the
    existing per-plugin lines:

    ```
    plugins/<name>.yaml                 @your-owning-team
    ```

    This line turns your future version bumps into owner-approved changes
    instead of central reviews.
- **Update:** edit the existing file. A version bump usually means changing
  `version` and each `platforms[].package` (if your package names include
  the version). You can also add new `platforms` entries.

### c. Fill in the manifest

Fill in every field following the [manifest file reference](#4-manifest-file-reference).
For a quick recap of the requirements:

- `name`, `version`, `feed.organization`, `feed.feed`, `feed.packageType`,
  `feed.packageName`, `support.documentation`, `support.channel`, and at
  least one `platforms` entry with `selector.os`, `selector.arch`, and
  `package`.
- No fields other than those documented. Unknown fields fail validation.

Then run the validator locally, as described in
[§5.4](#54-running-the-validator-locally). This step is optional but strongly
recommended.

### d. Commit, push, and open a PR to `main`

```sh
git add plugins/<name>.yaml           # plus CODEOWNERS for a new plugin
git commit -m "plugins: add <name> <version>"     # or "plugins: bump <name> to <version>"
git push -u origin plugin/<name>-<version>
```

Open the pull request against `main`. This repo is hosted on Azure DevOps.
Either use the "Create a pull request" prompt shown for your pushed branch
in the web UI, or use the CLI:

```sh
az repos pr create \
  --organization https://dev.azure.com/<org> \
  --project <project> \
  --repository <this-repo-name> \
  --source-branch plugin/<name>-<version> \
  --target-branch main \
  --title "plugins: add <name> <version>" \
  --description "New plugin <name> <version>. Platforms: linux/amd64, darwin/arm64."
```

Fill in the PR description using the repo's PR template checklist:

- `validate-manifest` passed locally for the changed manifest(s).
- Packages for **every** platform in the manifest are published to the feed
  named in `feed:`, at the version in `version:`.
- `version:` matches what was actually published.
- `name:` matches the filename.
- **New plugin only:** a `CODEOWNERS` line was added, central/platform-team
  review was requested, and the name doesn't collide with an existing
  `dongle <name>` command.

### e. CI validates automatically

Opening (or pushing to) a PR that changes any `plugins/*.yaml` file runs the
validation pipeline automatically. It checks every changed manifest for
correctness and confirms that every package it references exists in the
feed. Details, sample output, and fixes are in
[§5](#5-how-ci-validation-works). If it fails, fix the problem, commit, and
push to the same branch. CI then runs again.

### f. Review, approval, and merge

- **Update to an existing plugin:** approved by that plugin's owner(s) in
  `CODEOWNERS`.
- **New plugin:** CI flags the new file, and the central/platform team must
  review it (see [§5.6](#56-governance-new-plugins-vs-updates)).

Once CI is green and the required approvals are in, the PR is merged into
`main`. Your plugin reaches users with the **next index publish**, which a
maintainer runs from a `release/X.Y.Z` branch (see
[§1](#how-your-manifest-reaches-users)). After that, users get it with:

```sh
dongle update              # refresh the cached index now (optional)
dongle search              # your plugin and its shortDescription appear here
dongle install <name>      # new plugin
dongle upgrade <name>      # existing users, after a version bump
dongle support <name>      # shows your support block
```

---

## 4. Manifest file reference

A manifest is a YAML file at `plugins/<name>.yaml`. It has exactly seven
top-level keys: `name`, `version`, `shortDescription`, `requires`, `feed`,
`platforms`, `support`. There is no separate JSON Schema. This section, the
manifest data type in dongle, and the `validate-manifest` checks are the
whole definition.

**Parsing is strict.** The validator rejects any key that isn't listed
below, at any nesting level. This catches typos like `shortDesc` or
`documentaion`, so if you see an "unknown field" error, fix the key name.

**All values are strings** (except `platforms`, which is a list). Quote
values YAML might misread, especially constraints beginning with `>`, e.g.
`host: ">=0.1.0"`.

### 4.1 Field summary

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
| `platforms[].selector` | **required** per entry | mapping | `{ os: linux, arch: amd64 }` |
| `platforms[].selector.os` | **required** per entry | string | `linux` |
| `platforms[].selector.arch` | **required** per entry | string | `amd64` |
| `platforms[].package` | **required** per entry | string | `dongle-deploy_2.3.1_linux_amd64` |
| `support` | **required** | mapping | |
| `support.documentation` | **required** | string (URL) | `https://docs.acme.internal/dongle-deploy` |
| `support.channel` | **required** | string (URL) | `https://acme.slack.com/archives/C0DEPLOY` |
| `support.contact` | optional | string | `deploy-team@acme.example.com` |

### 4.2 Field details

#### `name` (required, string)

The plugin's name and its command: `name: deploy` registers `dongle deploy`.
It **must equal the manifest's filename without `.yaml`**. The validator
fails with `name "X" does not match filename "Y.yaml" (expected name: Y)`
otherwise. Use lowercase letters, digits, and hyphens. The name must not
collide with a dongle builtin (`search`, `install`, `remove`, `upgrade`,
`update`, `support`) or an existing plugin.

```yaml
name: deploy
```

#### `version` (required, string)

The plugin's own version. It must be valid semver,
`MAJOR.MINOR.PATCH` with optional `-prerelease` and `+build` suffixes. An
optional leading `v` is allowed. Exact rule (regex used by the validator):

```
^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-.]+)?(\+[0-9A-Za-z-.]+)?$
```

- Valid: `2.3.1`, `v2.3.1`, `1.0.0-rc.1`, `1.0.0+build.5`
- Invalid: `2.3`, `v2`, `2.3.1.4`, `02.3.1`, `version-2.3.1`

The leading `v` is stripped before the version is used to download packages,
so `v2.3.1` means every platform package must exist at version `2.3.1` in
the feed. `dongle upgrade` compares this field with the installed version to
decide whether an upgrade is available. Bump it every time you publish new
binaries.

```yaml
version: v2.3.1
```

#### `shortDescription` (optional, string)

A one-line summary shown in the `dongle search` listing. If you leave it
out, that column is blank. Keep it short and on one line.

```yaml
shortDescription: Deploy services to the platform
```

#### `requires` (optional, mapping)

The compatibility gate. dongle checks it when installing **and** before every
run of the plugin. If the check fails, the plugin is not installed or run,
and the user sees the reason.

- **`requires.host`** (optional, string): the dongle host versions this
  plugin works with. Exactly three forms are supported:

  | form | meaning |
  |---|---|
  | `""` or omitted | any host version |
  | `">=x.y.z"` | host version x.y.z or newer |
  | `"x.y.z"` | exactly host version x.y.z |

  Ranges like `^1.2.0`, `~1.2.0`, `<2.0.0`, or `a || b` are **not
  supported**. If `host` is set, the validator checks that it parses (fails
  with `requires.host "...": invalid semver "..."`). Always quote it.

- **`requires.protocol`** (optional, string): the host↔plugin contract version
  your plugin speaks. The current dongle host speaks **`v1`**. The value
  must match the host's protocol exactly, or install and run are refused.
  An empty or omitted value means "any protocol", which is rarely what you
  want. Set it to `"v1"`. The validator does **not** check this value, so a
  typo here (e.g. `"1"` or `"V1"`) passes CI but makes your plugin
  uninstallable for everyone.

```yaml
requires:
  host: ">=0.1.0"
  protocol: "v1"
```

#### `feed` (required, mapping)

Where your plugin's packages live: an Azure Artifacts feed. These
coordinates are used both by `dongle install` and by CI's existence check,
which always checks **the feed your manifest names**, not some global feed.

- **`feed.organization`** (required, string): the Azure DevOps organization
  name only, not a URL. dongle builds `https://dev.azure.com/<organization>`
  from it. Example: `acme`.
- **`feed.project`** (optional, string): set this **only if the feed is
  project-scoped**. When set, downloads add `--project <project> --scope
  project`. For an organization-scoped feed, **leave the line out entirely**.
  Setting it for an org-scoped feed (or omitting it for a project-scoped
  one) makes the download fail. Example: `acme-platform`.
- **`feed.feed`** (required, string): the feed name. Example:
  `dongle-plugins`.
- **`feed.packageType`** (required, string): the package format. Use
  **`upack`** (Azure Artifacts Universal Package), the only type dongle
  downloads. The validator only checks that this field is non-empty, but
  every download is a Universal Package download, so any other value is
  wrong.
- **`feed.packageName`** (required, string): your plugin's base package
  name, a publisher-side label for the plugin across all its per-platform
  packages, conventionally `dongle-<name>`. It must be non-empty. dongle
  doesn't use it to download anything; the per-platform
  `platforms[].package` values are what actually get fetched. Example:
  `dongle-deploy`.

```yaml
feed:
  organization: acme
  project: acme-platform   # omit this line for an org-scoped feed
  feed: dongle-plugins
  packageType: upack
  packageName: dongle-deploy
```

#### `platforms` (required, list, at least one entry)

One entry per OS/architecture you built and published a package for. You
don't need to cover every platform. A user on a platform with no entry gets
a clear error at install time (`<name> <version> has no build for
<os>/<arch>`). The validator only checks the entries you declare; it doesn't
require any particular coverage. An empty or missing list fails with
`platforms: at least one platform entry is required`.

Each entry has:

- **`selector`** (required, mapping) with:
  - **`os`** (required, string): the Go `GOOS` value, e.g. `linux`,
    `darwin`, `windows`.
  - **`arch`** (required, string): the Go `GOARCH` value, e.g. `amd64`,
    `arm64`.

  dongle picks the entry whose `os`/`arch` **exactly equals** the user's
  platform (case-sensitive). The validator only checks that both fields are
  non-empty, not that they're real platform names, so a value like `macos`,
  `x86_64`, or `Linux` passes CI but never matches any user.
- **`package`** (required, string): the exact Universal Package name you
  published for this platform (the `--name` you passed to `az artifacts
  universal publish`). It is downloaded at the manifest's `version`, without
  the `v`. It must contain exactly one file.

List each `os`/`arch` pair once. If a pair is listed twice, only the first
entry is ever used.

```yaml
platforms:
  - selector: { os: darwin, arch: arm64 }
    package: dongle-deploy_2.3.1_darwin_arm64
  - selector: { os: linux, arch: amd64 }
    package: dongle-deploy_2.3.1_linux_amd64
```

Block style is equivalent to the inline `{ os: ..., arch: ... }` form:

```yaml
platforms:
  - selector:
      os: linux
      arch: amd64
    package: dongle-deploy_2.3.1_linux_amd64
```

#### `support` (required, mapping)

Where users get help. `dongle support <name>` shows it as written.

- **`support.documentation`** (required, string): URL of your plugin's
  documentation. Missing it fails validation with
  `support.documentation is required`.
- **`support.channel`** (required, string): URL of your support channel (a
  chat channel, issue tracker, etc.). Missing it fails validation with
  `support.channel is required`.
- **`support.contact`** (optional, string): an email address or handle for
  direct contact.

```yaml
support:
  documentation: https://docs.acme.internal/dongle-deploy
  channel: https://acme.slack.com/archives/C0DEPLOY
  contact: deploy-team@acme.example.com
```

### 4.3 Full annotated example

A complete, valid `plugins/deploy.yaml`. Copy it, then replace every value
with your own.

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

# REQUIRED. The Azure Artifacts feed your packages are published to.
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
  # package: the Universal Package name published for this os/arch,
  # containing exactly one file (the executable), at version 2.3.1.
  - selector: { os: darwin, arch: arm64 }
    package: dongle-deploy_2.3.1_darwin_arm64
  - selector: { os: linux, arch: amd64 }
    package: dongle-deploy_2.3.1_linux_amd64

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

## 5. How CI validation works

### 5.1 When it runs

The validation pipeline (`azure-pipelines-validate.yml` at the repo root)
runs in two modes:

- **On every PR to `main` that touches `plugins/*.yaml`.** It validates only
  the manifests the PR **added, modified, copied, or renamed** compared with
  `main`. Deleted manifests aren't validated. A PR that doesn't touch
  `plugins/*.yaml` doesn't trigger validation.
- **Nightly (06:00 UTC) on `main`, plus on demand.** A full scan of every
  `plugins/*.yaml`, to catch drift, for example a package deleted from a feed
  after its manifest was merged. If your plugin starts failing the nightly
  scan, the platform team will contact you through your `CODEOWNERS` entry
  or `support` details.

The pipeline downloads a pinned version of the `validate-manifest` tool and
runs it on the manifests. All the validation logic lives in that tool, and it
uses the same manifest parsing as `dongle install` itself, so "passes CI"
and "dongle can read it" can't drift apart.

### 5.2 What it checks

Each manifest goes through two layers, in order. If layer 1 fails, layer 2
is skipped for that manifest.

**Layer 1: correctness.** Every error found is reported at once:

| check | error message |
|---|---|
| File parses as YAML and contains **no unknown fields** (at any level) | `invalid manifest at <path>: yaml: unmarshal errors: ... field <x> not found in type ...` |
| `name` present | `name is required` |
| `name` equals filename stem | `name "<name>" does not match filename "<file>" (expected name: <stem>)` |
| `version` present | `version is required` |
| `version` is valid semver (see [§4.2](#version-required-string)) | `version "<v>" is not a valid semver` |
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

`platforms[<i>]` counts from 0, so `platforms[1]` is the second entry.

**Layer 2: package existence.** For **each** `platforms` entry, the validator
downloads that entry's `package` at the manifest's `version` (leading `v`
stripped) **from the feed the manifest itself names**: `feed.organization`,
`feed.feed`, and `feed.project` if set. It uses the same
`az artifacts universal download` command `dongle install` uses. The download
goes to a throwaway directory that's deleted immediately. A successful
download proves the package exists and can be downloaded. Every platform is
reported individually, pass or fail.

**What is *not* checked** (get these right yourself, see
[§4.2](#42-field-details)):

- `requires.protocol`'s value. It must be `"v1"` for the current host.
- `feed.packageType`'s value. It must be `upack`.
- Whether `selector.os`/`selector.arch` are real Go platform names.
- Whether a package contains exactly one file. dongle enforces this at
  install time, not CI.
- Whether URLs in `support` work.
- Which platforms you cover.

### 5.3 What you see on pass or fail

The validator's output appears in the "Validate manifests" step of the
pipeline run linked from your PR. A passing run looks like:

```
== plugins/deploy.yaml ==
  correctness: OK
  darwin/arm64 dongle-deploy_2.3.1_darwin_arm64@2.3.1: OK
  linux/amd64 dongle-deploy_2.3.1_linux_amd64@2.3.1: OK

1/1 manifests passed
```

A correctness failure (layer 2 skipped):

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
  linux/amd64 dongle-deploy_2.3.1_linux_amd64@2.3.1: FAIL — az artifacts universal download: exit status 1: <az error text>

0/1 manifests passed
```

The pipeline (and the PR check) passes only if **every** changed manifest
passes **both** layers. The validator's exit codes are `0` (all passed), `1`
(at least one manifest failed, or existence couldn't be confirmed), and `2`
(usage error).

For a new plugin, the pipeline also logs a warning
`NEW PLUGIN MANIFEST: plugins/<name>.yaml — new plugins need central/platform-team review ...`
and, where configured, posts the same text as a PR comment. This warning is
informational: it doesn't fail the build.

### 5.4 Running the validator locally

Run the same check before you open the PR. The tool is published as a
Universal Package named `validate-manifest-linux-amd64`. The feed and version
CI uses are pinned in this repo's `azure-pipelines-validate.yml` as the
`TOOLING_FEED` and `VALIDATE_MANIFEST_VERSION` variables. Use the same
values:

```sh
az artifacts universal download \
  --organization https://dev.azure.com/<org> \
  --feed <TOOLING_FEED> \
  --name validate-manifest-linux-amd64 \
  --version <VALIDATE_MANIFEST_VERSION> \
  --path ./bin
chmod +x ./bin/*

# Validate specific files (what PR CI does)
./bin/validate-manifest* plugins/<name>.yaml

# Or scan the whole directory (what the nightly scan does)
./bin/validate-manifest* --dir plugins/
```

Requirements: `az` on your `PATH`, the `azure-devops` extension, and an
authenticated session (`az login` or `AZURE_DEVOPS_EXT_PAT`) with read access
to your plugin's feed. Without `az`, the validator prints
`warning: az CLI not found on PATH` and **every existence check fails**. A
clean local run means CI should pass, unless something changes in the feed in
between, or CI's build identity lacks Reader access to your feed (see
[§2.1](#21-tools-and-access)).

### 5.5 Fixing common failures

| symptom | cause | fix |
|---|---|---|
| `<os>/<arch> <pkg>@<ver>: FAIL — ... not found` (or similar "does not exist" error) | Package not published yet, published under a different name, or at a different version | Publish the package (see [§2.3](#23-packages-published-to-the-feed-before-the-pr)), or correct `platforms[].package` / `version` to match exactly what's in the feed. Remember the version is checked **without** the `v`. Push again. |
| Existence `FAIL` with `401` / `403` / unauthorized | CI's build identity can't read your feed | Have a feed admin grant the index repo's build service identity **Reader** on the feed. Your own access doesn't matter to CI. |
| Existence `FAIL` mentioning the feed or project not being found | Wrong `feed.organization` / `feed.feed`, or `feed.project` set for an org-scoped feed (or missing for a project-scoped one) | Fix the `feed` block. Delete `project:` for org-scoped feeds; add it for project-scoped feeds. |
| `name "X" does not match filename "Y.yaml"` | Filename and `name` differ | Rename the file to `plugins/<name>.yaml` (`git mv`), or fix `name`, so they match exactly, including case. |
| `version "..." is not a valid semver` | e.g. `2.3`, `v2`, `2.3.1.0`, leading zeros | Use `MAJOR.MINOR.PATCH`, optionally with a `v` prefix and `-pre`/`+build` suffixes. |
| `field <x> not found in type ...` | Misspelled or unsupported key | Correct the key name. Only the fields in [§4.1](#41-field-summary) exist. |
| `<field> is required` | A mandatory field is missing or empty | Add it. See [§4.1](#41-field-summary). |
| `requires.host "...": invalid semver` | Unsupported constraint form (`^`, `~`, `<`, `||`, `>= 1.0`, missing patch) | Use `""`, `">=x.y.z"`, or `"x.y.z"`. |
| Pipeline didn't run at all | PR doesn't target `main`, or doesn't change any `plugins/*.yaml` | Target `main`; make sure your manifest is at `plugins/<name>.yaml` with a `.yaml` extension (not `.yml`). |

After fixing, commit and push to the same branch. CI re-runs automatically.

### 5.6 Governance: new plugins vs. updates

Ownership is declared in `CODEOWNERS` (last matching line wins). The `*` line
is the central/platform team, and each plugin has its own
`plugins/<name>.yaml @owning-team` line. (On Azure DevOps, these rules are
applied as path-scoped required-reviewer branch policies.)

- **Updates to an existing plugin** (version bumps, added platforms, feed
  changes) are **approved by that plugin's owner(s)** from its `CODEOWNERS`
  line. Reviewers may still ask for central review of a change they consider
  significant, such as moving to a different feed.
- **New plugins** need **central/platform-team review**, in addition to (or,
  until the plugin has an owner line, instead of) a per-plugin owner. CI
  detects any `plugins/*.yaml` that doesn't exist on `main` yet and flags it
  with the `NEW PLUGIN MANIFEST` warning (and PR comment, where configured),
  so reviewers know to apply this. The new-plugin PR must also add the
  plugin's `CODEOWNERS` line. Once it merges, future updates are routine and
  owner-approved.

Changes outside `plugins/` (pipelines, `CODEOWNERS` itself, this file) fall
under the `*` line and are reviewed by the platform team.

---

## 6. Common mistakes and troubleshooting

- **Opening the manifest PR before publishing the packages.** CI downloads
  every referenced package and fails on any that are missing. Publish
  **all** platforms listed in the manifest first, then open the PR. Leave a
  platform out of `platforms` until its package is published.
- **Version typo or mismatch.** `version: v2.3.1` means every package must
  exist at `2.3.1`. Publishing at `2.3.10`, or forgetting to bump
  `version` while updating the `package` names, fails existence. Also check
  the `package` names themselves, since they often embed the version.
- **Filename doesn't match `name`.** `plugins/Deploy.yaml` with
  `name: deploy`, `plugins/deploy.yml`, or `plugins/deploy-cli.yaml` with
  `name: deploy` are all wrong. Use exactly `plugins/<name>.yaml`.
- **Missing required support fields.** `support.documentation` and
  `support.channel` are both mandatory; only `contact` is optional.
- **Misspelled keys.** Strict parsing rejects unknown fields. Watch for
  `shortdescription` (must be `shortDescription`), `packagetype`,
  `documentaion`, `platform` vs `platforms`, and `selectors`.
- **`feed.project` on an org-scoped feed** (or missing on a project-scoped
  one). Downloads fail for CI and for users.
- **Non-Go platform names.** `macos`, `win`, `x86_64`, `aarch64`, and `Linux`
  pass CI but never match a user's machine. Use `darwin`, `windows`,
  `linux`, `amd64`, `arm64`.
- **`requires.protocol` typo.** `"1"`, `"V1"`, or `"v2"` pass CI, but the
  current host refuses to install or run the plugin. Use `"v1"`.
- **Unsupported `requires.host` range.** `^1.0.0`, `~1.0.0`, and
  `>=1.0.0 <2.0.0` are rejected. Use `">=x.y.z"`, an exact `"x.y.z"`, or
  `""`.
- **Unquoted `>=` constraint.** Always write `host: ">=0.1.0"` with quotes.
- **More than one file in a package.** CI accepts it, but `dongle install`
  fails with `contains N files, expected exactly 1`. Publish a directory
  holding only the executable.
- **Forgetting `CODEOWNERS` on a new plugin.** Without your line, every
  future bump falls back to central review.
- **"Merged, but `dongle search` doesn't show it."** A merge doesn't publish
  the index. Your change appears after a maintainer runs the next index
  publish and users refresh with `dongle update`.
- **"`dongle install` says no build for my platform."** No `platforms` entry
  has `os`/`arch` exactly equal to that machine's `GOOS`/`GOARCH`. Add an
  entry and publish its package.
- **Need help?** Mention the platform team (the owners on the `*` line of
  `CODEOWNERS`) in your PR.
