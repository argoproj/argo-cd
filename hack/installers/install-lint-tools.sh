#!/bin/bash
set -eux -o pipefail

# renovate: datasource=go packageName=github.com/golangci/golangci-lint
GOLANGCI_LINT_VERSION=2.2.1

GO111MODULE=on go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}"
