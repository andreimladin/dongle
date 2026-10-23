# dongle release notes

## dongle v1.0.1

**Fewer index checks, faster commands.**

dongle now remembers when it last checked the plugin index and skips the
check for an hour afterwards.

### What changed

- **The plugin index check is cached for 1 hour.** Before `dongle search`,
  `dongle install` and `dongle upgrade` run, dongle asks the feed whether a
  newer plugin index exists. It now does that at most once an hour: after
  the index is downloaded, or confirmed to be the latest, the commands that
  follow within the hour use your local index straight away, with no feed
  call and no prompt.
- **`dongle search` never prompts.** When a newer index is available,
  `search` uses the one you have and prints a note suggesting
  `dongle update` or `--sync`. Only `install` and `upgrade` ask whether to
  update the index first.

### What it means for you

- **Faster, quieter commands.** Running several `search`, `install` or
  `upgrade` commands in a row no longer contacts the feed every time, and
  you won't see `Checking for a newer plugin index...` on each one.
- **A new index can take up to an hour to show up.** If an index was
  released since dongle last checked, you may not see its plugins or
  versions until the hour is up. To pick it up right away, run
  `dongle update`, or add `--sync` to `search`, `install` or `upgrade`.
  Both always check the feed, whatever the time since the last check.
- **Saying no isn't remembered.** If you decline the `Update the index
  first?` prompt, the hour doesn't start: the next `install` or `upgrade`
  checks and asks again. The same goes for a check that can't reach the
  feed (for example when offline), so you'll see the warning on each command
  until the check succeeds.
- **`--no-sync` is unchanged.** It still skips the check entirely and uses
  your local index.
- **First run after installing.** The index built into dongle has never
  been checked, so the first `search`, `install` or `upgrade` checks the
  feed.

Details are in the [command reference](../reference/index.md#index-sync-flags).

## dongle v1.0.0

**One CLI, plug in the rest.**

dongle is a single command-line tool that brings the company's CLIs
together. Install dongle once, then add the tools you need as plugins and
run them all the same way, with no more tracking down each tool's download
location and update process separately.

This is the first release of dongle, built for everyone who uses those
tools day to day.

### What you can do

- **Install, remove and upgrade plugins** from one central plugin index:
  `dongle install <name>`, `dongle remove <name>`, and `dongle upgrade`
  (everything installed) or `dongle upgrade <name>` (just one).
- **Run any installed plugin as a dongle command.** `dongle deploy --help`
  runs the `deploy` plugin; everything after the plugin name is passed to it
  unchanged.
- **Browse what's available** with `dongle search`, which lists every plugin
  in the index with its latest version and a short description.
- **Stay current** with `dongle update`, which checks for a newer plugin
  index and downloads it only if there is one.
- **See exactly what you have** with `dongle --version`: the dongle version,
  the plugin index version, and every installed plugin with its version.
- **Works out of the box.** dongle ships with its plugin index and default
  plugins (`tacho` and `bell`) built in. The first time you run it, it sets
  itself up in a few seconds, even offline.
- **Get help fast.** `dongle support <plugin>` shows a plugin's
  documentation, support channel and contact, whether or not you've
  installed it. `dongle --help` shows usage for dongle itself.

```sh
dongle search            # what can I install?
dongle install deploy    # install a plugin
dongle deploy --help     # run it
dongle upgrade           # upgrade everything installed
dongle --version         # dongle, index and plugin versions
```

### Getting started

1. Install the Azure CLI, add its DevOps extension
   (`az extension add --name azure-devops`), and sign in with `az login`.
   Your account needs access to the dongle feeds.
2. Install dongle from **Self Service** (macOS) or **Company Portal**
   (Windows).
3. Open a new terminal and run `dongle --version`.

The full walkthrough is in [Getting started](../getting-started/index.md).

### Want to add your own CLI?

Plugin authors can make their CLI available through dongle. See the plugin
index repo's [CONTRIBUTING.md](<TODO-INDEX-REPO-URL>/CONTRIBUTING.md).

### Good to know

- **The Azure CLI is required** for anything that downloads from the feed:
  `dongle install`, `dongle upgrade` and `dongle update`. Running installed
  plugins, `dongle search`, `dongle support` and `dongle --version` work
  from what's already on your machine.
- **Supported platforms:** macOS and Windows, installed through your
  company's app portal.
- **Plugin availability varies by platform.** Each plugin decides which
  platforms it supports; if one has no build for your machine, dongle tells
  you and `dongle support <plugin>` shows who to contact.
- **Updating dongle itself** is done through Self Service or Company
  Portal. `dongle upgrade` upgrades plugins, not dongle.
