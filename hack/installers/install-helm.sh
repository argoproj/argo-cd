#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)
INSTALL_PATH="${BIN:-$PROJECT_ROOT/dist}"

export TARGET_FILE=helm-v${helm3_version}-${INSTALL_OS}-${ARCHITECTURE}.tar.gz

[ -e "$DOWNLOADS/${TARGET_FILE}" ] || curl -sLf --retry 3 -o "$DOWNLOADS/${TARGET_FILE}" "https://get.helm.sh/helm-v${helm3_version}-$INSTALL_OS-$ARCHITECTURE.tar.gz"
"$(dirname "$0")"/compare-chksum.sh
mkdir -p /tmp/helm && tar -C /tmp/helm -xf "$DOWNLOADS/${TARGET_FILE}"
if [ -w "$INSTALL_PATH" ]; then
  install -m 0755 "/tmp/helm/$INSTALL_OS-$ARCHITECTURE/helm" "$INSTALL_PATH/helm"
else
  sudo install -m 0755 "/tmp/helm/$INSTALL_OS-$ARCHITECTURE/helm" "$INSTALL_PATH/helm"
fi
helm version
