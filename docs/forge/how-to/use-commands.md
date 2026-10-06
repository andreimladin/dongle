# Use dongle commands

This page covers the everyday tasks. For every flag and exit code, see the
[Command reference](../reference/commands.md).

Before you start, make sure you have the [prerequisites](../getting-started/prerequisites.md)
set up: the Azure CLI installed and signed in.

## Find a plugin

```sh
dongle search
```

This lists every plugin in the index with its latest version and a short
description:

```
deploy    2.3.1  Deploy services to the platform
```

## Install a plugin

```sh
dongle install deploy
```

dongle checks that the plugin is compatible with your dongle version,
downloads the build for your OS and CPU, and installs it. Then run it like
any other command, passing its own arguments after the name:

```sh
dongle deploy --help
dongle deploy service-a --env prod
```

Everything after the plugin name goes to the plugin unchanged.

## See what's installed

```sh
dongle --version
```

This shows your dongle version, the index version, and every installed
plugin with its version. There is no separate `list` command.

## Upgrade plugins

```sh
dongle upgrade            # every installed plugin
dongle upgrade deploy     # just one
```

A plugin is upgraded when the index has a newer version than the one you
have. dongle never downgrades. If your installed version is newer than the
index's, it's left alone. To force the index's version, run
`dongle remove <name>` and then `dongle install <name>`.

## Remove a plugin

```sh
dongle remove deploy
```

This deletes every installed version of the plugin.

## Keeping the index up to date

The **index** is the list of available plugins and their latest versions.
dongle keeps a local copy and never replaces it without telling you:

- **`search`, `install` and `upgrade`** first ask the feed whether a newer
  index exists (a quick check, nothing is downloaded). If there is one:
  - in a terminal, dongle asks:
    `A newer plugin index is available (1.4.0 → 1.5.0). Update the index first? [y/N]`
  - in a script or CI, it doesn't ask. It uses the local copy and prints a
    `note:` telling you how to update.
- **`--sync`** updates the index first without asking. **`--no-sync`**
  skips the check entirely and uses the local copy. These flags work on
  `search`, `install` and `upgrade`.
- **`dongle update`** checks and downloads a newer index right away, then
  lists the plugins in it, marking which you have installed and which have
  an upgrade available.

If the feed can't be reached, `search`, `install` and `upgrade` print a
`warning:` and carry on with the local copy. `dongle update` fails instead,
because updating is its only job.

## Get help with a plugin

```sh
dongle support deploy
```

This shows where the plugin's owners provide help: documentation, a support
channel, and optionally a contact. It works for any plugin in the index,
whether or not you've installed it. For problems with a plugin, contact its
owners there. See [Troubleshooting](../troubleshooting/troubleshooting.md).

## Get help with dongle

```sh
dongle --help             # overview and list of commands
dongle install --help     # help for one command
dongle deploy --help      # a plugin's own help
```

There is no `help` command. Use `--help` (or `-h`).

## Scripting tips

- Results go to **stdout**. Progress, warnings, errors and prompts go to
  **stderr**, so piping stdout gives you clean output.
- Colors and spinners appear only on a terminal. Set `NO_COLOR` to turn
  colors off everywhere.
- In scripts, pass `--sync` or `--no-sync` so the result doesn't depend on
  whether a newer index happens to exist.
- A plugin's exit code is passed through unchanged.
