# Plug in your CLI

Owners of a CLI can make it available to every dongle user. Once your plugin
is in the index, anyone can run `dongle install <your-cli>` and then
`dongle <your-cli> ...`.

The authoritative checklist for contributing, covering what's accepted, the
review rules and the release request, is the index repo's
**CONTRIBUTING.md**:
<!-- TODO: index repo URL --> `<TODO-INDEX-REPO-URL>/CONTRIBUTING.md`

This page is an overview of the journey.

## What your CLI needs

In short (CONTRIBUTING.md has the full list):

- It's a **single executable**, with no other tools or runtimes required on
  the user's machine. If it needs any, contact the dongle team first.
- It runs on **macOS, Windows, or both**. Linux builds are optional, but
  Linux alone isn't accepted.
- It works as a dongle plugin. dongle runs your executable as a child
  process with:
  - **arguments**: everything the user typed after `dongle <your-cli>`;
  - **terminal**: stdin, stdout, stderr and TTY inherited, so prompts and
    colors work;
  - **environment**: the user's environment, plus `DONGLE_VERSION` (dongle's
    version), `DONGLE_PROTOCOL` (currently `v1`) and `DONGLE_PLUGIN_NAME`;
  - **exit code**: passed back to the user unchanged.

  An existing CLI usually needs no changes. It just has to make sense when
  invoked as `dongle <name> <args>`.

## The journey

1. **Build and publish** one executable per supported platform to an Azure
   Artifacts feed, as Universal Packages (see the package rules in the
   [Manifest reference](../reference/manifest-reference.md#platformspackage)).
2. **[Run your plugin locally](plug-in-your-cli/run-locally.md)** through
   dongle to check it installs and runs.
3. **[Validate your manifest](plug-in-your-cli/validate-your-manifest.md)**
   with the same tool CI uses.
4. **[Add your manifest](plug-in-your-cli/add-your-manifest.md)** to the
   index repo with a PR to `develop`.
5. Get the PR merged: **CI passes** and **2 approvals from the dongle team**.
6. **Ask the dongle team to release a new index version.** Your plugin
   reaches users only after that release.

To understand how the pieces fit together, read
[Concepts](../explanation/concepts.md) and
[How dongle works](../explanation/design.md).
