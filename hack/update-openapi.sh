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

TARGET_SCRIPT=kube_codegen.sh

# codegen utilities are installed outside of kube_codegen.sh so remove the `go install` step in the script.
sed -e '/go install/d' ${PROJECT_ROOT}/vendor/k8s.io/code-generator/kube_codegen.sh >${TARGET_SCRIPT}
# generate-groups.sh assumes codegen utilities are installed to GOBIN, but we just ensure the CLIs
# are in the path and invoke them without assumption of their location
sed -i.bak -e 's#${gobin}/##g' ${TARGET_SCRIPT}

VERSION="v1alpha1"

[ -e ./v2 ] || ln -s . v2
[ -e "${GOPATH_PROJECT_ROOT}" ] || (mkdir -p "$(dirname "${GOPATH_PROJECT_ROOT}")" && ln -s "${PROJECT_ROOT}" "${GOPATH_PROJECT_ROOT}")

# shellcheck source=pkg/apis/application/v1alpha1/kube_codegen.sh
. ${TARGET_SCRIPT}

kube::codegen::gen_helpers pkg/apis/application/v1alpha1
kube::codegen::gen_openapi pkg/apis \
  --output-dir "pkg/apis/application/${VERSION}" \
  --output-pkg github.com/argoproj/argo-cd/v2/pkg/apis/application/${VERSION} \
  --boilerplate ${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
  --report-filename pkg/apis/api-rules/violation_exceptions.list \
  --update-report

rm ${TARGET_SCRIPT}
rm ${TARGET_SCRIPT}.bak

[ -L "${GOPATH_PROJECT_ROOT}" ] && rm -rf "${GOPATH_PROJECT_ROOT}"
[ -L ./v2 ] && rm -rf v2

export GO111MODULE=on
go build -o ./dist/gen-crd-spec "${PROJECT_ROOT}/hack/gen-crd-spec"
./dist/gen-crd-spec
