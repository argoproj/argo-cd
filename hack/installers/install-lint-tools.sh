#!/bin/bash
set -eux -o pipefail

GO111MODULE=on go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.26.0
