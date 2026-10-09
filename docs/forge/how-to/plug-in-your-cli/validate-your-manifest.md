# Validate your manifest

This page shows how to check your manifest with **1es-cli-validate-manifest**
before opening a PR. It's the same tool, running the same checks, that the
index repo's CI runs on your PR. If it passes locally, CI should pass too.
For its full usage, flags and the list of checks, see the
[1es-cli-validate-manifest reference](../../reference/tools/1es-cli-validate-manifest.md).

**Before you start:** have the Azure CLI installed, with the
`azure-devops` extension, and signed in with read access to your plugin's
feed. See [Prerequisites](../../getting-started/index.md#step-1--prerequisites). The tool
uses it to check that your packages exist.

## 1. Get the tool

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
     (VALIDATE_MANIFEST_VERSION in the index repo's
     azure-pipelines-validate.yml). If the packages are renamed to
     1es-cli-validate-manifest-<os>-<arch>, update the names here. -->

**Option B: run it from the dongle source** (any OS, including Windows;
needs Go installed). From a clone of the dongle repo:

```sh
go run ./tools/validate-manifest <path-to>/plugins/<name>.yaml
```

## 2. Run it on your manifest

From the root of your index-repo fork:

```sh
./bin/validate-manifest-darwin-arm64 plugins/<name>.yaml
```

## 3. Read the result

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
  described in the [Manifest reference](../../reference/manifest-reference.md).
  The package checks don't run until the manifest itself is correct.
- **A platform line ends in `FAIL — ...`**: that package couldn't be
  downloaded. Usually it hasn't been published yet, or the name, version or
  feed in your manifest doesn't match what's in the feed. The `az` error
  text follows the dash.

Fix, re-run, and repeat until you see `1/1 manifests passed`.

## What validation doesn't cover

The tool checks your manifest and that your packages exist. It doesn't run
your plugin. To check that it installs and works through dongle, see
[Run your plugin locally](run-locally.md).
