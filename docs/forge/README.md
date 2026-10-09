# dongle (1ES CLI) documentation

<!-- TODO: Forge base URL — this folder is the source of the Forge docs site:
     <TODO-FORGE-BASE-URL>. Keep page paths stable; the index repo's
     README.md and CONTRIBUTING.md link to them. -->

`dongle` is a single host CLI that other CLIs plug into. You install a plugin
once with `dongle install <name>`, then run it as `dongle <name> [args...]`.

| section | page | read it when you want to… |
|---|---|---|
| **Getting started** | [Getting started](getting-started/index.md) | prerequisites, installation and first run |
| **How to** | [Use dongle commands](how-to/use-commands.md) | find, install, upgrade and remove plugins |
| | [Plug in your CLI](how-to/plug-in-your-cli.md) | make your own CLI available through dongle |
| | ↳ [Add your manifest](how-to/plug-in-your-cli/add-your-manifest.md) | submit your plugin to the index |
| | ↳ [Validate your manifest](how-to/plug-in-your-cli/validate-your-manifest.md) | check your manifest before opening a PR |
| | ↳ [Run your plugin locally](how-to/plug-in-your-cli/run-locally.md) | test your plugin through dongle before opening a PR |
| **Explanation** | [How dongle works](explanation/design.md) | understand what happens behind the scenes |
| | [Concepts](explanation/concepts.md) | learn what a plugin, a manifest and the index are |
| **Troubleshooting** | [Troubleshooting](troubleshooting/troubleshooting.md) | get help when something goes wrong |
| **Reference** | [Command reference](reference/commands.md) | look up every command, flag and exit code |
| | [Manifest reference](reference/manifest-reference.md) | look up every manifest field (the authoritative spec) |
| | [1es-cli-validate-manifest](reference/tools/1es-cli-validate-manifest.md) | look up the validator's usage and checks |
| **Releases** | [Release notes](releases/release-notes.md) | see what changed in each release |
