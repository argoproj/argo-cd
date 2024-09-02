#!/bin/bash

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(
  cd $(dirname "$0")/..
  pwd
)
PATH="${PROJECT_ROOT}/dist:${PATH}"
GOPATH=$(go env GOPATH)
GOPATH_PROJECT_ROOT="${GOPATH}/src/github.com/argoproj/argo-cd"

VERSION="v1alpha1"

[ -e ./v2 ] || ln -s . v2
[ -e "${GOPATH_PROJECT_ROOT}" ] || (mkdir -p "$(dirname "${GOPATH_PROJECT_ROOT}")" && ln -s "${PROJECT_ROOT}" "${GOPATH_PROJECT_ROOT}")

openapi-gen \
  --go-header-file ${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
  --output-pkg github.com/argoproj/argo-cd/v2/pkg/apis/application/${VERSION} \
  --report-filename pkg/apis/api-rules/violation_exceptions.list \
  --output-dir "${GOPATH}/src" \
  $@

[ -L "${GOPATH_PROJECT_ROOT}" ] && rm -rf "${GOPATH_PROJECT_ROOT}"
[ -L ./v2 ] && rm -rf v2

export GO111MODULE=on
go build -o ./dist/gen-crd-spec "${PROJECT_ROOT}/hack/gen-crd-spec"
./dist/gen-crd-spec
