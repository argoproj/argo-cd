#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

export TARGET_FILE=git-lfs-${INSTALL_OS}-${ARCHITECTURE}-v${git_lfs_version}.tar.gz

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "https://github.com/git-lfs/git-lfs/releases/download/v${git_lfs_version}/${TARGET_FILE}"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/git-lfs && tar -C /tmp/git-lfs --strip-components=1 -xzf "$DOWNLOADS/${TARGET_FILE}"
sudo install -m 0755 "/tmp/git-lfs/git-lfs" "$BIN/git-lfs"
# Version check may fail under QEMU user-mode emulation due to Go runtime
# crashes (lfstack.push/taggedPointerPack). Checksum verification above
# ensures binary integrity, so a failed smoke test is non-fatal.
if ! git-lfs version 2>/dev/null; then
  echo "Warning: git-lfs version check failed (possibly QEMU emulation). Binary integrity was verified by checksum." >&2
fi
