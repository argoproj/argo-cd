#!/usr/bin/env bash

# This script requires bash shell - sorry.

NEW_TAG="${1}"
GIT_REMOTE="${2}"
COMMIT_MSG="${3}"
origToken=""

set -ue

restoreToken() {
	if test "$origToken" != ""; then
		echo ">> Restoring original Git comment char"
		git config core.commentChar "$origToken"
	fi
}

cleanLocalTriggerTag() {
	if test "$TRIGGER_TAG" != ""; then
		echo ">> Remove trigger tag '${TRIGGER_TAG}' from local repository."
		git tag -d $TRIGGER_TAG
	fi
}

cleanup() {
	restoreToken
	cleanLocalTriggerTag
}

if test "${NEW_TAG}" = "" -o "${GIT_REMOTE}" = ""; then
	echo "!! Usage: $0 <release tag> <remote> [path to release notes file]" >&2
	exit 1
fi

# Target (version) tag must match version scheme vMAJOR.MINOR.PATCH with an
# optional pre-release tag.
if ! echo "${NEW_TAG}" | egrep -q '^v[0-9]+\.[0-9]+\.[0-9]+(-rc[0-9]+)*$'; then
	echo "!! Malformed version tag: '${NEW_TAG}', must match 'vMAJOR.MINOR.PATCH(-rcX)'" >&2
	exit 1
fi

TRIGGER_TAG="release-${NEW_TAG}"

# Check whether we are in correct branch of local repository
RELEASE_BRANCH="${NEW_TAG%\.[0-9]*}"
RELEASE_BRANCH="release-${RELEASE_BRANCH#*v}"

currentBranch=$(git branch --show-current)
if test "$currentBranch" != "${RELEASE_BRANCH}"; then
	echo "!! Please checkout branch '${RELEASE_BRANCH}' (currently in branch: '${currentBranch}')" >&2
	exit 1
fi

echo ">> Working in release branch '${RELEASE_BRANCH}'"

# Check for trigger tag existing in local repo
if git tag -l | grep -q -E "^${TRIGGER_TAG}$"; then
	echo "!! Release tag '${TRIGGER_TAG}' already exists in local repository" >&2
	exit 1
fi

# Check for trigger tag existing in remote repo
if git ls-remote ${GIT_REMOTE} refs/tags/${TRIGGER_TAG} | grep -q -E "^${NEW_TAG}$"; then
	echo "!! Target trigger tag '${TRIGGER_TAG}' already exists in remote '${GIT_REMOTE}'" >&2
	echo "!! Another operation currently in progress?" >&2
	exit 1
fi

# Check for target (version) tag in local repo
if git tag -l | grep -q -E "^${NEW_TAG}$"; then
	echo "!! Target version tag '${NEW_TAG}' already exists in local repository" >&2
	exit 1
fi

# Check for target (version) tag in remote repo
if git ls-remote ${GIT_REMOTE} refs/tags/${NEW_TAG} | grep -q -E "^${NEW_TAG}$"; then
	echo "!! Target version tag '${NEW_TAG}' already exists in remote '${GIT_REMOTE}'" >&2
	exit 1
fi

echo ">> Creating new release '${NEW_TAG}' by pushing '${TRIGGER_TAG}' to '${GIT_REMOTE}'"

GIT_ARGS=""
if test "${COMMIT_MSG}" != ""; then
	if ! test -f "${COMMIT_MSG}"; then
		echo "!! Release notes at '${COMMIT_MSG}' do not exist or are not readable." >&2
		exit 1
	fi
	GIT_ARGS="-F ${COMMIT_MSG}"
fi

# We need different git comment char than '#', because markdown makes extensive
# use of '#' - we chose ';' for our operation.
origToken=$(git config core.commentChar || echo '#')
echo ">> Saving original Git comment char '${origToken}' and setting it to ';' for this run"
if ! git config core.commentChar ';'; then
	echo "!! Could not set git config commentChar ';'" >&2
	exit 1
fi

trap cleanup SIGINT EXIT

# Create trigger tag in local repository
git tag -a ${GIT_ARGS} ${TRIGGER_TAG}

# Push the trigger tag to remote repository
git push ${GIT_REMOTE} ${TRIGGER_TAG}
