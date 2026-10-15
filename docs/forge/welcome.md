# Welcome to dongle

**dongle** is one command-line tool that other teams' CLIs plug into. You
install dongle once, then add the CLIs you need as plugins:

```sh
dongle install deploy
dongle deploy service-a --env prod
```

Every plugin is found, installed, upgraded and removed the same way, so you
don't have to track down each tool, its download location and its update
process separately.

## Where to go next

1. **New here?** Start with [Getting started](getting-started/index.md):
   prerequisites, installation and your first run.
2. **Using dongle day to day?** [Use dongle commands](how-to/use-commands.md)
   shows how to find, install, upgrade and remove plugins.
3. **Want to know how it works?** [Concepts](explanation/concepts.md)
   explains plugins, manifests and the index;
   [How dongle works](explanation/design.md) explains what happens behind the
   scenes.
4. **Something went wrong?** See [Troubleshooting](troubleshooting/troubleshooting.md).
5. **Need an exact flag or exit code?** See the
   [Command reference](reference/index.md).

Want to make your own CLI available through dongle? That's covered in the
plugin index repo's
[CONTRIBUTING.md](<TODO-INDEX-REPO-URL>/CONTRIBUTING.md).
