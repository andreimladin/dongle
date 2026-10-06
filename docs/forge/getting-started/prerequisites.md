# Prerequisites

dongle doesn't download anything itself. It calls the **Azure CLI (`az`)**
to read the plugin index and to download plugins from Azure Artifacts
feeds. Before using dongle, you need:

1. The Azure CLI, installed.
2. The Azure CLI's `azure-devops` extension.
3. An `az` session signed in to an account that can **read the feeds**
   dongle downloads from.

Without these, commands that reach the feed (`search`, `install`,
`upgrade`, `update`) fail with:

```
error: the Azure CLI is required: install it and run `az extension add --name azure-devops`
```

## 1. Install the Azure CLI

### Windows

Install with **winget** (from PowerShell or Command Prompt):

```powershell
winget install --exact --id Microsoft.AzureCLI
```

Or download and run the Azure CLI MSI installer from Microsoft. Afterwards,
**open a new terminal** so `az` is on your `PATH`.

### macOS

Install with **Homebrew**:

```sh
brew update && brew install azure-cli
```

### Check the install

```sh
az version
```

## 2. Add the `azure-devops` extension

dongle uses `az artifacts` (to download) and `az devops` (to check for the
latest index version). Both come from this extension:

```sh
az extension add --name azure-devops
```

If it's already installed, `az extension update --name azure-devops`
brings it up to date.

## 3. Sign in

```sh
az login
```

This opens a browser to sign in. Use the account that has access to the
Azure DevOps organization hosting the dongle feeds. dongle uses whatever
session `az` already has. It never asks for credentials and never stores
any.

### Feed access

Your account needs at least **Reader** access on:

- the feed that holds the **plugin index** (the `dongle-index` package);
- the feed each plugin you install is published to (named in that plugin's
  manifest).
<!-- TODO: name the feed(s) and how to request access (e.g. an access
     request link or group to join). -->

If you can't read a feed, the `az` error (for example 401, 403, or "not
found") is shown under dongle's error message.

### Non-interactive use (CI)

In pipelines, skip `az login` and set a personal access token with
**Packaging (Read)** scope instead. `az artifacts` and `az devops` pick it
up automatically:

```sh
export AZURE_DEVOPS_EXT_PAT=<token>        # macOS/Linux
$env:AZURE_DEVOPS_EXT_PAT = "<token>"      # Windows PowerShell
```

## 4. Get dongle

Each dongle release is published as one package per platform, named
`dongle-<os>-<arch>`: `dongle-darwin-arm64`, `dongle-darwin-amd64`,
`dongle-windows-amd64`, `dongle-windows-arm64` (Linux builds exist too).
<!-- TODO: host feed name, organization, recommended install location, and
     the exact download command for each OS. -->

Put the binary on your `PATH` as `dongle` (`dongle.exe` on Windows), then
check that it runs:

```sh
dongle --version
```

Next: [Your first run](first-run.md).
