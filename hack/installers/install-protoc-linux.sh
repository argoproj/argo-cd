#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

case $ARCHITECTURE in
  arm|arm64)
    [ -e $DOWNLOADS/protoc.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-linux-aarch_64.zip
    ;;
  *)
    [ -e $DOWNLOADS/protoc.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-linux-x86_64.zip
    ;;
esac

unzip $DOWNLOADS/protoc.zip bin/protoc -d /usr/local/
chmod +x /usr/local/bin/protoc
unzip $DOWNLOADS/protoc.zip include/* -d /usr/local/
protoc --version
