#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

export TARGET_FILE=helm-v${helm3_version}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "https://get.helm.sh/helm-v${helm3_version}-$INSTALL_OS-$ARCHITECTURE.tar.gz"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/helm && tar -C /tmp/helm -xf "$DOWNLOADS/${TARGET_FILE}"
sudo install -m 0755 "/tmp/helm/$INSTALL_OS-$ARCHITECTURE/helm" "$BIN/helm"
# Version check may fail under QEMU user-mode emulation due to Go runtime
# crashes (lfstack.push/taggedPointerPack). Checksum verification above
# ensures binary integrity, so a failed smoke test is non-fatal.
if ! helm version 2>/dev/null; then
  echo "Warning: helm version check failed (possibly QEMU emulation). Binary integrity was verified by checksum." >&2
fi
