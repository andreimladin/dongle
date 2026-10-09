# Concepts

The three ideas you need to understand dongle. For how they work together
at run time, see [How dongle works](design.md).

## What is a plugin

A **plugin** is a CLI that runs *through* dongle. It's an ordinary
executable built and owned by another team. Once installed, it appears as a
dongle command:

```sh
dongle install deploy
dongle deploy service-a --env prod
```

From the plugin's point of view, nothing special happens: it's started
with its arguments, gets the user's terminal, and its exit code is passed
back. dongle adds a few environment variables (`DONGLE_VERSION`,
`DONGLE_PROTOCOL`, `DONGLE_PLUGIN_NAME`) in case the plugin wants to know it
was started by dongle.

Each plugin has:

- a **name**, which is the command users type;
- its own **version**, released on its own schedule;
- **one build per platform** it supports (for example macOS on Apple
  silicon, Windows x64), each stored as a package in a feed;
- a **support** entry pointing users to its owners.

dongle's own commands are *builtins*, not plugins.

## What is a manifest

A **manifest** is a plugin's entry in the catalog: one small YAML file per
plugin, written by the plugin's owners. It answers the questions dongle
needs answered before it can install the plugin:

- *What's it called, and which version is current?*
- *Which dongle versions can run it?*
- *Where are its builds stored, and which build is for which platform?*
- *Where do users go for help?*

The manifest is the only thing a plugin owner submits to dongle. The
plugin's code and executables stay with its owners and their feed.

Manifests are reviewed and validated before they're accepted, so every
entry users see has been checked.

The exact fields and rules are in the
[Manifest reference](../reference/manifest-reference.md). That page is the
specification.

## What is the index

The **index** is the catalog of all plugins: the collection of every
manifest, one file per plugin.

- It's **maintained** in a git repo (the index repo), where plugin owners
  add or update their manifest through pull requests.
- It's **released** by the dongle team as a versioned package
  (`dongle-index`). A merged manifest reaches users only after the next
  index release.
- It's **cached** on each user's machine. dongle checks for a newer index
  version and downloads it only when you agree, or when you run
  `dongle update`. Release builds ship with a copy built in, so dongle works
  from its first run.

`dongle search` shows what's in the index you have, and `dongle --version`
shows its version.
