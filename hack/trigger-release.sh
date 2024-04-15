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
if ! echo "${NEW_TAG}" | egrep -q '^v[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)*$'; then
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

echo ">> Ensuring release branch is up to date."
# make sure release branch is up to date
git pull ${GIT_REMOTE} ${RELEASE_BRANCH}

# Check for target (version) tag in local repo
if git tag -l | grep -q -E "^${NEW_TAG}$"; then
	echo "!! Target version tag '${NEW_TAG}' already exists in local repository" >&2
	exit 1
fi

# Check for target (version) tag in remote repo
if git ls-remote ${GIT_REMOTE} refs/tags/${NEW_TAG} | grep -q -E "${NEW_TAG}$"; then
	echo "!! Target version tag '${NEW_TAG}' already exists in remote '${GIT_REMOTE}'" >&2
	exit 1
fi

echo ">> Creating new release '${NEW_TAG}' by pushing '${NEW_TAG}' to '${GIT_REMOTE}'"

# Create new tag in local repository
git tag ${NEW_TAG}

# Push the new tag to remote repository
git push ${GIT_REMOTE} ${NEW_TAG}
