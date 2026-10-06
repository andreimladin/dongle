# Command reference

Every dongle command, flag, environment variable and exit code. For
task-oriented guidance, see [Use dongle commands](../how-to/use-commands.md).

## Synopsis

```
dongle <command> [flags]
dongle <plugin> [args...]
dongle --version
dongle [command] --help
```

| command | arguments | flags | purpose |
|---|---|---|---|
| [`search`](#dongle-search) | none | `--sync`, `--no-sync` | list plugins available in the index |
| [`install`](#dongle-install) | `<name>` (exactly 1) | `--sync`, `--no-sync` | install a plugin from the index |
| [`upgrade`](#dongle-upgrade) | `[name]` (0 or 1) | `--sync`, `--no-sync` | upgrade one or all installed plugins |
| [`remove`](#dongle-remove) | `<name>` (exactly 1) | — | remove an installed plugin |
| [`update`](#dongle-update) | none | — | refresh the plugin index from the feed |
| [`support`](#dongle-support) | `<plugin-name>` (exactly 1) | — | show where to get help with a plugin |
| [`<plugin>`](#dongle-plugin) | anything | passed to the plugin | run an installed plugin |

Every command also accepts `-h`/`--help`. There is **no** `help` command
and no shell-completion command. `dongle help` is treated like any other
unknown name.

## Global flags

| flag | effect |
|---|---|
| `-h`, `--help` | print help for dongle, or for the command it follows (`dongle install --help`). After a plugin name (`dongle deploy --help`), it goes to the plugin instead |
| `--version` | print dongle's version, the index version, and installed plugins (see [`--version`](#dongle---version)). Only valid as the first argument |

## Index sync flags

`search`, `install` and `upgrade` first make sure an index is cached, then
check the feed for a newer index version (a metadata query, nothing is
downloaded). What happens when a newer one exists depends on these flags:

| flag | behavior |
|---|---|
| *(neither)* | on a terminal: ask `Update the index first? [y/N]` (default **no**). Not on a terminal: keep the cached index and print a `note:` suggesting `dongle update` or `--sync` |
| `--sync` | download and apply the newer index without asking |
| `--no-sync` | skip the check; use the cached index (`Using cached plugin index X (--no-sync).`) |

`--sync` and `--no-sync` are mutually exclusive. Passing both is a usage
error (exit 2).

If the feed can't be reached, or the download fails, a `warning:` is
printed and the cached index is used. The command doesn't fail because of
it. If nothing is cached and the binary has no embedded index, the index
is downloaded first (`Downloading plugin index...`), and failing that **is**
an error.

## `dongle search`

```
dongle search [--sync | --no-sync]
```

Lists every plugin in the index, sorted by name, one per line on stdout:
name, version (without a leading `v`), short description. Prints
`The plugin index is empty.` when there are none.

## `dongle install`

```
dongle install <name> [--sync | --no-sync]
```

Installs plugin `<name>` from the index:

1. [index sync](#index-sync-flags); if the index was updated, its summary is
   printed to stderr;
2. look up `plugins/<name>.yaml` in the index (`no plugin named <name> in
   the index` if it's missing);
3. compatibility check against `requires.host` and `requires.protocol`
   (`<name> requires host … but this host is …` / `<name> speaks protocol
   … but this host supports "v1"`, then `— not installing`);
4. select the `platforms` entry for this machine's OS/arch (`<name>
   <version> has no build for <os>/<arch>`);
5. download that package from the manifest's feed at the manifest's
   version; it must contain exactly one file;
6. place it at `<data dir>/plugins/<name>/<version>/` and record it in
   `state.json`.

On success it prints `✓ Installed <name> <version> (run it with: dongle
<name>)`. Installing an already-installed plugin installs the index's
version again.

## `dongle upgrade`

```
dongle upgrade [name] [--sync | --no-sync]
```

Upgrades installed plugins to the version the index declares.

- **With `name`**: the plugin must be installed (`<name> is not installed;
  use dongle install <name>`). An error if it's not in the index, or if the
  installed version is newer than the index's (never downgrades).
- **Without `name`**: every installed plugin, in name order. Plugins that
  are missing from the index, or newer than it, are **skipped** with a
  warning instead of failing the run. Ends with a summary: `N upgraded, N
  already up to date, N skipped, N failed`. Exit code 1 if any failed.
- With no plugins installed, it prints `No plugins installed; nothing to
  upgrade.` and exits 0, without checking the feed.

A plugin that's already up to date prints `<name> is already up to date
(<version>)`. Upgrading runs the same compatibility check, platform
selection and download as `install`.

## `dongle remove`

```
dongle remove <name>
```

Deletes every installed version of `<name>` from disk and removes it from
`state.json`. Error `<name> is not installed` if it isn't installed.
Doesn't contact the feed.

## `dongle update`

```
dongle update
```

Refreshes the cached index, ignoring how recently it was checked:

1. asks the feed for the latest index version;
2. if it's not newer than the cached one, prints `Plugin index X is already
   up to date.`; otherwise downloads and applies it without prompting;
3. prints the index version and every plugin in it, marking each as
   `installed`, or `installed <v>, upgrade available`.

If the feed can't be reached, or the download fails, it prints `could not
update the plugin index: …` and **exits 1**. The existing cache stays in use
(or, with nothing cached, the embedded index is unpacked).

## `dongle support`

```
dongle support <plugin-name>
```

Prints the plugin's `support` block from its manifest: Documentation,
Channel, and Contact if set. Works for any plugin in the cached index,
installed or not. **Doesn't contact the feed.** It reads the cached index as
it is (downloading one only if nothing is cached at all).

## `dongle --version`

```
dongle --version
```

Prints, on stdout:

```
dongle    <host version>
index     <index version> [(embedded)]

installed plugins:
  <name>  <version>
```

- `index none (run dongle update)` when no index is cached.
- `(embedded)` when the cached index is still the copy built into the
  binary.
- `no plugins installed` when there are none.

This is also how you list installed plugins. Doesn't contact the feed.

## `dongle <plugin>`

```
dongle <plugin> [args...]
```

Any first argument that isn't a builtin is treated as a plugin name:

- **installed**: dongle re-checks compatibility, then runs the plugin with
  `args` passed through unchanged (no flag parsing by dongle), with the
  terminal inherited. dongle exits with **the plugin's exit code**;
- **not installed**: `error: command "<plugin>" is not supported`, plus
  `Did you mean: …?` suggestions where applicable, and the root help on
  stderr. Exit code 2;
- **starts with `-`** (an unknown flag): `error: unknown flag "<flag>"`
  plus the root help. Exit code 2.

Builtin names always take precedence over plugin names.

**Environment passed to plugins**: the caller's full environment, plus:

| variable | value |
|---|---|
| `DONGLE_VERSION` | dongle's own version |
| `DONGLE_PROTOCOL` | host↔plugin protocol, currently `v1` |
| `DONGLE_PLUGIN_NAME` | the plugin name as invoked |

## Exit codes

| code | meaning |
|---|---|
| `0` | success |
| `1` | the command failed (error printed on stderr) |
| `2` | usage error: unknown command/flag, wrong number of arguments, conflicting flags |
| *any* | when running a plugin: the plugin's own exit code |

## Output conventions

- **stdout**: command results only (`search` list, `--version` report,
  `support` block, `install`/`remove` result lines, `update` summary).
- **stderr**: progress spinners, status lines (`✓`, `•`), `note:`,
  `warning:`, `error:`, and prompts.
- Colors, symbols and spinners appear only when the stream is a terminal.
  Otherwise output is plain text, one line per step, and dongle never
  prompts.

## Environment variables

| variable | effect |
|---|---|
| `DONGLE_DATA_DIR` | data folder (default `~/.dongle`). Holds the index cache, installed plugins and `state.json` |
| `NO_COLOR` | if set (to any value), disables colors |
| `TERM=dumb` | disables colors |
| `AZURE_DEVOPS_EXT_PAT` | read by the Azure CLI (not by dongle) to authenticate without `az login`, e.g. in CI |
| `DONGLE_INDEX_ORG`, `DONGLE_INDEX_PROJECT`, `DONGLE_INDEX_FEED`, `DONGLE_INDEX_PACKAGE` | developer overrides for where the index package is read from (organization, project for project-scoped feeds, feed, package name; default package `dongle-index`). Not needed with release builds |

## Data folder layout

```
<data dir>/
  index/plugins/<name>.yaml     # cached index manifests
  index.version                 # cached index version
  index.origin                  # "embedded" or "fetched"
  index.meta                    # when the index was last confirmed current
  plugins/<name>/<version>/     # installed plugin executable
  state.json                    # installed plugins, active versions, requires
```
