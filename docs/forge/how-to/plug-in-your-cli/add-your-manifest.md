# Add your manifest

This page shows how to submit your plugin to the dongle index, or update one
that's already there. Every field is specified in the
[Manifest reference](../../reference/manifest-reference.md). This page
doesn't repeat it.

**Before you start**

- Your plugin meets the requirements in the index repo's CONTRIBUTING.md
  <!-- TODO: index repo URL --> (`<TODO-INDEX-REPO-URL>/CONTRIBUTING.md`),
  including a passing Checkmarx scan.
- Your executables are **already published** to the feed, one package per
  platform, at the version you're about to declare. CI checks that they
  exist.

## Steps

### 1. Fork the index repo

Fork the dongle index repo and clone your fork.
<!-- TODO: index repo URL -->

### 2. Create or edit `plugins/<name>.yaml`

- **New plugin:** create `plugins/<name>.yaml`. The filename (without
  `.yaml`) must equal the `name` field, and it becomes the command
  (`dongle <name>`). The quickest start is to copy the
  [full annotated example](../../reference/manifest-reference.md#full-annotated-example)
  and replace the values.
- **New version of an existing plugin:** edit its file. Usually you bump
  `version` and update each `platforms[].package` to the packages you just
  published.

Only touch your own plugin's manifest. The rest of the repo is maintained
by the dongle team.

### 3. Validate it

Run the validator against your file and fix anything it reports. See
[Validate your manifest](validate-your-manifest.md).

To also check that your plugin installs and runs through dongle, see
[Run your plugin locally](run-locally.md).

### 4. Open a PR to `develop`

Commit, push to your fork, and open a pull request **into the index repo's
`develop` branch**. Fill in the PR template and **attach your Checkmarx
report**.

### 5. Get it merged

The PR can be merged once:

- **CI passes**. It validates the manifests your PR changed and checks
  that every package they reference exists in the feed. If it fails, read
  the log, fix, and push again. CI re-runs automatically.
- **2 members of the dongle team approve.**

### 6. Ask for an index release

Merging doesn't make your plugin available. **Ask the dongle team to release
a new index version.** After the release, users get your plugin with
`dongle install <name>`, or the new version with `dongle upgrade <name>`.
