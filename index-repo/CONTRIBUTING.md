# Contributing a plugin

How to onboard your plugin to the dongle index, or update one that's
already there.

<!-- TODO: Forge base URL — replace <TODO-FORGE-BASE-URL> in the links below. -->

## Prerequisites

Your plugin must meet all of these before you open a PR:

- [ ] **Checkmarx scan passed** with no critical or high findings, and the
      **report is attached to your PR**.
- [ ] **It's an executable**: one self-contained file per platform.
- [ ] **It needs no other external tools.** It must not rely on runtimes,
      interpreters or other CLIs being installed on the user's machine. If
      it does, **contact the dongle team** before onboarding.
- [ ] **It supports macOS, Windows, or both.** Linux builds are optional,
      but Linux-only plugins aren't accepted.
- [ ] **Its executables are published to the feed** your manifest names, one
      package per platform, at the manifest's version. See the package
      requirements in the
      [Manifest reference](<TODO-FORGE-BASE-URL>/reference/manifest-reference#platformspackage).

## Onboarding steps

### 1. Add your plugin manifest

1. **Fork** this repo.
2. **Add your manifest to the `plugins/` folder** as `plugins/<name>.yaml`.
   The filename must match the `name` field. For an update, edit your
   existing file (usually `version` and the package names).
   - Every field is specified in the
     [Manifest reference](<TODO-FORGE-BASE-URL>/reference/manifest-reference),
     which also has a full annotated example to copy.
   - Walkthrough: [Add your manifest](<TODO-FORGE-BASE-URL>/how-to/plug-in-your-cli/add-your-manifest).
   - Recommended before the PR:
     [Validate your manifest](<TODO-FORGE-BASE-URL>/how-to/plug-in-your-cli/validate-your-manifest)
     and [Run your plugin locally](<TODO-FORGE-BASE-URL>/how-to/plug-in-your-cli/run-locally).
3. **Open a PR to the `develop` branch.** Fill in the PR template and attach
   your Checkmarx report.

### 2. Get the PR merged

A PR is merged only when both of these are true:

- [ ] **The CI pipeline passes.** It validates the structure of the
      manifests you changed (required fields, filename matches `name`,
      valid version, no unknown fields). It also checks that the executable
      package for **each declared platform exists in the feed**. CI doesn't
      run or test your plugin itself. Details:
      [1es-cli-validate-manifest](<TODO-FORGE-BASE-URL>/reference/tools/1es-cli-validate-manifest).
- [ ] **2 approvals from the dongle team.**

### 3. Release a new index version

Merging doesn't make your plugin available. **Ask the dongle team to release
a new index version.** After the release, users get it with
`dongle install <name>` (or `dongle upgrade <name>` for a new version).
