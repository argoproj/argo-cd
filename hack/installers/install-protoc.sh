#!/bin/bash
set -eux -o pipefail

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/../..; pwd)
DIST_PATH="${PROJECT_ROOT}/dist"
PATH="${DIST_PATH}:${PATH}"

. $(dirname $0)/../tool-versions.sh

OS=$(go env GOOS)
case $OS in
  darwin)
    # For macOS, the x86_64 binary is used even on Apple Silicon (it is run through rosetta), so
    # we download and install the x86_64 version. See: https://github.com/protocolbuffers/protobuf/pull/8557
    protoc_os=osx
    protoc_arch=x86_64
    ;;
  *)
    protoc_os=linux
    case $ARCHITECTURE in
      arm64|arm)
        protoc_arch=aarch_64
        ;;
      s390x)
        protoc_arch=s390_64
        ;;
      ppc64le)
        protoc_arch=ppcle_64
        ;;
      *)
        protoc_arch=x86_64
        ;;
    esac
    ;;
esac

export TARGET_FILE=protoc-${protoc_version}-${protoc_os}-${protoc_arch}.zip
url=https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-${protoc_os}-${protoc_arch}.zip
[ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} ${url}
$(dirname $0)/compare-chksum.sh
mkdir -p /tmp/protoc-${protoc_version}
unzip -o $DOWNLOADS/${TARGET_FILE} -d /tmp/protoc-${protoc_version}
mkdir -p ${DIST_PATH}/protoc-include
sudo install -m 0755 /tmp/protoc-${protoc_version}/bin/protoc ${DIST_PATH}/protoc
(cd /tmp/protoc-${protoc_version}/include/ && find -- * -type d -exec install -m 0755 -d "${DIST_PATH}/protoc-include/{}" \;)
(cd /tmp/protoc-${protoc_version}/include/ && find -- * -type f -exec install -m 0644 "/tmp/protoc-${protoc_version}/include/{}" "${DIST_PATH}/protoc-include/{}" \;)
protoc --version
