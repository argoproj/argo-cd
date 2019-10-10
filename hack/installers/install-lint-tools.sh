#!/bin/bash
set -eux -o pipefail

mkdir -p $DOWNLOADS/codegen-tools
cd $DOWNLOADS/codegen-tools

# later versions seem to need go1.13
GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.18.0
GO111MODULE=on go get golang.org/x/tools/cmd/goimports@v0.0.0-20190627203933-19ff4fff8850
