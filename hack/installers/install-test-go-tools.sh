#!/bin/bash
set -eux -o pipefail

PROJECT_ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")"/../.. && pwd)


# All binaries are compiled into the argo-cd/dist directory
export GOBIN="${PROJECT_ROOT}/dist"
mkdir -p "$GOBIN"

# renovate: datasource=go packageName=github.com/jstemmer/go-junit-report/v2
JUNIT_REPORT_VERSION=2.1.0
go install github.com/jstemmer/go-junit-report/v2@v${JUNIT_REPORT_VERSION}
