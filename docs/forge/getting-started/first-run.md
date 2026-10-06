# Your first run

The first time you run any `dongle` command, it sets itself up before the
command runs. This happens once, takes a moment, and **doesn't need network
access**.

## What happens

A released dongle binary carries two things inside it:

- **a copy of the plugin index**, so dongle knows which plugins exist before
  it ever contacts the feed;
- **default plugins**, preinstalled for everyone.
  <!-- TODO: list the default plugins shipped in the current release
       (configured in the dongle repo's configs/build.yaml `embedded:`). -->

On the first run, dongle:

1. **Initializes the plugin index**: it unpacks the built-in copy into its
   data folder.
2. **Installs the default plugins**: it unpacks each one into its data
   folder, as if you had run `dongle install` for it.
3. Runs the command you typed.

## What you see

In a terminal, each step shows a spinner while it works, then a check
mark when it's done:

```
✓ Initialized plugin index 1.4.0
✓ Installed tacho 1.2.0
✓ Installed bell 0.3.1
✓ Initialization complete.
```

(Versions and plugin names depend on your release.) These progress lines go
to stderr, so they never mix with the command's own output. When output
isn't a terminal (piped, or in CI), there are no spinners or colors. Each
step is printed as a plain line instead, starting with
`Initializing dongle...`.

If a step fails, dongle prints a `warning:` line and carries on. For
example, a default plugin that fails to install doesn't block the others.
The failed plugin can be installed later with `dongle install <name>`.

## What you have afterwards

Run `dongle --version`:

```
dongle    1.0.0
index     1.4.0 (embedded)

installed plugins:
  bell    0.3.1
  tacho   1.2.0
```

`(embedded)` means you're still using the index copy that shipped inside
the binary. It goes away once dongle downloads a newer index from the feed.

Everything dongle stores lives in one data folder: `~/.dongle` on macOS
(`%USERPROFILE%\.dongle` on Windows). It holds the index cache, installed
plugins, and `state.json` (the record of what's installed).

## After the first run

Setup never runs again. From now on, `search`, `install` and `upgrade`
first ask the feed whether a newer index exists. If one does, dongle asks
before downloading it (see
[Use dongle commands](../how-to/use-commands.md#keeping-the-index-up-to-date)).
This is the first point where you need the [prerequisites](prerequisites.md)
(Azure CLI, signed in).

A dongle built without a built-in index (a developer build) skips
setup. Instead, its first `search`/`install`/`upgrade` downloads the index
from the feed (`Downloading plugin index...`).

Next: [Use dongle commands](../how-to/use-commands.md).
