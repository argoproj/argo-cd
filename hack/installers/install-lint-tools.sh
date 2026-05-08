#!/bin/bash
set -eux -o pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)

# All binaries are compiled into the argo-cd/dist directory, which is added to the PATH during codegen
export GOBIN="${PROJECT_ROOT}/dist"
mkdir -p "$GOBIN"

# renovate: datasource=go packageName=github.com/golangci/golangci-lint/v2
GOLANGCI_LINT_VERSION=2.11.4

GO111MODULE=on go install "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v${GOLANGCI_LINT_VERSION}"
