#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)
INSTALL_PATH="${BIN:-$PROJECT_ROOT/dist}"

export TARGET_FILE=git-lfs-${INSTALL_OS}-${ARCHITECTURE}-v${git_lfs_version}.tar.gz

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "https://github.com/git-lfs/git-lfs/releases/download/v${git_lfs_version}/${TARGET_FILE}"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/git-lfs && tar -C /tmp/git-lfs --strip-components=1 -xzf "$DOWNLOADS/${TARGET_FILE}"
if [ -w "$INSTALL_PATH" ]; then
  install -m 0755 "/tmp/git-lfs/git-lfs" "$INSTALL_PATH/git-lfs"
else
  sudo install -m 0755 "/tmp/git-lfs/git-lfs" "$INSTALL_PATH/git-lfs"
fi
git-lfs version
