# Contributing to the dongle plugin index

This repo is the **dongle plugin index**: one `plugins/<name>.yaml` manifest
per plugin. The manifest is the only thing you PR here — your plugin's
binaries live in the Azure Artifacts feed the manifest points at, not in
this repo.

## How to contribute a plugin

At a high level:

1. Build your plugin's binary for each `os`/`arch` you want to support, and
   publish each one as its own Universal Package to the feed.
2. Open a PR adding (new plugin) or updating (version bump)
   `plugins/<name>.yaml`.

Full step-by-step guide, including the manifest format field-by-field with
an annotated example: **[docs/publishing-plugins.md](docs/publishing-plugins.md)**.

## Validation

Every PR that touches `plugins/*.yaml` is validated automatically
(`azure-pipelines-validate.yml`): mandatory fields, valid semver, and — for
each platform your manifest declares — that the package it names actually
exists at the stated version in the feed the manifest itself points at. A
nightly full scan re-checks every manifest the same way, to catch drift
(e.g. a package removed from the feed after its manifest was merged).

Run the same check locally before opening a PR — see
[docs/publishing-plugins.md](docs/publishing-plugins.md#validating-locally)
for how to download the validator from the feed and run it against your
manifest.

## Governance

- **Version bumps** to an existing plugin are approved by that plugin's
  owner(s), as listed in [CODEOWNERS](CODEOWNERS).
- **New plugins** require central/platform-team review in addition to (or
  instead of, if there's no owner yet) a per-plugin owner. The validation
  pipeline flags any `plugins/*.yaml` that's new relative to the target
  branch so reviewers know to apply this — see the new-plugin walkthrough in
  [docs/publishing-plugins.md](docs/publishing-plugins.md#new-plugin-onboarding).

## Getting help

<!-- TODO: link your team's support channel / docs here (Slack channel,
Teams channel, internal wiki page, etc.) — parked until those exist. -->
