#!/bin/bash
set -eux -o pipefail

# renovate: datasource=go packageName=github.com/golangci/golangci-lint
GOLANGCI_LINT_VERSION=1.62.2

GO111MODULE=on go install "github.com/golangci/golangci-lint/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}"
