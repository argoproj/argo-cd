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
GOPATH=$(go env GOPATH)
GOPATH_PROJECT_ROOT="${GOPATH}/src/github.com/argoproj/argo-cd"

# Both served API versions get OpenAPI definitions. v1alpha1 is generated first so
# that v1beta1 (which embeds/references v1alpha1 types) can $ref its definitions.
VERSIONS="v1alpha1 v1beta1"

[ -e ./v3 ] || ln -s . v3
[ -e "${GOPATH_PROJECT_ROOT}" ] || (mkdir -p "$(dirname "${GOPATH_PROJECT_ROOT}")" && ln -s "${PROJECT_ROOT}" "${GOPATH_PROJECT_ROOT}")

# kube-openapi's api linter truncates its report file on every invocation
# (os.Create), so pointing both versions at the same --report-filename would
# leave only the last version's violations in the list. Each version reports
# to its own temporary file; they are merged into the single committed list
# afterwards.
REPORT_TMP_DIR=$(mktemp -d)
trap 'rm -rf "${REPORT_TMP_DIR}"' EXIT

for VERSION in ${VERSIONS}; do
  openapi-gen \
    --go-header-file "${PROJECT_ROOT}/hack/custom-boilerplate.go.txt" \
    --output-pkg "github.com/argoproj/argo-cd/v3/pkg/apis/application/${VERSION}" \
    --report-filename "${REPORT_TMP_DIR}/${VERSION}.list" \
    --output-dir "${GOPATH}/src" \
    "$@"
done

: > "${PROJECT_ROOT}/pkg/apis/api-rules/violation_exceptions.list"
for VERSION in ${VERSIONS}; do
  cat "${REPORT_TMP_DIR}/${VERSION}.list" >> "${PROJECT_ROOT}/pkg/apis/api-rules/violation_exceptions.list"
done

[ -L "${GOPATH_PROJECT_ROOT}" ] && rm -rf "${GOPATH_PROJECT_ROOT}"
[ -L ./v3 ] && rm -rf v3

export GO111MODULE=on
go build -o ./dist/gen-crd-spec "${PROJECT_ROOT}/hack/gen-crd-spec"
./dist/gen-crd-spec
