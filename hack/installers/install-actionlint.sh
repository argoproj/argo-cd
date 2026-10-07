#!/bin/bash
set -eux -o pipefail

# renovate: datasource=go packageName=github.com/rhysd/actionlint
ACTIONLINT_VERSION=1.7.12

GO111MODULE=on go install "github.com/rhysd/actionlint/cmd/actionlint@v${ACTIONLINT_VERSION}"
