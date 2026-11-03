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
3. pick the `platforms` entry matching this machine's OS and CPU,
   automatically. More precisely, dongle matches the OS and architecture
   its own executable was built for (for example `darwin/arm64` or
   `linux/amd64`), which is the same as your machine's as long as you
   installed the dongle build for your machine;
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
the index as a versioned package, **`dongle-index`**, in the feed. (How a
release is built is described in the index repo's
[README.md](<TODO-INDEX-REPO-URL>/README.md#how-the-index-is-released).)

dongle keeps an extracted copy in `~/.dongle/index/`. To stay current, it
splits the work into a cheap step and an expensive one:

- **checking**: before `search`, `install` and `upgrade`, it asks the feed
  only for the latest `dongle-index` *version*, without downloading
  anything — and only when the local copy was last checked more than an
  hour ago (`dongle update` and `--sync` always check);
- **downloading**: only if that version is newer, and only with your
  consent (a prompt, `--sync`, or `dongle update`), it downloads and
  unpacks the archive and swaps it in.

The local index is therefore never replaced behind your back, and a merged
manifest reaches users only through a deliberate index release.

## First run: downloading the index and default plugins

The dongle executable contains **only dongle itself** — no plugins and no
plugin index are built into it. On the
[first run](../getting-started/index.md#step-3--first-run), dongle sets
itself up from the feed:

1. it downloads the latest `dongle-index` archive;
2. it reads the archive's **`defaults.yaml`**, the list of default plugins
   the dongle team maintains in the index repo (it is the single source of
   truth for which plugins every new user gets);
3. it installs each of those plugins exactly as `dongle install` would.

It does this once. After that, the normal checking/downloading cycle
described above takes over.

**The first run needs the feed**: network access and an `az login` that
can read it. If the feed can't be reached, dongle stops with an error,
changes nothing, and tries again from the start the next time you run it.
This happens on the first run of any command, `dongle --help` included.

## Local data

Everything lives under one data folder, `~/.dongle` (it can't be
relocated):

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
| nothing built into the executable; index and defaults downloaded on first run | small executable; the default plugins are decided by the index (`defaults.yaml`), not by a dongle release |
