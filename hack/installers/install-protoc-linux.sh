#!/bin/bash
set -eux -o pipefail

[ -e $DOWNLOADS/protoc.zip ] || curl -sLf --retry 3 -o $DOWNLOADS/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
unzip $DOWNLOADS/protoc.zip bin/protoc -d /usr/local/
chmod +x /usr/local/bin/protoc
unzip $DOWNLOADS/protoc.zip include/* -d /usr/local/
protoc --version
