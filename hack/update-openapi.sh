#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname "$0")/.. ; pwd)
CODEGEN_PKG=$(go env GOPATH)/pkg/mod/k8s.io/kube-openapi@v0.0.0-20190502190224-411b2483e503
VERSION="v1alpha1"

go run ${CODEGEN_PKG}/cmd/openapi-gen/openapi-gen.go \
  --go-header-file ${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
  --input-dirs github.com/argoproj/argo-cd/pkg/apis/application/${VERSION} \
  --output-package github.com/argoproj/argo-cd/pkg/apis/application/${VERSION} \
  --report-filename pkg/apis/api-rules/violation_exceptions.list \
  $@

go run ./hack/gen-crd-spec/main.go
