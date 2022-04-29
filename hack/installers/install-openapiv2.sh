#!/bin/bash
set -eux -o pipefail

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/../..; pwd)
DIST_PATH="${PROJECT_ROOT}/dist"
PATH="${DIST_PATH}:${PATH}"

. $(dirname $0)/../tool-versions.sh

OS=$(go env GOOS)
case $OS in
  darwin)
    openapiv2_os=darwin
    openapiv2_arch=x86_64
    ;;
  *)
    openapiv2_os=linux
    case $ARCHITECTURE in
      arm64|arm)
        openapiv2_arch=aarch_64
        ;;
      *)
        openapiv2_arch=x86_64
        ;;
    esac
    ;;
esac

export TARGET_FILE=protoc-gen-openapiv2_${openapiv2_version}
url=https://github.com/grpc-ecosystem/grpc-gateway/releases/download/v${openapiv2_version}/protoc-gen-openapiv2-v${openapiv2_version}-${openapiv2_os}-${openapiv2_arch}
[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o ${DIST_PATH}/${TARGET_FILE} ${url}
chmod +x ${DIST_PATH}/${TARGET_FILE}
cp ${DIST_PATH}/${TARGET_FILE} ${DIST_PATH}/protoc-gen-openapiv2
protoc-gen-openapiv2 --version
