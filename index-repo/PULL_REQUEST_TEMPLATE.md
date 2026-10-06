## What changed

<!-- Which plugin(s)/manifest(s) does this PR touch? New plugin, version
bump, platform addition, or a feed coordinate change? -->

## Checklist

- [ ] PR targets `develop`
- [ ] `name:` matches the filename (`plugins/<name>.yaml` → `name: <name>`)
- [ ] `version:` bumped and matches what was actually published
- [ ] Package(s) for every platform listed in the manifest published to the feed named in `feed:`, at `version:` (without the leading `v`), one executable per package
- [ ] The plugin is a single executable with no extra tools/runtimes needed (or the dongle team has been contacted)
- [ ] The executable was tested on every OS/arch listed under `platforms:` — CI does not run or test the plugin itself

### New plugin only

_Skip this section for an update to an existing plugin._

- [ ] The plugin name doesn't collide with an existing plugin or a `dongle` builtin command

## Before this reaches users

Merging requires CI to pass and 2 approvals from the dongle team. After
merge, ask the dongle team to release a new index version (see
`CONTRIBUTING.md`).

## Notes for reviewers

<!-- Anything reviewers should know: platforms intentionally not built yet,
known gaps, the packages you published, etc. -->
