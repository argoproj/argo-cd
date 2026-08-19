#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

INSTALL_SUDO=${INSTALL_SUDO:-sudo}

export TARGET_FILE=helm-v${HELM_VERSION}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "https://get.helm.sh/helm-v${HELM_VERSION}-$INSTALL_OS-$ARCHITECTURE.tar.gz"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/helm && tar -C /tmp/helm -xf "$DOWNLOADS/${TARGET_FILE}"
${INSTALL_SUDO} install -m 0755 "/tmp/helm/$INSTALL_OS-$ARCHITECTURE/helm" "$BIN/helm"
"$BIN/helm" version
