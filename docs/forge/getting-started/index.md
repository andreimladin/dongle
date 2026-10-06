# Getting started

Go from nothing to running your first plugin in three steps. Follow them in
order; each one depends on the one before.

| step | what you do | time |
|---|---|---|
| [Step 1 — Prerequisites](#step-1--prerequisites) | install the Azure CLI and sign in | ~10 min |
| [Step 2 — Installation](#step-2--installation) | install dongle from Self Service or Company Portal | ~2 min |
| [Step 3 — First run](#step-3--first-run) | run dongle once and check the result | ~1 min |

## Step 1 — Prerequisites

dongle uses the **Azure CLI (`az`)** to download the plugin index and plugins
from Azure Artifacts feeds, so `az` must be installed **and** signed in with
access to those feeds.

1. Install the Azure CLI — Windows: `winget install --exact --id Microsoft.AzureCLI`;
   macOS: `brew install azure-cli`.
2. Add the extension dongle needs: `az extension add --name azure-devops`.
3. Sign in: `az login`, with an account that has **Reader** access on the
   dongle feeds.

**Done when:** `az version` works and `az login` succeeded.
Details, CI setup and feed access: **[Prerequisites](prerequisites.md)**.

## Step 2 — Installation

Install dongle from your company's app portal:

- **macOS:** open **Self Service**, search for **dongle**, click **Install**.
- **Windows:** open **Company Portal**, search for **dongle**, click **Install**.

To update dongle later, use the same portal.

**Done when:** `dongle --version` prints a version in a new terminal.
Details: **[Installation](installation.md)**.

## Step 3 — First run

Run any dongle command, for example:

```sh
dongle --version
```

The first time, dongle sets itself up — it unpacks its built-in plugin index
and installs the default plugins (no network needed) — then runs your
command. You'll see a few `✓` lines ending in `✓ Initialization complete.`

**Done when:** `dongle --version` shows an index version and the default
plugins under `installed plugins:`.
What happens and what you'll see: **[Your first run](first-run.md)**.

## Next steps

- Find and install plugins: [Use dongle commands](../how-to/use-commands.md)
- Understand what's going on: [Concepts](../explanation/concepts.md)
- Something not working: [Troubleshooting](../troubleshooting/troubleshooting.md)
