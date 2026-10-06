# Run your plugin locally

This page shows how to test your plugin through dongle on your own machine
**before** opening the index PR, using your real manifest and your published
packages. Your normal dongle setup stays untouched.

You'll create a throwaway dongle data folder that holds a "local index"
with just your manifest. dongle then installs from that local index instead
of the real one.

**Before you start**

- A **released** dongle binary. A developer build (`dongle --version` shows
  `dev`) rejects any manifest that sets `requires.host`.
- The Azure CLI, signed in (see [Prerequisites](../../getting-started/index.md#step-1--prerequisites)).
- Your plugin's packages published to the feed named in your manifest.

## Quick check: run the executable directly

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

## Full check: install and run through dongle

### 1. Create a sandbox data folder with a local index

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

### 2. Install from the local index

Always pass `--no-sync`. Without it, dongle checks the feed, sees that the
real index is newer than `0.0.0-local`, and offers to replace your local
index:

```sh
dongle search --no-sync               # your plugin should be listed
dongle install <name> --no-sync       # downloads YOUR package for this OS/arch from the feed
```

This exercises the same steps a user's install does: the compatibility
check (`requires`), picking the `platforms` entry for your OS/arch, and
downloading and installing the package.

### 3. Run it

```sh
dongle <name> --help
dongle <name> <your usual arguments>
dongle support <name>                 # shows your support block
dongle --version                      # lists <name> with its version
```

Check that prompts, colors and exit codes behave as expected.

### 4. Iterate and clean up

After changing the manifest, copy it into the sandbox again, then run
`dongle remove <name>` followed by `dongle install <name> --no-sync`.

When you're done, delete the sandbox folder and close the terminal (or
unset `DONGLE_DATA_DIR`).

## Common failures

| message | meaning |
|---|---|
| `bad constraint in manifest: invalid semver "dev"` | you're using a developer build of dongle; use a released one |
| `<name> requires host >=X but this host is Y` | your `requires.host` is newer than your dongle |
| `<name> speaks protocol "..." but this host supports "v1"` | `requires.protocol` must be `"v1"` |
| `<name> <version> has no build for <os>/<arch>` | no `platforms` entry matches this machine |
| `az download <package>@<version>: ...` | the package isn't in the feed under that name/version, or you can't read the feed |
| `package ... contains N files, expected exactly 1` | the package must contain only the executable |

All manifest rules are in the [Manifest reference](../../reference/manifest-reference.md).
