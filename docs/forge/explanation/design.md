# How dongle works

This page explains what happens behind the scenes: how a command reaches a
plugin, where plugins and the index come from, and why it's built this way.
For the terms used here, see [Concepts](concepts.md).

## The host and its plugins

dongle is a **host**: a small CLI with a few builtin commands (`search`,
`install`, `upgrade`, `remove`, `update`, `support`, plus `--version` and
`--help`). Everything else is a **plugin**, which is a separate executable
owned by another team.

When you type `dongle <word> ...`, dongle decides what to do in order:

1. If `<word>` is a builtin, the builtin runs.
2. Otherwise, if a plugin named `<word>` is installed, dongle runs that
   plugin.
3. Otherwise, dongle prints `error: command "<word>" is not supported`, with
   suggestions and the help text, and exits with code 2.

Builtin names therefore always win over plugin names.

## Running a plugin: one process, one shot

dongle doesn't load plugins into itself, and nothing runs in the
background. Each plugin invocation is a single **child process**:

- dongle looks the plugin up in its local record of installed plugins
  (`state.json`);
- it **re-checks compatibility** (see below), because dongle itself may have
  been upgraded since the plugin was installed;
- it starts the plugin's executable with everything after the plugin name
  as arguments, unchanged;
- the plugin **inherits the terminal** (stdin, stdout, stderr), so prompts,
  colors and progress bars work as if you'd run it directly;
- the plugin gets your environment plus `DONGLE_VERSION`,
  `DONGLE_PROTOCOL` and `DONGLE_PLUGIN_NAME`;
- when it exits, dongle exits with **the plugin's exit code**.

This contract is deliberately small and language-neutral. A plugin can be
written in anything that produces an executable, and an existing CLI usually
works as a plugin without changes. dongle doesn't handle sign-in for plugins.
A plugin that needs credentials gets them the same way it would standalone.

## Compatibility checks

Each plugin's manifest can declare:

- the **dongle versions** it works with (`requires.host`, e.g. `>=0.1.0`);
- the **protocol** it speaks (`requires.protocol`). The protocol is the
  version of the host↔plugin contract above, currently `v1`. It changes
  much more rarely than dongle's own version, and only when the contract
  itself changes.

dongle checks both at install time and again before every run. An
incompatible plugin is refused with a reason (for example
`requires host >=1.2.0 but this host is 1.1.0`) instead of failing in an
unpredictable way.

So there are three independent version axes: dongle's version, each
plugin's own version, and the protocol version.

## Where plugins come from: feeds

dongle has no package server of its own. Plugin executables are stored as
**Universal Packages in Azure Artifacts feeds**, one package per plugin
version and platform. dongle doesn't talk to Azure itself. It calls the
**Azure CLI (`az`)**, which uses whatever sign-in you already have. That's
why `az` is a [prerequisite](../getting-started/index.md#step-1--prerequisites), and why
dongle never stores credentials.

Installing a plugin means:

1. read its manifest from the local index;
2. check compatibility;
3. pick the `platforms` entry matching this machine's OS and CPU;
4. download that package (`az artifacts universal download`) into a staging
   folder;
5. take the single file in it, place it under
   `~/.dongle/plugins/<name>/<version>/`, and name it consistently;
6. record it in `state.json`.

Because the file is staged first and only moved into place at the end, a
failed download never leaves a half-installed plugin.

## The index: a released archive, not a live repo

The index (the catalog of plugin manifests) is maintained in a git repo,
but dongle never reads that repo. Instead, the dongle team **releases**
the index:

- a release packs the repo's `plugins/` folder, plus a `VERSION` file, into
  `index.tar.gz`;
- that archive is published to the feed as the **`dongle-index`** package,
  at a version chosen by the dongle team (from a `release/X.Y.Z` branch).

dongle keeps an extracted copy in `~/.dongle/index/`. To stay current, it
splits the work into a cheap step and an expensive one:

- **checking**: before `search`, `install` and `upgrade`, it asks the feed
  only for the latest `dongle-index` *version*, without downloading
  anything;
- **downloading**: only if that version is newer, and only with your
  consent (a prompt, `--sync`, or `dongle update`), it downloads and
  unpacks the archive and swaps it in.

The local index is therefore never replaced behind your back, and a merged
manifest reaches users only through a deliberate index release.

## Batteries included: the embedded index and default plugins

Release builds of dongle **embed** two things at build time:

- a copy of the current `dongle-index` archive;
- a set of **default plugins** chosen by the dongle team.

On the [first run](../getting-started/index.md#step-3--first-run), dongle unpacks both,
so a new install works immediately and **offline**, and `--version` labels
that index `(embedded)`. After that, the normal checking/downloading cycle
takes over. Developer builds embed nothing and download the index on first
use instead.

## Local data

Everything lives under one data folder, `~/.dongle` (or `DONGLE_DATA_DIR` if
set):

```
~/.dongle/
  index/plugins/*.yaml        # extracted index
  index.version               # its version
  plugins/<name>/<version>/   # installed plugin executables
  state.json                  # what's installed, active versions, requires
```

`dongle remove` deletes a plugin's folder and its `state.json` entry.

## Design choices, in short

| choice | why |
|---|---|
| plugins are separate executables run per invocation | any language; no daemon; a plugin crash can't take dongle down |
| terminal and exit code passed straight through | plugins behave exactly as if run directly |
| `az` for all feed access | reuses existing sign-in and permissions; dongle stores no secrets |
| index released as a versioned package | users get reviewed, deliberate catalog updates |
| check the version first, download only on consent | fast commands, no surprise changes |
| embedded index and defaults | a fresh install works offline from the first command |
