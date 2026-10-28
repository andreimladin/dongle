# Troubleshooting

## A problem with a plugin

Each plugin is built and supported by **its own team**, not by the dongle
team. If a plugin misbehaves (wrong results, crashes, questions about its
options), contact its owners:

```sh
dongle support <plugin-name>
```

```
deploy — support
  Documentation:    https://docs.acme.internal/dongle-deploy
  Channel:          https://acme.slack.com/archives/C0DEPLOY
  Contact:          deploy-team@acme.example.com
```

This works for any plugin in the index, installed or not. If the plugin
isn't listed, run `dongle update` first.

## A problem with dongle itself

Check that the [prerequisites](../getting-started/index.md#step-1--prerequisites) are in
place, then look for your message below.

| message | what to do |
|---|---|
| `dongle: command not found` / `'dongle' is not recognized` | open a new terminal. If it still fails, check that dongle is installed in Self Service (macOS) or Company Portal (Windows), and reinstall it if needed. On Linux, check that the `dongle` executable is on your `PATH` and executable (`chmod +x`); see [Installation](../getting-started/index.md#linux) |
| `the Azure CLI is required: install it and run az extension add --name azure-devops` | install the Azure CLI and the extension ([Prerequisites](../getting-started/index.md#step-1--prerequisites)) |
| an `az ...` error mentioning 401, 403, login or authorization | run `az login` with an account that can read the feed |
| `warning: could not check the feed for a newer plugin index` | the feed couldn't be reached; dongle continued with your local index. Check your network and `az login` |
| `command "X" is not supported` | `X` isn't a builtin or an installed plugin. Check `dongle search`, then `dongle install X` |
| `no plugin named X in the index` | check the spelling. If the plugin was added recently, run `dongle update`; if it's still missing, it hasn't been released in the index yet |
| `X requires host >=… but this host is …` | the plugin needs a newer dongle. Update dongle from Self Service (macOS) or Company Portal (Windows), or on Linux download the newer executable ([Installation](../getting-started/index.md#linux)) |
| `X … has no build for <os>/<arch>` | every plugin must ship for macOS, Windows and Linux on both arm64 and amd64. If `<os>/<arch>` is one of those, the plugin's index entry is incomplete: contact its owners (`dongle support X`). If it's anything else (for example `linux/386`), your dongle build isn't a supported platform: on Linux, reinstall the build matching `uname -m` ([Installation](../getting-started/index.md#linux)) |
| `installed X … is newer than the index …; not downgrading` | expected. To force the index's version: `dongle remove X`, then `dongle install X` |
| a script hangs or doesn't update the index | in scripts, pass `--sync` or `--no-sync` to `search`/`install`/`upgrade` |

For what each command does, see the [Command reference](../reference/index.md).

## Contact the 1ES CLI team

For problems with dongle itself (not a plugin), or for questions about
onboarding a plugin:

<!-- TODO: fill in the 1ES CLI team's contact details. -->
- **Support channel:** `<TODO>`
- **Email / distribution list:** `<TODO>`
- **Report a bug:** `<TODO>`

When reporting a problem, include the output of `dongle --version`, the
exact command you ran, and the full error output.
