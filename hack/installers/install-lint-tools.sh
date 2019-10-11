#!/bin/bash
set -eux -o pipefail

mkdir -p $DOWNLOADS/lint-tools
cd $DOWNLOADS/lint-tools

# later versions seem to need go1.13
GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.18.0
