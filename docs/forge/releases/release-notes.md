# dongle v1.0.1

**One CLI, plug in the rest.**

dongle is a single command-line tool that brings the company's CLIs
together. Install dongle once, then add the tools you need as plugins and
run them all the same way, with no more tracking down each tool's download
location and update process separately.

This is the first release of dongle, built for everyone who uses those
tools day to day.

## What you can do

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

## Getting started

1. Install the Azure CLI, add its DevOps extension
   (`az extension add --name azure-devops`), and sign in with `az login`.
   Your account needs access to the dongle feeds.
2. Install dongle from **Self Service** (macOS) or **Company Portal**
   (Windows).
3. Open a new terminal and run `dongle --version`.

The full walkthrough is in [Getting started](../getting-started/index.md).

## Want to add your own CLI?

Plugin authors can make their CLI available through dongle. See the plugin
index repo's [CONTRIBUTING.md](<TODO-INDEX-REPO-URL>/CONTRIBUTING.md).

## Good to know

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
