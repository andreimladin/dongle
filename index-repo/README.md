# Staged files for the plugin index repo

Everything under this directory is **not part of the dongle host module** —
it's staged content for the *separate* plugin index repo (the one holding
`plugins/*.yaml` manifests), kept here for convenience rather than in a repo
this session doesn't have access to. Copy this directory's contents to that
repo's root (it already mirrors the layout: `docs/` nests the same way).

| file | goes to (index repo) |
|---|---|
| `azure-pipelines-validate.yml` | repo root — PR + scheduled manifest validation pipeline |
| `azure-pipelines-publish-index.yml` | repo root — on merge to main, tags the next monotonic `0.0.N` version, archives `plugins/` into `index.tar.gz`, and publishes it to the shared feed as the `dongle-index` package (what dongle's CLI actually downloads — see below) |
| `CONTRIBUTING.md` | repo root — concise front door: what this repo is, how to contribute, governance summary |
| `CODEOWNERS` | repo root — per-plugin ownership |
| `PULL_REQUEST_TEMPLATE.md` | repo root — manifest PR checklist |
| `docs/publishing-plugins.md` | `docs/` — the detailed guide: manifest format field-by-field, publishing packages, validating locally, new-plugin onboarding |

This repo stays a normal git repo — PRs, review, `git tag` — but the dongle
CLI itself no longer clones it. `azure-pipelines-publish-index.yml` is what
turns a merge here into something dongle can fetch: a versioned
`index.tar.gz` Universal Package in the shared feed, TTL-cached locally and
refreshed with `dongle refresh`.

See `docs/publishing-plugins.md` for the manifest format itself, and
`tools/validate-manifest` (`../tools/validate-manifest` from here) in this
repo for the validator these pipeline files download and run.
