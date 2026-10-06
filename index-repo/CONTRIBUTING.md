# Contributing a plugin

This guide covers how to onboard a plugin to the dongle plugin index, or
update one that's already there. For general information about this repo,
how dongle uses the index, and the full manifest format, see
[README.md](README.md).

## Prerequisites

Before you start, make sure your plugin meets these requirements:

- **Your plugin is a single executable.** dongle runs exactly one file per
  plugin.
- **Your plugin doesn't need any other tools to be installed.** It must not
  rely on interpreters, runtimes, frameworks, or other CLIs on the user's
  machine. If it does need something like that, **contact the dongle team
  before onboarding.**
- **Your executable runs on macOS, Windows, or both.** Build one executable
  per OS/architecture you support. See the
  [supported platforms](README.md#the-plugin).
- **Your executables are published to the feed** your manifest will point
  at, following the [package requirements](README.md#the-packages): one
  file per package, and the package version equal to your manifest
  `version` without the leading `v`. CI checks that these packages exist,
  so publish them before opening the PR.

## Onboarding steps

### 1. Add your plugin manifest

1. **Fork this repo.**
2. **Put your manifest in the `plugins/` folder** as `plugins/<name>.yaml`.
   The filename must equal the `name` field inside it, and `<name>` becomes
   the command users run (`dongle <name>`).
   - Every field is described in the
     [manifest file reference](README.md#manifest-file-reference). The
     easiest start is to copy the
     [full annotated example](README.md#full-annotated-example) and replace
     the values.
   - To **update** an existing plugin, edit its existing file instead.
     Usually you bump `version` and the `platforms[].package` names.
3. **Create a PR from your fork to the `develop` branch.** Fill in the PR
   template checklist.

### 2. Get the PR merged

A PR can be merged only when **both** of these are true:

- **The CI pipeline passes.** It runs automatically on your PR and checks
  the manifests you changed:
  - the manifest is correct: all required fields are present, the filename
    matches `name`, `version` is valid semver, and there are no unknown
    fields;
  - every package your manifest references exists and can be downloaded
    from the feed the manifest names.

  **There is no step that validates your plugin itself.** CI never runs or
  inspects your executable, so it can't tell whether the executable works,
  runs on the platforms you listed, or has extra dependencies. That is your
  responsibility. See [CI validation](README.md#ci-validation) for exactly
  what is and isn't checked, plus how to fix failures.
- **It has 2 approvals from the dongle team.**

### 3. Release a new index version

Merging your PR **doesn't** make your plugin available to users. It reaches
users only after a new version of the index is released.

**Ask the dongle team to release a new index version.** Once it's released,
users can pick up your change with:

```sh
dongle update            # refresh the index now
dongle install <name>    # new plugin
dongle upgrade <name>    # new version of an existing plugin
```

See [How an index version is released](README.md#how-an-index-version-is-released)
for how the release works.
