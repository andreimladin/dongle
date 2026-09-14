# Publishing a plugin to the dongle index

This repo is the **plugin index**: one `plugins/<name>.yaml` manifest per
plugin. It is the only thing a plugin author PRs here — the plugin's binary
itself lives in the Azure Artifacts feed the manifest points at, not in this
repo. `dongle` (the host CLI) clones this repo, reads a plugin's manifest to
find its feed coordinates, and downloads the binary from there at install
time.

There is no JSON Schema for the manifest yet, so this document — plus the
annotated example below — is the source of truth for the format. If it and
`validate-manifest`'s behavior ever disagree, treat `validate-manifest` as
correct and file a docs bug.

## The manifest format

A manifest is `plugins/<name>.yaml`. The **filename is the plugin name** —
it's what registers `dongle <name>`, so `plugins/deploy.yaml` must have
`name: deploy` inside it.

```yaml
# plugins/deploy.yaml
#
# Filename (minus .yaml) MUST equal `name` below — this is what registers
# `dongle deploy`.
name: deploy

# The plugin's own version. Must be valid semver; a leading "v" is
# accepted and stripped before it's used as the Azure Artifacts package
# version (upack versions are bare semver, no "v").
version: v2.3.1

# One-line description shown by `dongle plugin search`.
shortDescription: Deploy services to the platform

# Compatibility gate, checked by the host before install AND before every
# dispatch (dongle's internal/compat is the single source of truth for
# this logic — see dongle's own README for the constraint syntax dongle
# supports: "", ">=x.y.z", or an exact "x.y.z". No ^, ~, <, or ||.).
requires:
  host: ">=0.1.0"    # minimum dongle host version this plugin needs
  protocol: "v1"     # the host<->plugin connector-spec version this plugin speaks

# Where dongle downloads this plugin's binaries from: one Azure Artifacts
# feed, shared across plugins, with one Universal Package per
# plugin+platform.
feed:
  organization: acme          # Azure DevOps organization
  project: acme-platform       # OMIT this line entirely for an org-scoped feed;
                                # set it only if your feed is project-scoped
  feed: dongle-plugins          # the feed name
  packageType: upack            # Azure Artifacts Universal Package — the only type dongle supports today
  packageName: dongle-deploy     # a plugin-publisher convention (see below) — dongle doesn't parse it

# One entry per os/arch you've built and published a package for. You do
# NOT need to cover every platform — dongle only requires a build for
# whatever platform a given install actually runs on, and errors clearly
# at install time if a user's platform has no entry here.
platforms:
  - selector: { os: darwin, arch: arm64 }
    package: dongle-deploy_2.3.1_darwin_arm64   # the Universal Package name for THIS platform
  - selector: { os: linux, arch: amd64 }
    package: dongle-deploy_2.3.1_linux_amd64
```

Field-by-field:

| field | required | notes |
|---|---|---|
| `name` | yes | must equal the filename stem |
| `version` | yes | valid semver, `v` prefix optional |
| `shortDescription` | no | shown by `dongle plugin search`; omitting it just leaves that column blank |
| `requires.host` | no | `""` means "any host version" |
| `requires.protocol` | no | `""` means "any protocol" — you almost always want to set this |
| `feed.organization` | yes | Azure DevOps org |
| `feed.project` | only for project-scoped feeds | omit entirely for an org-scoped feed |
| `feed.feed` | yes | the feed name |
| `feed.packageType` | yes | `upack` — Universal Package, the only type dongle's downloader supports |
| `feed.packageName` | yes | your own naming convention (see below); dongle doesn't parse it |
| `platforms[].selector.os` / `.arch` | yes, per entry | Go `GOOS`/`GOARCH` spelling (`darwin`, `linux`, `windows`; `amd64`, `arm64`) |
| `platforms[].package` | yes, per entry | the Universal Package name published for **this** os/arch |

Unknown top-level or nested fields are rejected by `validate-manifest`
(strict parsing) — it's almost always a typo, so treat that error as "fix
the field name," not "the validator is wrong."

## Publishing your plugin's packages to the feed

1. Build your plugin's binary for each `os`/`arch` you want to support.
2. Publish **each platform's binary as its own Universal Package**, one file
   per package:

   ```sh
   az artifacts universal publish \
     --organization https://dev.azure.com/acme \
     --feed dongle-plugins \
     --name dongle-deploy_2.3.1_linux_amd64 \
     --version 2.3.1 \
     --path ./dist/linux_amd64
   ```

   The package name (`--name`) is entirely your own convention —
   `<packageName>_<version>_<os>_<arch>` (as above) keeps feed browsing
   sane, but dongle never parses it. What dongle *does* require: each
   package contains **exactly one file**, the binary itself — at install
   time dongle downloads it, takes that one file, and renames it to a
   canonical entrypoint regardless of what you called it inside the
   package.
3. Repeat per platform. You do not need every platform built before your
   first PR — see the `platforms:` note above.

## Validating locally

Before opening a PR, download `validate-manifest` (published by the dongle
repo's own tooling pipeline) from the feed and run it against your
manifest:

```sh
az artifacts universal download \
  --organization https://dev.azure.com/acme \
  --feed TODO-tooling-feed-name \
  --name validate-manifest-linux-amd64 \
  --version TODO-pin-a-version \
  --path ./bin
chmod +x ./bin/validate-manifest-linux-amd64

./bin/validate-manifest-linux-amd64 plugins/deploy.yaml
```

<!-- TODO: fill in the real feed name and a current validate-manifest
version above once they're set (see azure-pipelines-validate.yml in this
repo, and azure-pipelines-tooling.yml in the dongle repo). -->

This runs the exact same two checks the PR pipeline runs: manifest
correctness (strict parse, required fields, valid semver) and, since your
`az` session already needs to be logged in to read the feed, package
existence (confirms each platform's package actually exists at the version
your manifest declares). A clean local run means the PR pipeline should
pass too, modulo anything that changes on the feed between now and then.

## Opening the manifest PR

1. Add or update `plugins/<name>.yaml` (this is the only file you touch for
   a normal publish — no code lives in this repo).
2. Open a PR against `main`. The PR pipeline
   (`azure-pipelines-validate.yml`) automatically validates whatever
   `plugins/*.yaml` files your PR changed, and flags a brand-new manifest
   file for the new-plugin review below.
3. Fill in the PR template's checklist.

## Governance

- **Version bumps to an existing plugin** are approved by that plugin's
  owner(s), as listed in `CODEOWNERS`.
- **New plugins** need central/platform-team review in addition to (or
  instead of, if there isn't an owner yet) a per-plugin owner — the PR
  pipeline flags any `plugins/*.yaml` that's new relative to the target
  branch so reviewers know to apply this. Once a new plugin is approved,
  add its `plugins/<name>.yaml -> owning team` line to `CODEOWNERS` as part
  of that same PR.
- Platform coverage is up to each plugin owner — the validator does not
  require every `os`/`arch` to be covered, only that whatever you *do*
  declare is well-formed and actually published.

## Getting help

<!-- TODO: link your team's support channel / docs here (Slack channel,
Teams channel, internal wiki page, etc.) — parked until those exist. -->
