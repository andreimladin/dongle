# Getting started

Three steps to a working dongle.

## Step 1 — Prerequisites

dongle uses the Azure CLI to download plugins, so you need it installed and
signed in.

1. Install the Azure CLI:
   - **Windows:** `winget install --exact --id Microsoft.AzureCLI`
   - **macOS:** `brew install azure-cli`
2. Add the Azure DevOps extension:

   ```sh
   az extension add --name azure-devops
   ```

3. Sign in:

   ```sh
   az login
   ```

<!-- TODO: how to request access to the dongle feeds, if not granted by default. -->

## Step 2 — Installation

- **macOS:** open **Self Service**, search for **dongle**, and click
  **Install** (or **Update**).
- **Windows:** open **Company Portal**, search for **dongle**, and click
  **Install** (or **Update**).

## Step 3 — First run

Open a new terminal and run:

```sh
dongle --version
```

The first time, dongle sets itself up: it unpacks its plugin index and
installs the default plugins. This happens only once and takes a few
seconds:

```
✓ Initialized plugin index 1.4.0
✓ Installed tacho 1.2.0
✓ Installed bell 0.3.1
✓ Initialization complete.
dongle    1.0.0
index     1.4.0 (embedded)

installed plugins:
  bell    0.3.1
  tacho   1.2.0
```

(Your versions and default plugins may differ.)

You're ready. Next, find and install plugins:
[Use dongle commands](../how-to/use-commands.md).
