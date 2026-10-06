# Manifest reference

The complete specification of a plugin manifest: `plugins/<name>.yaml` in
the dongle index repo. **This page is the single source of truth for
manifest fields.** Other pages link here instead of repeating it.

What a manifest is for is explained in
[Concepts](../explanation/concepts.md#what-is-a-manifest). How to submit one
is in [Add your manifest](../how-to/plug-in-your-cli/add-your-manifest.md).

## Format rules

- **Location and name:** `plugins/<name>.yaml` in the index repo, with the
  extension `.yaml` (not `.yml`). The filename stem must equal the `name`
  field.
- **YAML, parsed strictly.** The manifest has exactly seven top-level keys:
  `name`, `version`, `shortDescription`, `requires`, `feed`, `platforms`,
  `support`. Any key not listed on this page, at any level, is rejected by
  validation (`field <x> not found in type ...`).
- **All values are strings**, except `platforms`, which is a list. Quote
  values YAML could misread, especially constraints that start with `>`:
  `host: ">=0.1.0"`.
- "Required" below means validation fails if the field is missing or empty.

## Field summary

| field | required | type | example |
|---|---|---|---|
| [`name`](#name) | **required** | string | `deploy` |
| [`version`](#version) | **required** | string (semver) | `v2.3.1` |
| [`shortDescription`](#shortdescription) | optional | string | `Deploy services to the platform` |
| [`requires`](#requires) | optional | mapping | |
| [`requires.host`](#requireshost) | optional | string (constraint) | `">=0.1.0"` |
| [`requires.protocol`](#requiresprotocol) | optional (set it) | string | `"v1"` |
| [`feed`](#feed) | **required** | mapping | |
| [`feed.organization`](#feedorganization) | **required** | string | `acme` |
| [`feed.project`](#feedproject) | project-scoped feeds only | string | `acme-platform` |
| [`feed.feed`](#feedfeed) | **required** | string | `dongle-plugins` |
| [`feed.packageType`](#feedpackagetype) | **required** | string | `upack` |
| [`feed.packageName`](#feedpackagename) | **required** | string | `dongle-deploy` |
| [`platforms`](#platforms) | **required**, at least 1 entry | list | |
| [`platforms[].selector.os`](#platformsselector) | **required** per entry | string | `darwin` |
| [`platforms[].selector.arch`](#platformsselector) | **required** per entry | string | `arm64` |
| [`platforms[].package`](#platformspackage) | **required** per entry | string | `dongle-deploy_2.3.1_darwin_arm64` |
| [`support`](#support) | **required** | mapping | |
| [`support.documentation`](#supportdocumentation) | **required** | string (URL) | `https://docs.acme.internal/dongle-deploy` |
| [`support.channel`](#supportchannel) | **required** | string (URL) | `https://acme.slack.com/archives/C0DEPLOY` |
| [`support.contact`](#supportcontact) | optional | string | `deploy-team@acme.example.com` |

## Fields

### `name`

**Required, string.** The plugin's name, which is also its command:
`name: deploy` registers `dongle deploy`.

- Must equal the filename without `.yaml`. Otherwise validation fails with
  `name "X" does not match filename "Y.yaml" (expected name: Y)`.
- Use lowercase letters, digits and hyphens.
- Must not collide with an existing plugin or a dongle builtin: `search`,
  `install`, `remove`, `upgrade`, `update`, `support`. Builtins always take
  precedence, so a plugin with a builtin's name could never be run. Also
  avoid `help` and anything starting with `-`.

### `version`

**Required, string.** The plugin's own version, in semver:
`MAJOR.MINOR.PATCH`, with optional `-prerelease` and `+build` suffixes and
an optional leading `v`. The exact rule:

```
^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[0-9A-Za-z-.]+)?(\+[0-9A-Za-z-.]+)?$
```

- Valid: `2.3.1`, `v2.3.1`, `1.0.0-rc.1`, `1.0.0+build.5`
- Invalid: `2.3`, `v2`, `2.3.1.4`, `02.3.1`, `version-2.3.1`

How it's used:

- every `platforms[].package` is downloaded **at this version, with the
  leading `v` removed** (`v2.3.1` → `2.3.1`);
- `dongle search` shows it, also without the `v`;
- `dongle upgrade` compares it with the installed version
  (`MAJOR.MINOR.PATCH`, numerically) and upgrades only when the index is
  newer.

Bump it every time you publish new executables.

### `shortDescription`

**Optional, string.** A one-line summary shown by `dongle search`. If
omitted, the column is blank.

### `requires`

**Optional, mapping.** The compatibility gate. dongle checks it at install,
at upgrade, and before every run. On failure, the plugin isn't
installed or run, and the user sees the reason.

#### `requires.host`

**Optional, string.** The dongle versions the plugin works with:

| value | meaning |
|---|---|
| `""` or omitted | any dongle version |
| `">=x.y.z"` | dongle `x.y.z` or newer |
| `"x.y.z"` | exactly dongle `x.y.z` |

No other forms are supported: no `^`, `~`, `<`, `||`, or ranges. If set,
validation checks that it parses (`requires.host "...": invalid semver
"..."`). On failure, users see `<name> requires host <c> but this host is
<v>`. A developer build of dongle (version `dev`) can't satisfy any
non-empty constraint.

#### `requires.protocol`

**Optional, string. Set it to `"v1"`.** The version of the host↔plugin
contract the plugin speaks. The current dongle speaks `v1`, and the value
must match exactly (case-sensitive). Empty or omitted means "any protocol".
On mismatch, users see `<name> speaks protocol "<p>" but this host supports
"v1"`.

Validation does **not** check this value. A typo such as `"1"` or `"V1"`
passes CI but makes the plugin impossible to install or run.

### `feed`

**Required, mapping.** The Azure Artifacts feed that holds the plugin's
packages. `dongle install` and validation both download from **the feed
named here**.

#### `feed.organization`

**Required, string.** The Azure DevOps organization *name*, not a URL.
dongle uses `https://dev.azure.com/<organization>`.

#### `feed.project`

**Set only for a project-scoped feed; omit the line entirely for an
organization-scoped feed.** When set, downloads add `--project <project>
--scope project`. Getting this wrong either way makes downloads fail.

#### `feed.feed`

**Required, string.** The feed name.

#### `feed.packageType`

**Required, string. Always `upack`** (Azure Artifacts Universal Package), the
only package type dongle downloads. Validation only checks that it's
non-empty.

#### `feed.packageName`

**Required, string.** The plugin's base package name, a label for its family
of per-platform packages, conventionally `dongle-<name>`. Must be
non-empty. Nothing is downloaded by this name. The `platforms[].package`
values are what get fetched.

### `platforms`

**Required, list with at least one entry** (`platforms: at least one
platform entry is required`). One entry per OS/architecture you have
published a package for. dongle picks the entry whose `os` and `arch`
exactly match the user's machine. If none matches, the install fails with
`<name> <version> has no build for <os>/<arch>`.

**Platform policy:** a plugin must support **macOS, Windows, or both**.
Linux entries are optional, and a Linux-only plugin isn't accepted.

#### `platforms[].selector`

**Required per entry, mapping** with two **required** string keys, using Go
`GOOS`/`GOARCH` spelling (lowercase, exact match):

| platform | `os` | `arch` |
|---|---|---|
| macOS, Apple silicon | `darwin` | `arm64` |
| macOS, Intel | `darwin` | `amd64` |
| Windows, x64 | `windows` | `amd64` |
| Windows, ARM64 | `windows` | `arm64` |
| Linux, x64 (optional) | `linux` | `amd64` |
| Linux, ARM64 (optional) | `linux` | `arm64` |

Validation only checks that `os` and `arch` are non-empty. Values like
`macos`, `win`, `x86_64`, `aarch64` or `Darwin` pass CI but never match any
user. List each OS/arch pair once. If a pair appears twice, only the first
entry is used.

Inline and block YAML forms are equivalent:

```yaml
- selector: { os: darwin, arch: arm64 }
  package: dongle-deploy_2.3.1_darwin_arm64
- selector:
    os: windows
    arch: amd64
  package: dongle-deploy_2.3.1_windows_amd64
```

#### `platforms[].package`

**Required per entry, string.** The exact Universal Package name published
for this platform in `feed`. Requirements on the package itself:

- **published before the manifest PR**: validation downloads each one and
  fails if it's missing;
- **at version `version` without the leading `v`**. All of a plugin's
  platform packages therefore share one version;
- **contains exactly one file: the executable.** Otherwise
  `dongle install` fails with `package <p>@<v> contains N files, expected
  exactly 1`. The file's own name doesn't matter, because dongle installs it
  under a fixed name;
- **the executable is self-contained**: no other tools or runtimes needed
  on the user's machine.

The name is your choice. dongle doesn't parse it. The recommended convention
is `<packageName>_<version>_<os>_<arch>`, e.g.
`dongle-deploy_2.3.1_windows_amd64`.

### `support`

**Required, mapping.** Where users get help with the plugin. `dongle support
<name>` shows it as written.

#### `support.documentation`

**Required, string.** URL of the plugin's documentation.

#### `support.channel`

**Required, string.** URL of the plugin's support channel (chat channel,
issue tracker, etc.).

#### `support.contact`

**Optional, string.** An email address or handle for direct contact.

## Full annotated example

A complete, valid `plugins/deploy.yaml`:

```yaml
# plugins/deploy.yaml
#
# The filename stem ("deploy") MUST equal `name` below.

# REQUIRED. Plugin name = command name: registers `dongle deploy`.
name: deploy

# REQUIRED. Plugin version, semver; leading "v" optional.
# Packages are fetched at this version with the "v" stripped (2.3.1).
version: v2.3.1

# OPTIONAL. One line shown by `dongle search`.
shortDescription: Deploy services to the platform

# OPTIONAL (recommended). Checked at install, upgrade and every run.
requires:
  # dongle version constraint: "" (any), ">=x.y.z", or exact "x.y.z". Quote it.
  host: ">=0.1.0"
  # Host<->plugin protocol; must be exactly "v1" (not validated by CI).
  protocol: "v1"

# REQUIRED. Azure Artifacts feed holding the packages.
feed:
  # REQUIRED. Azure DevOps organization name (not a URL).
  organization: acme
  # Project-scoped feeds ONLY; delete this line for an org-scoped feed.
  project: acme-platform
  # REQUIRED. Feed name.
  feed: dongle-plugins
  # REQUIRED. Always "upack" (Universal Package).
  packageType: upack
  # REQUIRED. Base package name (a label; not downloaded).
  packageName: dongle-deploy

# REQUIRED. At least one entry; macOS and/or Windows required, Linux optional.
platforms:
  # selector: Go GOOS/GOARCH spelling, exact match against the user's machine.
  # package: Universal Package for this os/arch, at version 2.3.1,
  #          containing exactly one file (the executable).
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

## Validation summary

Exactly what CI checks, with every error message, is in the
[1es-cli-validate-manifest reference](tools/1es-cli-validate-manifest.md#what-it-checks).
In short: strict parsing, required fields, `name` = filename, valid
`version`, a parseable `requires.host`, and the existence of every
`platforms[].package` in `feed`. Not checked: `requires.protocol`,
`feed.packageType` and `selector` values, the package's file count and
contents, URLs, and the platform policy (that's checked in review).
