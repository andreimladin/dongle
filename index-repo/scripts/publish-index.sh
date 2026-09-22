#!/usr/bin/env bash
# Manual, local equivalent of ../azure-pipelines-publish-index.yml — what a
# maintainer runs by hand today (the pipeline isn't wired up / triggerable
# yet) to do exactly what that pipeline will do once it is: tag the next
# monotonic 0.0.N version, archive plugins/ (+ a VERSION file) into
# index.tar.gz, and publish it to the shared feed as the dongle-index
# Universal Package.
#
# This script IS the shared implementation, not a parallel one:
# azure-pipelines-publish-index.yml calls this same script for all of the
# version-compute/tag/archive/publish logic below (with ORGANIZATION/
# PROJECT/FEED/PACKAGE_NAME set from its own variables block, and
# AZURE_DEVOPS_EXT_PAT already in the environment instead of an
# interactive `az login`) — so a manual run and a pipeline run always
# produce byte-identical archives under the exact same versioning logic.
# Do not duplicate or reimplement this logic elsewhere; if the pipeline's
# needs and this script's ever diverge, change it here and let the
# pipeline pick it up, not the other way around.
#
# Auth: uses whatever the caller already has.
#   - Interactively (a maintainer's machine): run `az login` first — this
#     script does not log you in itself.
#   - From the pipeline: AZURE_DEVOPS_EXT_PAT (System.AccessToken) is
#     already set in the environment, and `az artifacts`/`az repos` pick
#     it up on their own, no `az login` needed — see the "Two different
#     auth paths" note in azure-pipelines-validate.yml for the same
#     pattern used there. The git tag push instead relies on the
#     pipeline's `checkout: self, persistCredentials: true`.
#
# Requires: az (with the azure-devops extension: `az extension add --name
# azure-devops`), git, tar.
#
# Usage: ./scripts/publish-index.sh
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

# guard_clean_main refuses to publish unless HEAD is main and the working
# tree is clean, so a manual publish always corresponds to a clean,
# committed state on main — never a half-finished local edit or a feature
# branch. BUILD_SOURCEBRANCHNAME (set by Azure Pipelines) is checked first
# because a pipeline's `checkout: self` typically leaves the repo in
# detached HEAD, where `git rev-parse --abbrev-ref HEAD` would report
# "HEAD" rather than "main" even on a legitimate main-branch build.
guard_clean_main() {
	local branch
	if [ -n "${BUILD_SOURCEBRANCHNAME:-}" ]; then
		branch="$BUILD_SOURCEBRANCHNAME"
	else
		branch=$(git rev-parse --abbrev-ref HEAD)
	fi
	if [ "$branch" != "main" ]; then
		echo "error: refusing to publish from '$branch' — this must be run on main" >&2
		exit 1
	fi
	if [ -n "$(git status --porcelain)" ]; then
		echo "error: working tree is dirty — commit or stash changes before publishing" >&2
		exit 1
	fi
}

# next_version reads the highest existing 0.0.* tag and returns the next
# monotonic value as bare semver 0.0.<N+1> (or 0.0.1 if none exist yet).
next_version() {
	git fetch --tags --force >&2

	local latest
	latest=$(git tag --list | grep -E '^0\.0\.[0-9]+$' | sed -E 's/^0\.0\.//' | sort -n | tail -1)

	local next
	if [ -z "$latest" ]; then
		next=1
	else
		next=$((latest + 1))
	fi
	echo "0.0.${next}"
}

tag_and_push() {
	local version="$1"
	git tag "$version"
	git push origin "$version"
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
	guard_clean_main

	local version
	version=$(next_version)
	echo "next version: ${version}"

	tag_and_push "$version"
	echo "tagged and pushed ${version}"

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
