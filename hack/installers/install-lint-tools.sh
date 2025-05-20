#!/bin/bash
set -eux -o pipefail

GO111MODULE=on go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.58.2
