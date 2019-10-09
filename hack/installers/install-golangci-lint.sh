#!/bin/bash
set -eux -o pipefail

cd $DOWNLOADS
GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.20.0