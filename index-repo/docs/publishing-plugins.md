# Publishing a plugin to the dongle index

This repo is the **plugin index**: one `plugins/<name>.yaml` manifest per
plugin. It is the only thing a plugin author PRs here — the plugin's binary
itself lives in the Azure Artifacts feed the manifest points at, not in this
repo. `dongle` (the host CLI) clones this repo, reads a plugin's manifest to
find its feed coordinates, and downloads the binary from there at install
time.

`dongle` (the host CLI) never clones this repo directly. On every merge to
`main`, `azure-pipelines-publish-index.yml` tags the commit with the next
monotonic `0.0.N` version, archives `plugins/` into `index.tar.gz`, and
publishes it to the shared feed as the `dongle-index` package; `dongle`
downloads and TTL-caches that archive (`dongle refresh` forces an update),
then reads a plugin's manifest out of the cached, extracted copy to find its
feed coordinates at install time.

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

# Where users go for help with this plugin. Surfaced verbatim by
# `dongle support deploy`. documentation and channel are REQUIRED —
# validate-manifest hard-fails a manifest missing either. contact is
# optional.
support:
  documentation: https://docs.acme.internal/dongle-deploy
  channel: https://acme.slack.com/archives/C0DEPLOY
  contact: deploy-team@acme.example.com
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
| `support.documentation` | yes | URL to the plugin's docs; validation fails without it |
| `support.channel` | yes | URL to the plugin's support channel (Slack, issue tracker, etc.); validation fails without it |
| `support.contact` | no | an email/handle for direct contact |

`support.documentation` and `support.channel` are shown verbatim by `dongle
support <plugin-name>`, so users can find help without knowing where your
team hangs out. Both are mandatory — `validate-manifest` hard-fails a
manifest missing either — `support.contact` is optional.

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

## New plugin onboarding

Onboarding a brand-new plugin is a heavier path than a routine version
bump: there's no existing owner yet, no established feed coordinates on
file, and it needs central/platform-team review rather than just your own
sign-off. Walkthrough:

1. **Pick a name.** It must not collide with an existing `dongle <name>`
   builtin or another plugin already in `plugins/`.
2. **Publish before you PR.** Set up (or reuse) the Azure Artifacts feed
   you'll publish to, then build and publish your plugin's per-platform
   packages (see "Publishing your plugin's packages to the feed" above).
   You need at least one platform published before your manifest can pass
   the existence check.
3. **Write the manifest.** Copy the annotated example above into
   `plugins/<name>.yaml`, fill in your own values, and validate it locally
   (see "Validating locally" above) before opening a PR.
4. **Claim ownership.** In the same PR, add a
   `plugins/<name>.yaml -> owning team` line to `CODEOWNERS`. This is what
   turns future version bumps to your plugin into an owner-approved change
   instead of requiring central review every time.
5. **Open the PR.** The validation pipeline detects that this manifest is
   new relative to the target branch and flags it (a pipeline log warning,
   and — where the pipeline is configured for it — a PR comment) so
   reviewers know it needs central/platform-team sign-off, not just a
   per-plugin owner who doesn't exist yet.
6. **Fill in the "New plugin only" section** of the PR template.

Once this merges, every later PR to `plugins/<name>.yaml` is a routine
version bump, approved by the team you just added to `CODEOWNERS` — no
central review needed unless you're adding a platform or changing feed
coordinates in a way reviewers flag as significant.

## Governance

Summarized here for context; see
[CONTRIBUTING.md](../CONTRIBUTING.md#governance) for the authoritative
version:

- **Version bumps** to an existing plugin are approved by that plugin's
  owner(s), as listed in `CODEOWNERS`. Once a bumped `plugins/<name>.yaml`
  merges, `dongle plugin update <name>` is how users pick it up: it compares
  an installed plugin's version against whatever this index currently
  declares and fetches the new one if the index is ahead (refusing to
  downgrade if a user somehow has something newer installed already).
- **New plugins** require central/platform-team review — see the
  walkthrough above.
- Platform coverage is up to each plugin owner — the validator does not
  require every `os`/`arch` to be covered, only that whatever you *do*
  declare is well-formed and actually published.
