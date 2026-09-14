## What changed

<!-- Which plugin(s)/manifest(s) does this PR touch? New plugin, version
bump, platform addition, or a feed coordinate change? -->

## Checklist

- [ ] `validate-manifest` run locally against the changed manifest(s) and passed (see `docs/publishing-plugins.md` — "Validating locally")
- [ ] Package(s) for every platform listed in the manifest published to the feed named in `feed:`, at the version in `version:`
- [ ] `version:` bumped in `plugins/<name>.yaml` and matches what was actually published
- [ ] `name:` matches the filename (`plugins/<name>.yaml` → `name: <name>`)

### New plugin only

_Skip this section for a version bump to an existing plugin._

- [ ] Added a `plugins/<name>.yaml -> owning team` line to `CODEOWNERS`
- [ ] Central/platform-team review requested — new plugins are not owner-approved alone (see governance in `docs/publishing-plugins.md`)
- [ ] Confirmed the plugin name doesn't collide with an existing `dongle <name>` command

## Notes for reviewers

<!-- Anything reviewers should know: platforms intentionally not built yet,
known gaps, links to the packages you published, etc. -->
