# Installation

**Step 2 of 3** in [Getting started](index.md). Before you begin, complete
[Step 1 — Prerequisites](prerequisites.md): the Azure CLI must be installed
and signed in, because you download dongle with it.

dongle is a single, self-contained executable — there's no installer. Each
release is published to the dongle host feed as one Universal Package per
platform:

| platform | package | file inside |
|---|---|---|
| macOS, Apple silicon | `dongle-darwin-arm64` | `dongle-darwin-arm64` |
| macOS, Intel | `dongle-darwin-amd64` | `dongle-darwin-amd64` |
| Windows, x64 | `dongle-windows-amd64` | `dongle-windows-amd64.exe` |
| Windows, ARM64 | `dongle-windows-arm64` | `dongle-windows-arm64.exe` |

(Linux packages `dongle-linux-amd64` / `dongle-linux-arm64` are also
published.)

<!-- TODO: fill in the host feed coordinates (organization, project if the
     feed is project-scoped, feed name) and the current release version, or
     confirm "*" (latest) is the recommended way to install. -->

## macOS

```sh
# 1. Download (use dongle-darwin-amd64 on an Intel Mac)
az artifacts universal download \
  --organization https://dev.azure.com/<TODO-org> \
  --feed <TODO-host-feed> \
  --name dongle-darwin-arm64 \
  --version "*" \
  --path ./dongle-download

# 2. Rename and make it executable
mv ./dongle-download/dongle-darwin-arm64 ./dongle
chmod +x ./dongle

# 3. Put it on your PATH
sudo mv ./dongle /usr/local/bin/dongle
rm -rf ./dongle-download
```

`--version "*"` downloads the latest release; pass an exact version (for
example `--version 1.0.0`) to pin one. If the feed is project-scoped, add
`--project <TODO-project> --scope project`.

## Windows (PowerShell)

```powershell
# 1. Download (use dongle-windows-arm64 on an ARM device)
az artifacts universal download `
  --organization https://dev.azure.com/<TODO-org> `
  --feed <TODO-host-feed> `
  --name dongle-windows-amd64 `
  --version "*" `
  --path .\dongle-download

# 2. Rename it into a tools folder
$dir = "$env:USERPROFILE\bin"
New-Item -ItemType Directory -Force $dir | Out-Null
Move-Item .\dongle-download\dongle-windows-amd64.exe "$dir\dongle.exe" -Force
Remove-Item -Recurse .\dongle-download

# 3. Add the folder to your PATH (current user; once)
[Environment]::SetEnvironmentVariable("Path", "$([Environment]::GetEnvironmentVariable('Path','User'));$dir", "User")
```

Open a **new** terminal so the updated `PATH` takes effect.

## Why rename to `dongle`

dongle names installed plugins after its own file name and shows that name
in hints such as `run it with: dongle deploy`. Keeping the file named
`dongle` (`dongle.exe`) keeps both consistent.

## Check the installation

From a new terminal:

```sh
dongle --version
```

If you see `command not found` (macOS) or `not recognized` (Windows), the
folder isn't on your `PATH` — revisit step 3 above.

Running this command also triggers dongle's one-time setup — that's the
next step.

## Upgrading dongle

Repeat the download with the newer version and replace the existing file.
Your installed plugins and index live in `~/.dongle`
(`%USERPROFILE%\.dongle`) and are kept.

**Next: [Step 3 — First run](first-run.md).**
