#!/usr/bin/env bash

# This script requires bash shell - sorry.

NEW_TAG="${1}"
GIT_REMOTE="${2}"

set -ue

if test "${NEW_TAG}" = "" -o "${GIT_REMOTE}" = ""; then
	echo "!! Usage: $0 <release tag> <remote>" >&2
	exit 1
fi

# Target (version) tag must match version scheme vMAJOR.MINOR.PATCH with an
# optional pre-release tag.
if ! echo "${NEW_TAG}" | grep -E -q '^v[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)*$'; then
	echo "!! Malformed version tag: '${NEW_TAG}', must match 'vMAJOR.MINOR.PATCH(-rcX)'" >&2
	exit 1
fi

# Check whether we are in correct branch of local repository
RELEASE_BRANCH="${NEW_TAG%\.[0-9]*}"
RELEASE_BRANCH="release-${RELEASE_BRANCH#*v}"

currentBranch=$(git branch --show-current)
if test "$currentBranch" != "${RELEASE_BRANCH}"; then
	echo "!! Please checkout branch '${RELEASE_BRANCH}' (currently in branch: '${currentBranch}')" >&2
	exit 1
fi

echo ">> Working in release branch '${RELEASE_BRANCH}'"

# Safety check: Warn if pushing to official argoproj/argo-cd repository
REMOTE_URL=$(git remote get-url "${GIT_REMOTE}")
if echo "${REMOTE_URL}" | grep -q "argoproj/argo-cd"; then
	echo "" >&2
	echo "!! ============================================================================" >&2
	echo "!! WARNING: Remote '${GIT_REMOTE}' points to OFFICIAL argoproj/argo-cd!" >&2
	echo "!! Remote URL: ${REMOTE_URL}" >&2
	echo "!! ============================================================================" >&2
	echo "!!" >&2
	echo "!! This will create an OFFICIAL Argo CD release:" >&2
	echo "!!   - Tag: ${NEW_TAG}" >&2
	echo "!!   - Images: quay.io/argoproj/argocd:${NEW_TAG}" >&2
	echo "!!   - GitHub Release: https://github.com/argoproj/argo-cd/releases" >&2
	echo "!!   - Visible to ALL users" >&2
	echo "!!" >&2
	echo "!! If you want to release from YOUR FORK:" >&2
	echo "!!   1. Press Ctrl+C now" >&2
	echo "!!   2. Use your fork remote: ./hack/trigger-release.sh ${NEW_TAG} origin" >&2
	echo "!!" >&2
	echo "!! To proceed with OFFICIAL release, type 'y' (30 second timeout):" >&2
	read -t 30 -r confirmation
	if [ "$confirmation" != "y" ]; then
		echo "!! Cancelled. Did not receive 'y' confirmation." >&2
		exit 1
	fi
	echo ">> Confirmed official release. Proceeding..." >&2
fi

echo ">> Ensuring release branch is up to date."
# make sure release branch is up to date
git pull "${GIT_REMOTE}" "${RELEASE_BRANCH}"

# Check for target (version) tag in local repo
if git tag -l | grep -q -E "^${NEW_TAG}$"; then
	echo "!! Target version tag '${NEW_TAG}' already exists in local repository" >&2
	exit 1
fi

# Check for target (version) tag in remote repo
if git ls-remote "${GIT_REMOTE}" "refs/tags/${NEW_TAG}" | grep -q -E "${NEW_TAG}$"; then
	echo "!! Target version tag '${NEW_TAG}' already exists in remote '${GIT_REMOTE}'" >&2
	exit 1
fi

echo ">> Creating new release '${NEW_TAG}' by pushing '${NEW_TAG}' to '${GIT_REMOTE}'"

# Create new tag in local repository
git tag "${NEW_TAG}"

# Push the new tag to remote repository
git push "${GIT_REMOTE}" "${NEW_TAG}"
