# Staged files for the plugin index repo

Everything under this directory is **not part of the dongle host module** —
it's staged content for the *separate* plugin index repo (the one holding
`plugins/*.yaml` manifests), kept here for convenience rather than in a repo
this session doesn't have access to. Copy this directory's contents to that
repo's root (it already mirrors the layout: `docs/` nests the same way).

| file | goes to (index repo) |
|---|---|
| `azure-pipelines-validate.yml` | repo root — PR + scheduled manifest validation pipeline |
| `azure-pipelines-publish-index.yml` | repo root — manual-only (mirrors the dongle host repo's own release pipelines): run against a `release/X.Y.Z` branch, reads that version straight from the branch name, archives `plugins/` into `index.tar.gz`, and publishes it to the shared feed as the `dongle-index` package (what dongle's CLI actually downloads — see below). Never writes to git. All of the logic lives in this one file — there is no separate script. |
| `CONTRIBUTING.md` | repo root — concise front door: what this repo is, how to contribute, governance summary |
| `CODEOWNERS` | repo root — per-plugin ownership |
| `PULL_REQUEST_TEMPLATE.md` | repo root — manifest PR checklist |
| `docs/publishing-plugins.md` | `docs/` — the detailed guide: manifest format field-by-field, publishing packages, validating locally, new-plugin onboarding |

This repo stays a normal git repo — PRs, review — but the dongle CLI itself
no longer clones it. Publishing a new index version means queuing
`azure-pipelines-publish-index.yml` by hand against a `release/X.Y.Z`
branch: whichever version the branch name names is what gets published,
verbatim — no git tags, no auto-increment, no git writes at all. That's
what turns this repo's `plugins/` into something dongle can fetch: a
versioned `index.tar.gz` Universal Package in the shared feed, TTL-cached
locally and refreshed with `dongle sync`.

See `docs/publishing-plugins.md` for the manifest format itself, and
`tools/validate-manifest` (`../tools/validate-manifest` from here) in this
repo for the validator these pipeline files download and run.
