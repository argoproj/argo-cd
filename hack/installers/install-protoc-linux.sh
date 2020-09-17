#!/bin/bash
set -eux -o pipefail

. $(dirname $0)/../tool-versions.sh

[ -e $DOWNLOADS/protoc.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v${protoc_version}/protoc-${protoc_version}-linux-x86_64.zip
unzip $DOWNLOADS/protoc.zip bin/protoc -d /usr/local/
chmod +x /usr/local/bin/protoc
unzip $DOWNLOADS/protoc.zip include/* -d /usr/local/
protoc --version
