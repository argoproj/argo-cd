#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

GO111MODULE=on
go get k8s.io/kube-openapi@v0.0.0-20190502190224-411b2483e503

PROJECT_ROOT=$(cd $(dirname "$0")/.. ; pwd)
VERSION="v1alpha1"

go run $GOPATH//pkg/mod/k8s.io/kube-openapi@v0.0.0-20190918143330-0270cf2f1c1d/cmd/openapi-gen/openapi-gen.go \
  --go-header-file ${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
  --input-dirs github.com/argoproj/argo-cd/pkg/apis/application/${VERSION} \
  --output-package github.com/argoproj/argo-cd/pkg/apis/application/${VERSION} \
  --report-filename pkg/apis/api-rules/violation_exceptions.list \
  $@

go run ./hack/gen-crd-spec/main.go
