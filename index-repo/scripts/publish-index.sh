#!/usr/bin/env bash
# Manual, local equivalent of ../azure-pipelines-publish-index.yml — what a
# maintainer runs by hand today (the pipeline isn't wired up / triggerable
# yet) to do exactly what that pipeline will do once it is: archive
# plugins/ (+ a VERSION file) into index.tar.gz and publish it to the
# shared feed as the dongle-index Universal Package.
#
# Mirrors the dongle host repo's own release pattern (see its
# scripts/build.sh + azure-pipelines-release.yml / azure-pipelines-
# tooling.yml): run this against a release/X.Y.Z branch, and the version
# published is read straight from the branch name — NOT auto-incremented,
# NOT a git tag. This script never writes to git at all (no tag, no
# push, no commit) — it only reads the current branch name and the
# working tree's contents.
#
# This script IS the shared implementation, not a parallel one:
# azure-pipelines-publish-index.yml calls this same script for all of the
# branch-validation/version-extraction/archive/publish logic below (with
# ORGANIZATION/PROJECT/FEED/PACKAGE_NAME set from its own variables block,
# and AZURE_DEVOPS_EXT_PAT already in the environment instead of an
# interactive `az login`) — so a manual run and a pipeline run always
# produce byte-identical archives under the exact same version-from-
# branch-name logic. Do not duplicate or reimplement this logic elsewhere;
# if the pipeline's needs and this script's ever diverge, change it here
# and let the pipeline pick it up, not the other way around.
#
# Auth: uses whatever the caller already has.
#   - Interactively (a maintainer's machine): run `az login` first — this
#     script does not log you in itself.
#   - From the pipeline: AZURE_DEVOPS_EXT_PAT (System.AccessToken) is
#     already set in the environment, and `az artifacts` picks it up on
#     its own, no `az login` needed — see the "Two different auth paths"
#     note in azure-pipelines-validate.yml for the same pattern used
#     there. No git auth is needed at all: this script never pushes.
#
# Requires: az (with the azure-devops extension: `az extension add --name
# azure-devops`), git, tar.
#
# Usage (from a release/X.Y.Z branch, e.g. release/0.0.5):
#   ./scripts/publish-index.sh
set -euo pipefail
cd "$(dirname "$0")/.."

# TODO: set these to your real Azure DevOps org/project + shared feed
# before running this for real (override via env instead of editing the
# script, e.g. `FEED=my-feed ./scripts/publish-index.sh`) — MUST match
# azure-pipelines-publish-index.yml's own `variables:` block, since that
# pipeline sources these same names from its variables when it calls this
# script.
ORGANIZATION="${ORGANIZATION:-TODO-azure-devops-org}" # e.g. "acme" (used as https://dev.azure.com/<ORGANIZATION>)
PROJECT="${PROJECT:-TODO-azure-devops-project}"       # leave empty ("") for an org-scoped feed
FEED="${FEED:-TODO-shared-feed-name}"                 # the SAME shared feed plugin binaries publish to
PACKAGE_NAME="${PACKAGE_NAME:-dongle-index}"

for bin in az git tar; do
	if ! command -v "$bin" >/dev/null 2>&1; then
		echo "error: $bin is required (see script header)" >&2
		exit 1
	fi
done

# current_branch prefers Azure Pipelines' own full source-ref variable
# (Build.SourceBranch, exposed to script steps as BUILD_SOURCEBRANCH) over
# git, for two reasons: a pipeline checkout typically lands in detached
# HEAD (where `git rev-parse --abbrev-ref HEAD` would report "HEAD", not
# the real branch), and Build.SourceBranchName — the OTHER predefined
# variable, easy to reach for instead — truncates any branch name
# containing a slash to just its last segment (release/0.0.5 -> "0.0.5"),
# which would silently defeat the release/X.Y.Z pattern check below. This
# mirrors azure-pipelines-release.yml's own Guard job, which uses
# Build.SourceBranch for exactly the same reason.
current_branch() {
	if [ -n "${BUILD_SOURCEBRANCH:-}" ]; then
		echo "${BUILD_SOURCEBRANCH#refs/heads/}"
	else
		git rev-parse --abbrev-ref HEAD
	fi
}

# determine_version fails fast unless the current branch matches
# release/X.Y.Z, then returns the X.Y.Z suffix — this IS the version
# that gets published, verbatim: no auto-increment, no reading git tags.
determine_version() {
	local branch
	branch=$(current_branch)
	if [[ ! "$branch" =~ ^release/[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
		echo "error: must be run on a release/X.Y.Z branch (got '$branch')" >&2
		exit 1
	fi
	echo "${branch#release/}"
}

# guard_clean refuses to publish unless the working tree is clean, so the
# published archive always matches a clean, committed state — never a
# half-finished local edit.
guard_clean() {
	if [ -n "$(git status --porcelain)" ]; then
		echo "error: working tree is dirty — commit or stash changes before publishing" >&2
		exit 1
	fi
}

# archive_plugins writes index.tar.gz into out_dir, containing plugins/
# plus a top-level VERSION file — the exact layout internal/index (in the
# dongle host repo) expects after extraction.
archive_plugins() {
	local version="$1" out_dir="$2"
	if [ ! -d plugins ]; then
		echo "error: no plugins/ directory to archive" >&2
		exit 1
	fi

	echo -n "$version" >VERSION
	tar -czf "$out_dir/index.tar.gz" VERSION plugins
	rm -f VERSION
}

publish_archive() {
	local version="$1" dir="$2"

	if ! az extension show --name azure-devops >/dev/null 2>&1; then
		az extension add --name azure-devops --only-show-errors
	fi

	local args=(artifacts universal publish
		--organization "https://dev.azure.com/${ORGANIZATION}"
		--feed "$FEED"
		--name "$PACKAGE_NAME"
		--version "$version"
		--path "$dir"
		--description "dongle plugin index $version")
	if [ -n "$PROJECT" ]; then
		args+=(--project "$PROJECT" --scope project)
	fi

	az "${args[@]}"
}

main() {
	local version
	version=$(determine_version)
	echo "publishing version: ${version}"

	guard_clean

	local stage
	stage=$(mktemp -d)
	trap 'rm -rf "$stage"' EXIT

	archive_plugins "$version" "$stage"
	echo "archived plugins/ (+ VERSION) -> $stage/index.tar.gz:"
	tar -tzf "$stage/index.tar.gz"

	echo "publishing ${PACKAGE_NAME}@${version} to feed '${FEED}'..."
	publish_archive "$version" "$stage"

	echo
	echo "== published ${PACKAGE_NAME}@${version} =="
}

main "$@"
