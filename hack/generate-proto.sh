#!/bin/bash

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/..; pwd)
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${PROJECT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}

go build -i -o dist/go-to-protobuf ./vendor/k8s.io/code-generator/cmd/go-to-protobuf
go build -i -o dist/protoc-gen-gogo ./vendor/k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo

PATH="${PROJECT_ROOT}/dist:${PATH}"

# Generate protobufs for our types
PACKAGES=(
    github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1
)
go-to-protobuf \
    --logtostderr \
    --go-header-file=${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
    --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
    --apimachinery-packages=-k8s.io/apimachinery/pkg/apis/meta/v1,-k8s.io/api/core/v1,-k8s.io/apimachinery/pkg/runtime/schema \
    --proto-import=./vendor


# Generate protobufs for our services
PROTO_FILES=$(find ${PROJECT_ROOT}/server -name "*.proto" -not -path "${PROJECT_ROOT}/vendor/*")
for i in ${PROTO_FILES}; do
    # Both /root/go and ${PROJECT_ROOT} are added to the protoc includes, in order to support
    # the requirement of running make inside docker and on desktop, respectively.
    protoc \
        -I${PROJECT_ROOT} \
        -I/usr/local/include \
        -I./vendor \
        -I$GOPATH/src \
        -I/root/go/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
        -I${PROJECT_ROOT}/vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis \
        --go_out=plugins=grpc:$GOPATH/src \
        --grpc-gateway_out=logtostderr=true:$GOPATH/src \
        $i
done
