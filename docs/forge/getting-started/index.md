# Getting started

Three steps to a working dongle.

## Step 1 — Prerequisites

dongle uses the Azure CLI to download plugins, so you need it installed and
signed in.

1. Install the Azure CLI:
   - **Windows:** `winget install --exact --id Microsoft.AzureCLI`
   - **macOS:** `brew install azure-cli`
   - **Linux:** follow Microsoft's
     [install guide for your distribution](https://learn.microsoft.com/cli/azure/install-azure-cli-linux).
     On Debian or Ubuntu, for example:

     ```sh
     curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
     ```
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

dongle runs on macOS, Windows and Linux.

### macOS and Windows

Install dongle from your company's app portal:

- **macOS:** open the **Self Service** app, search for **dongle**, and
  click **Install**.
- **Windows:** open the **Company Portal** app, search for **dongle**, and
  click **Install**.

To update dongle later, do the same and click **Update**.

The portal sets everything up, including adding `dongle` to your `PATH`.
There's nothing to download or configure by hand.

### Linux

There's no app portal on Linux: download the dongle executable from the
dongle feed with the Azure CLI you set up in Step 1, then put it on your
`PATH`.

<!-- TODO: fill in the real Azure DevOps organization and host feed name
     (HOST_FEED in the dongle repo's azure-pipelines-release.yml), and the
     latest dongle version. -->

1. Download the build for your CPU (`dongle-linux-amd64` for x64,
   `dongle-linux-arm64` for ARM64):

   ```sh
   az artifacts universal download \
     --organization https://dev.azure.com/<TODO-ORG> \
     --feed <TODO-HOST-FEED> \
     --name dongle-linux-amd64 \
     --version <TODO-VERSION> \
     --path ./dongle-download
   ```

2. Make it executable, rename it to `dongle` and move it onto your `PATH`:

   ```sh
   chmod +x ./dongle-download/dongle-linux-amd64
   mkdir -p ~/.local/bin
   mv ./dongle-download/dongle-linux-amd64 ~/.local/bin/dongle
   rm -rf ./dongle-download
   ```

3. If `~/.local/bin` isn't on your `PATH` yet (`echo $PATH` to check), add
   it to your shell profile (`~/.bashrc`, `~/.zshrc`, ...):

   ```sh
   export PATH="$HOME/.local/bin:$PATH"
   ```

To update dongle later, repeat steps 1 and 2 with the newer version.

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
