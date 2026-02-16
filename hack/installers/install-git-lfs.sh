#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

export TARGET_FILE=git-lfs-${INSTALL_OS}-${ARCHITECTURE}-v${git_lfs_version}.tar.gz

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "https://github.com/git-lfs/git-lfs/releases/download/v${git_lfs_version}/${TARGET_FILE}"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/git-lfs && tar -C /tmp/git-lfs --strip-components=1 -xf "$DOWNLOADS/${TARGET_FILE}"
sudo install -m 0755 "/tmp/git-lfs/git-lfs" "$BIN/git-lfs"
git-lfs version
