#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

case $ARCHITECTURE in
  arm64|arm)
    export TARGET_FILE=protoc_${protoc_version}_linux_${ARCHITECTURE}.zip
    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-linux-aarch_64.zip
    $(dirname $0)/compare-chksum.sh
    mkdir -p /tmp/protoc-${protoc_version}
    unzip $DOWNLOADS/${TARGET_FILE} -d /tmp/protoc-${protoc_version}
    sudo install -m 0755 /tmp/protoc-${protoc_version}/bin/protoc /usr/local/bin/protoc
    sudo cp -a /tmp/protoc-${protoc_version}/include/* /usr/local/include
    protoc --version
    ;;
  *)
    export TARGET_FILE=protoc_${protoc_version}_linux_${ARCHITECTURE}.zip
    [ -e $DOWNLOADS/${TARGET_FILE} ] || curl -sLf --retry 3 -o $DOWNLOADS/${TARGET_FILE} https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-linux-x86_64.zip
    $(dirname $0)/compare-chksum.sh
    mkdir -p /tmp/protoc-${protoc_version}
    unzip $DOWNLOADS/${TARGET_FILE} -d /tmp/protoc-${protoc_version}
    sudo install -m 0755 /tmp/protoc-${protoc_version}/bin/protoc /usr/local/bin/protoc
    sudo cp -a /tmp/protoc-${protoc_version}/include/* /usr/local/include
    protoc --version
    ;;
esac