# Contributing a plugin

How to make your CLI available to every dongle user, by onboarding it to
the dongle index or updating one that's already there. Once your plugin is
in a released index, anyone can run `dongle install <your-cli>` and then
`dongle <your-cli> ...`.

New to dongle? Read [Concepts](<TODO-FORGE-BASE-URL>/explanation/concepts)
first. Every manifest field is specified in the
[Manifest reference](README.md#manifest-reference); this page doesn't repeat
it.

<!-- TODO: Forge base URL — replace <TODO-FORGE-BASE-URL> in the links below. -->

## Overview

1. [Build and publish your executables](#1-build-and-publish-your-executables)
2. [Add your manifest](#2-add-your-manifest)
3. [Run your plugin locally](#3-run-your-plugin-locally) (recommended)
4. [Open a PR to `develop`](#4-open-a-pr-to-develop)
5. [Get the PR merged](#5-get-the-pr-merged)
6. [Ask for an index release](#6-ask-for-an-index-release)

## Prerequisites

Your plugin must meet all of these before you open a PR:

- [ ] **Checkmarx scan passed** with no critical or high findings, and the
      **report is attached to your PR**.
- [ ] **It's an executable**: one self-contained file per platform.
- [ ] **It needs no other external tools.** It must not rely on runtimes,
      interpreters or other CLIs being installed on the user's machine. If
      it does, **contact the dongle team** before onboarding.
- [ ] **It provides executables for macOS, Windows and Linux.** All three
      are mandatory: your manifest needs at least one `platforms` entry for
      each, and CI fails the PR if any of them is missing or its package
      isn't in the feed. See the
      [platform policy](README.md#platforms).
- [ ] **It works as a dongle plugin.** dongle runs it as a child process
      following the host↔plugin contract in
      [How dongle works](<TODO-FORGE-BASE-URL>/explanation/design#running-a-plugin-one-process-one-shot).
      An existing CLI usually needs no changes. It just has to make sense
      when invoked as `dongle <name> <args>`.

## 1. Build and publish your executables

Publish one executable per platform — at least one each for macOS,
Windows and Linux — to an Azure Artifacts feed,
as Universal Packages, at the version you're about to declare in your
manifest. The package rules (one file per package, versioning, naming) are
in the Manifest reference under
[`platforms[].package`](README.md#platformspackage). CI
checks that the packages exist, so publish them **before** opening the PR.

## 2. Add your manifest

1. **Fork** this repo and clone your fork.
2. **Create or edit `plugins/<name>.yaml`:**
   - **New plugin:** create `plugins/<name>.yaml`. The filename (without
     `.yaml`) must equal the `name` field, and it becomes the command
     (`dongle <name>`). The quickest start is to copy the
     [full annotated example](README.md#full-annotated-example)
     and replace the values.
   - **New version of an existing plugin:** edit its file. Usually you bump
     `version` and update each `platforms[].package` to the packages you
     just published.

Only touch your own plugin's manifest. The rest of the repo is maintained
by the dongle team.

## 3. Run your plugin locally

Recommended: test your plugin through dongle on your own machine
**before** opening the PR, using your real manifest and your published
packages. Your normal dongle setup stays untouched.

**Before you start**

- A **released** dongle binary. A developer build (`dongle --version` shows
  `dev`) rejects any manifest that sets `requires.host`.
- The Azure CLI, signed in (see
  [Prerequisites](<TODO-FORGE-BASE-URL>/getting-started#step-1--prerequisites)).
- Your plugin's packages published to the feed named in your manifest.

### Quick check: run the executable directly

dongle runs your executable with your arguments and three extra
environment variables. You can do the same yourself:

macOS and Linux:

```sh
chmod +x ./your-executable    # if it isn't executable yet
DONGLE_VERSION=1.0.0 DONGLE_PROTOCOL=v1 DONGLE_PLUGIN_NAME=<name> \
  ./your-executable --help
```

Windows (PowerShell):

```powershell
$env:DONGLE_VERSION="1.0.0"; $env:DONGLE_PROTOCOL="v1"; $env:DONGLE_PLUGIN_NAME="<name>"
.\your-executable.exe --help
```

### Full check: install and run through dongle

You'll create a throwaway dongle data folder that holds a "local index"
with just your manifest. dongle then installs from that local index instead
of the real one.

**1. Create a sandbox data folder with a local index.**

macOS and Linux:

```sh
export DONGLE_DATA_DIR="$PWD/dongle-sandbox"
mkdir -p "$DONGLE_DATA_DIR/index/plugins"
cp plugins/<name>.yaml "$DONGLE_DATA_DIR/index/plugins/"
printf 0.0.0-local > "$DONGLE_DATA_DIR/index.version"
```

Windows (PowerShell):

```powershell
$env:DONGLE_DATA_DIR = "$PWD\dongle-sandbox"
New-Item -ItemType Directory -Force "$env:DONGLE_DATA_DIR\index\plugins" | Out-Null
Copy-Item plugins\<name>.yaml "$env:DONGLE_DATA_DIR\index\plugins\"
Set-Content -NoNewline "$env:DONGLE_DATA_DIR\index.version" "0.0.0-local"
```

`DONGLE_DATA_DIR` points dongle at the sandbox instead of your real
`~/.dongle`. It applies only to the current terminal.

**2. Install from the local index.** Always pass `--no-sync`. Without it,
dongle checks the feed, sees that the real index is newer than
`0.0.0-local`, and offers to replace your local index:

```sh
dongle search --no-sync               # your plugin should be listed
dongle install <name> --no-sync       # downloads YOUR package for this OS/arch from the feed
```

This exercises the same steps a user's install does: the compatibility
check (`requires`), picking the `platforms` entry for your OS/arch, and
downloading and installing the package.

**3. Run it.**

```sh
dongle <name> --help
dongle <name> <your usual arguments>
dongle support <name>                 # shows your support block
dongle --version                      # lists <name> with its version
```

Check that prompts, colors and exit codes behave as expected.

**4. Iterate and clean up.** After changing the manifest, copy it into the
sandbox again, then run `dongle remove <name>` followed by
`dongle install <name> --no-sync`.

When you're done, delete the sandbox folder and close the terminal (or
unset `DONGLE_DATA_DIR`).

### Common failures

| message | meaning |
|---|---|
| `bad constraint in manifest: invalid semver "dev"` | you're using a developer build of dongle; use a released one |
| `<name> requires host >=X but this host is Y` | your `requires.host` is newer than your dongle |
| `<name> speaks protocol "..." but this host supports "v1"` | `requires.protocol` must be `"v1"` |
| `<name> <version> has no build for <os>/<arch>` | no `platforms` entry matches this machine |
| `az download <package>@<version>: ...` | the package isn't in the feed under that name/version, or you can't read the feed |
| `package ... contains N files, expected exactly 1` | the package must contain only the executable |

For every dongle command and flag, see the
[Command reference](<TODO-FORGE-BASE-URL>/reference).

## 4. Open a PR to `develop`

Commit, push to your fork, and open a pull request **into this repo's
`develop` branch**. Fill in the PR template and **attach your Checkmarx
report**.

## 5. Get the PR merged

A PR is merged only when both of these are true:

- [ ] **The CI pipeline passes.** It validates the manifests your PR
      changed, **fails any manifest without a macOS, Windows and Linux
      entry**, and checks that the executable package for **each declared
      platform exists in the feed** (see
      [What CI checks](README.md#what-ci-checks)). CI doesn't run or test
      your plugin itself.
- [ ] **2 approvals from the dongle team.**

If CI fails, read the validation step's log:

- **`correctness: FAIL`**: each `error:` line names a field. Fix it as
  described in the [Manifest reference](README.md#manifest-reference). The
  package checks don't run until the manifest itself is correct. A
  `manifest <name>.yaml is missing a <OS> platform entry` error means you
  need to add a `platforms` entry for that OS.
- **A platform line ends in `FAIL — ...`**: that package couldn't be
  downloaded. Usually it hasn't been published yet, or the name, version or
  feed in your manifest doesn't match what's in the feed. The `az` error
  text follows the dash.

Fix and push again; CI re-runs automatically.

## 6. Ask for an index release

Merging doesn't make your plugin available. **Ask the dongle team to release
a new index version** (see
[How the index is released](README.md#how-the-index-is-released)). After
the release, users get your plugin with `dongle install <name>`, or the new
version with `dongle upgrade <name>`.
