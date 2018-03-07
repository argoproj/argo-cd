#!/bin/bash

# This script auto-generates protobuf related files. It is intended to be run manually when either
# API types are added/modified, or server gRPC calls are added. The generated files should then
# be checked into source control.

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/..; pwd)
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${PROJECT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../code-generator)}
PATH="${PROJECT_ROOT}/dist:${PATH}"

# protbuf tooling required to build .proto files from go annotations from k8s-like api types
go build -i -o dist/go-to-protobuf ./vendor/k8s.io/code-generator/cmd/go-to-protobuf
go build -i -o dist/protoc-gen-gogo ./vendor/k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo

# Generate pkg/apis/<group>/<apiversion>/(generated.proto,generated.pb.go)
# NOTE: any dependencies of our types to the k8s.io apimachinery types should be added to the
# --apimachinery-packages= option so that go-to-protobuf can locate the types, but prefixed with a
# '-' so that go-to-protobuf will not generate .proto files for it.
PACKAGES=(
    github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1
)
APIMACHINERY_PKGS=(
    +k8s.io/apimachinery/pkg/util/intstr
    +k8s.io/apimachinery/pkg/api/resource
    +k8s.io/apimachinery/pkg/runtime/schema
    +k8s.io/apimachinery/pkg/runtime
    k8s.io/apimachinery/pkg/apis/meta/v1
    k8s.io/api/core/v1
)
go-to-protobuf \
    --logtostderr \
    --go-header-file=${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
    --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
    --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}") \
    --proto-import=./vendor

# protoc-gen-go or protoc-gen-gofast is used to build server/*/<service>.pb.go from .proto files
# NOTE: it is possible to use golang/protobuf or gogo/protobuf interchangeably
go build -i -o dist/protoc-gen-gofast ./vendor/github.com/gogo/protobuf/protoc-gen-gofast
#go build -i -o dist/protoc-gen-go ./vendor/github.com/golang/protobuf/protoc-gen-go

# protoc-gen-grpc-gateway is used to build <service>.pb.gw.go files from from .proto files
go build -i -o dist/protoc-gen-grpc-gateway ./vendor/github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway

# Generate server/<service>/(<service>.pb.go|<service>.pb.gw.go)
PROTO_FILES=$(find $PROJECT_ROOT \( -name "*.proto" -and -path '*/server/*' -or -path '*/reposerver/*' -and -name "*.proto" \))
for i in ${PROTO_FILES}; do

    # Path to the google API gateway annotations.proto will be different depending if we are
    # building natively (e.g. from workspace) vs. part of a docker build.
    if [ -f /.dockerenv ]; then
        GOOGLE_PROTO_API_PATH=/root/go/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis
        GOGO_PROTOBUF_PATH=/root/go/src/github.com/gogo/protobuf
    else
        GOOGLE_PROTO_API_PATH=${PROJECT_ROOT}/vendor/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis
        GOGO_PROTOBUF_PATH=${PROJECT_ROOT}/vendor/github.com/gogo/protobuf
    fi
    protoc \
        -I${PROJECT_ROOT} \
        -I/usr/local/include \
        -I./vendor \
        -I$GOPATH/src \
        -I${GOOGLE_PROTO_API_PATH} \
        -I${GOGO_PROTOBUF_PATH} \
        --go_out=plugins=grpc:$GOPATH/src \
        --grpc-gateway_out=logtostderr=true:$GOPATH/src \
        $i
done
