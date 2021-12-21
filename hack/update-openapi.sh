#!/bin/bash

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname "$0")/.. ; pwd)
PATH="${PROJECT_ROOT}/dist:${PATH}"
VERSION="v1alpha1"

[ -e ./v2 ] || ln -s . v2
openapi-gen \
  --go-header-file ${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
  --input-dirs github.com/argoproj/argo-cd/v2/pkg/apis/application/${VERSION} \
  --output-package github.com/argoproj/argo-cd/v2/pkg/apis/application/${VERSION} \
  --report-filename pkg/apis/api-rules/violation_exceptions.list \
  $@
[ -e ./v2 ] && rm -rf v2

export GO111MODULE=off
go build -o ./dist/gen-crd-spec ${PROJECT_ROOT}/hack/gen-crd-spec
./dist/gen-crd-spec
