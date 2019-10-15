#!/bin/bash
set -eux -o pipefail

mkdir -p $DOWNLOADS/codegen-tools
cd $DOWNLOADS/codegen-tools

GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.21.0
