#!/bin/bash
set -eux -o pipefail

# renovate: datasource=go packageName=github.com/yannh/kubeconform
KUBECONFORM_VERSION=0.8.0

GO111MODULE=on go install "github.com/yannh/kubeconform/cmd/kubeconform@v${KUBECONFORM_VERSION}"
