# Contributing a plugin

How to make your CLI available to every dongle user, by onboarding it to
the dongle index or updating one that's already there. Once your plugin is
in a released index, anyone can run `dongle install <your-cli>` and then
`dongle <your-cli> ...`.

New to dongle? Read [Concepts](<TODO-FORGE-BASE-URL>/explanation/concepts)
first. Every manifest field is specified in the
[Manifest reference](docs/manifest-reference.md); this page doesn't repeat
it.

<!-- TODO: Forge base URL — replace <TODO-FORGE-BASE-URL> in the links below. -->

## Overview

1. [Build and publish your executables](#1-build-and-publish-your-executables)
2. [Add your manifest](#2-add-your-manifest)
3. [Validate your manifest](#3-validate-your-manifest)
4. [Run your plugin locally](#4-run-your-plugin-locally) (recommended)
5. [Open a PR to `develop`](#5-open-a-pr-to-develop)
6. [Get the PR merged](#6-get-the-pr-merged)
7. [Ask for an index release](#7-ask-for-an-index-release)

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
- [ ] **It works as a dongle plugin.** dongle runs it as a child process
      following the host↔plugin contract in
      [How dongle works](<TODO-FORGE-BASE-URL>/explanation/design#running-a-plugin-one-process-one-shot).
      An existing CLI usually needs no changes. It just has to make sense
      when invoked as `dongle <name> <args>`.

## 1. Build and publish your executables

Publish one executable per supported platform to an Azure Artifacts feed,
as Universal Packages, at the version you're about to declare in your
manifest. The package rules (one file per package, versioning, naming) are
in the Manifest reference under
[`platforms[].package`](docs/manifest-reference.md#platformspackage). CI
checks that the packages exist, so publish them **before** opening the PR.

## 2. Add your manifest

1. **Fork** this repo and clone your fork.
2. **Create or edit `plugins/<name>.yaml`:**
   - **New plugin:** create `plugins/<name>.yaml`. The filename (without
     `.yaml`) must equal the `name` field, and it becomes the command
     (`dongle <name>`). The quickest start is to copy the
     [full annotated example](docs/manifest-reference.md#full-annotated-example)
     and replace the values.
   - **New version of an existing plugin:** edit its file. Usually you bump
     `version` and update each `platforms[].package` to the packages you
     just published.

Only touch your own plugin's manifest. The rest of the repo is maintained
by the dongle team.

## 3. Validate your manifest

Check your manifest with **1es-cli-validate-manifest**. It's the same tool,
running the same checks, that CI runs on your PR, so if it passes locally,
CI should pass too. Its full usage, flags and list of checks are in the
[1es-cli-validate-manifest reference](docs/1es-cli-validate-manifest.md).

**Before you start:** have the Azure CLI installed, with the
`azure-devops` extension, and signed in with read access to your plugin's
feed (see
[Prerequisites](<TODO-FORGE-BASE-URL>/getting-started#step-1--prerequisites)).
The tool uses it to check that your packages exist.

### Get the tool

Choose one option.

**Option A: download a published build** (macOS on Apple silicon, or
Linux x64):

```sh
az artifacts universal download \
  --organization https://dev.azure.com/<TODO-org> \
  --feed <TODO-tooling-feed> \
  --name validate-manifest-darwin-arm64 \
  --version <TODO-version> \
  --path ./bin
chmod +x ./bin/validate-manifest-darwin-arm64
```

Use `--name validate-manifest-linux-amd64` on Linux.
<!-- TODO: tooling feed name and the validator version CI currently pins
     (VALIDATE_MANIFEST_VERSION in azure-pipelines-validate.yml). If the
     packages are renamed to 1es-cli-validate-manifest-<os>-<arch>, update
     the names here. -->

**Option B: run it from the dongle source** (any OS, including Windows;
needs Go installed). From a clone of the dongle repo:

```sh
go run ./tools/validate-manifest <path-to>/plugins/<name>.yaml
```

### Run it

From the root of your fork:

```sh
./bin/validate-manifest-darwin-arm64 plugins/<name>.yaml
```

### Read the result

When everything passes:

```
== plugins/deploy.yaml ==
  correctness: OK
  darwin/arm64 dongle-deploy_2.3.1_darwin_arm64@2.3.1: OK
  windows/amd64 dongle-deploy_2.3.1_windows_amd64@2.3.1: OK

1/1 manifests passed
```

The exit code is `0`. Anything else needs fixing:

- **`correctness: FAIL`**: each `error:` line names a field. Fix it as
  described in the [Manifest reference](docs/manifest-reference.md).
  The package checks don't run until the manifest itself is correct.
- **A platform line ends in `FAIL — ...`**: that package couldn't be
  downloaded. Usually it hasn't been published yet, or the name, version or
  feed in your manifest doesn't match what's in the feed. The `az` error
  text follows the dash.

Fix, re-run, and repeat until you see `1/1 manifests passed`.

The validator checks your manifest and that your packages exist. It
doesn't run your plugin. For that, see the next step.

## 4. Run your plugin locally

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

macOS:

```sh
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

macOS:

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

## 5. Open a PR to `develop`

Commit, push to your fork, and open a pull request **into this repo's
`develop` branch**. Fill in the PR template and **attach your Checkmarx
report**.

## 6. Get the PR merged

A PR is merged only when both of these are true:

- [ ] **The CI pipeline passes.** It validates the manifests your PR
      changed (required fields, filename matches `name`, valid version, no
      unknown fields) and checks that the executable package for **each
      declared platform exists in the feed**. CI doesn't run or test your
      plugin itself. If it fails, read the log, fix, and push again; CI
      re-runs automatically. Details:
      [1es-cli-validate-manifest](docs/1es-cli-validate-manifest.md#in-the-index-repos-ci).
- [ ] **2 approvals from the dongle team.**

## 7. Ask for an index release

Merging doesn't make your plugin available. **Ask the dongle team to release
a new index version** (see
[How the index is released](README.md#how-the-index-is-released)). After
the release, users get your plugin with `dongle install <name>`, or the new
version with `dongle upgrade <name>`.
