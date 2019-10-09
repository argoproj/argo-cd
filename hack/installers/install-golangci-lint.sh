#!/bin/bash
set -eux -o pipefail

cd $DOWNLOADS
GO111MODULE=on go get golangci/golangci-lint@v1.20