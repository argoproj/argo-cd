#!/bin/bash
set -eux -o pipefail

curl -sLf -C - -o /tmp/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v3.7.1/protoc-3.7.1-linux-x86_64.zip
unzip /tmp/protoc.zip bin/protoc -d /usr/local/
sudo chmod +x /usr/local/bin/protoc
sudo unzip /tmp/protoc.zip include/* -d /usr/local/
protoc --version
