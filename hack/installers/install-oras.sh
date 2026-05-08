#!/bin/bash
set -eux -o pipefail

. "$(dirname "$0")"/../tool-versions.sh

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)
INSTALL_PATH="${BIN:-$PROJECT_ROOT/dist}"
PATH="${INSTALL_PATH}:${PATH}"
[ -d "$INSTALL_PATH" ] || mkdir -p "$INSTALL_PATH"

# shellcheck disable=SC2154
export TARGET_FILE=oras_${oras_version}_${INSTALL_OS}_${ARCHITECTURE}.tar.gz
# shellcheck disable=SC2154
[ -e "$DOWNLOADS"/"${TARGET_FILE}" ] || curl -sLf --retry 3 -o "${DOWNLOADS}"/"${TARGET_FILE}" "https://github.com/oras-project/oras/releases/download/v${oras_version}/oras_${oras_version}_${INSTALL_OS}_${ARCHITECTURE}.tar.gz"
"$(dirname "$0")"/compare-chksum.sh

tar -C /tmp -xf "${DOWNLOADS}"/"${TARGET_FILE}"
if [ -w "$INSTALL_PATH" ]; then
  install -m 0755 /tmp/oras "$INSTALL_PATH"/oras
else
  sudo install -m 0755 /tmp/oras "$INSTALL_PATH"/oras
fi
