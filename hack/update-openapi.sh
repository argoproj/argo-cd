#!/bin/bash

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(
  cd "$(dirname "$0")/.."
  pwd
)
PATH="${PROJECT_ROOT}/dist:${PATH}"

VERSION="v1alpha1"

# Module path uses /v3; create a local symlink so go list resolves the package.
[ -e ./v3 ] || ln -s . v3

openapi-gen \
  --go-header-file "${PROJECT_ROOT}/hack/custom-boilerplate.go.txt" \
  --output-pkg github.com/argoproj/argo-cd/v3/pkg/apis/application/${VERSION} \
  --output-file openapi_generated.go \
  --output-dir "${PROJECT_ROOT}/pkg/apis/application/${VERSION}" \
  --report-filename pkg/apis/api-rules/violation_exceptions.list \
  github.com/argoproj/argo-cd/v3/pkg/apis/application/${VERSION} \
  "$@"

[ -L ./v3 ] && rm -rf v3

export GO111MODULE=on
go build -o ./dist/gen-crd-spec "${PROJECT_ROOT}/hack/gen-crd-spec"
./dist/gen-crd-spec
