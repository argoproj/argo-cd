#!/bin/bash
set -eux -o pipefail

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/../..; pwd)
DIST_PATH="${PROJECT_ROOT}/dist"
PATH="${DIST_PATH}:${PATH}"

. $(dirname $0)/../tool-versions.sh

OS=$(go env GOOS)
case $OS in
  darwin)
    buf_os=Darwin
    buf_arch=x86_64
    ;;
  *)
    buf_os=Linux
    case $ARCHITECTURE in
      arm64|arm)
        buf_arch=aarch_64
        ;;
      *)
        buf_arch=x86_64
        ;;
    esac
    ;;
esac

export TARGET_FILE=buf_${buf_version}
url=https://github.com/bufbuild/buf/releases/download/v${buf_version}/buf-${buf_os}-${buf_arch}
[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DIST_PATH}/${TARGET_FILE} ${url}
chmod +x ${DIST_PATH}/${TARGET_FILE}
cp ${DIST_PATH}/${TARGET_FILE} ${DIST_PATH}/buf
buf --version
