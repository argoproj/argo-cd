#! /usr/bin/env bash

# This script auto-generates protobuf related files. It is intended to be run manually when either
# API types are added/modified, or server gRPC calls are added. The generated files should then
# be checked into source control.

set -x
set -o errexit
set -o nounset
set -o pipefail

# output tool versions
protoc --version
swagger version
jq --version

# shellcheck disable=SC2034
GO111MODULE=on
go get k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo@v0.0.0-20191003035328-700b1226c0bd

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/..; pwd)
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${PROJECT_ROOT}; ls -d -1 $GOPATH/pkg/mod/k8s.io/code-generator@v0.0.0-20191003035328-700b1226c0bd 2>/dev/null || echo ../code-generator)}

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
    --go-header-file=${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
    --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
    --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}")

# Either protoc-gen-go, protoc-gen-gofast, or protoc-gen-gogofast can be used to build
# server/*/<service>.pb.go from .proto files. golang/protobuf and gogo/protobuf can be used
# interchangeably. The difference in the options are:
# 1. protoc-gen-go - official golang/protobuf
#go build -i -o protoc-gen-go ./vendor/github.com/golang/protobuf/protoc-gen-go
#GOPROTOBINARY=go
# 2. protoc-gen-gofast - fork of golang golang/protobuf. Faster code generation
#go build -i -o protoc-gen-gofast ./vendor/github.com/gogo/protobuf/protoc-gen-gofast
#GOPROTOBINARY=gofast
# 3. protoc-gen-gogofast - faster code generation and gogo extensions and flexibility in controlling
# the generated go code (e.g. customizing field names, nullable fields)
go get github.com/gogo/protobuf/protoc-gen-gogofast@v1.3.0
GOPROTOBINARY=gogofast

# protoc-gen-grpc-gateway is used to build <service>.pb.gw.go files from from .proto files
go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway@v1.11.3
# protoc-gen-swagger is used to build swagger.json
go get github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger@v1.11.3

# Generate server/<service>/(<service>.pb.go|<service>.pb.gw.go)
PROTO_FILES=$(find $PROJECT_ROOT \( -name "*.proto" -and -path '*/server/*' -or -path '*/reposerver/*' -and -name "*.proto" \) | sort)
for i in ${PROTO_FILES}; do
    protoc \
        -I${PROJECT_ROOT} \
        -I/usr/local/include \
        -I$GOPATH/src \
        -I$GOPATH/pkg/mod/github.com/grpc-ecosystem/grpc-gateway@v1.11.3/third_party/googleapis \
        -I$GOPATH/pkg/mod/github.com/gogo/protobuf@v1.3.0 \
        --${GOPROTOBINARY}_out=plugins=grpc:$GOPATH/src \
        --grpc-gateway_out=logtostderr=true:$GOPATH/src \
        --swagger_out=logtostderr=true:. \
        $i
done

# collect_swagger gathers swagger files into a subdirectory
collect_swagger() {
    SWAGGER_ROOT="$1"
    EXPECTED_COLLISIONS="$2"
    SWAGGER_OUT="${PROJECT_ROOT}/assets/swagger.json"
    PRIMARY_SWAGGER=`mktemp`
    COMBINED_SWAGGER=`mktemp`

    cat <<EOF > "${PRIMARY_SWAGGER}"
{
  "swagger": "2.0",
  "info": {
    "title": "Consolidate Services",
    "description": "Description of all APIs",
    "version": "version not set"
  },
  "paths": {}
}
EOF

    /bin/rm -f "${SWAGGER_OUT}"

    /usr/bin/find "${SWAGGER_ROOT}" -name '*.swagger.json' -exec /usr/local/bin/swagger mixin -c "${EXPECTED_COLLISIONS}" "${PRIMARY_SWAGGER}" '{}' \+ > "${COMBINED_SWAGGER}"
    /usr/local/bin/jq -r 'del(.definitions[].properties[]? | select(."$ref"!=null and .description!=null).description) | del(.definitions[].properties[]? | select(."$ref"!=null and .title!=null).title)' "${COMBINED_SWAGGER}" > "${SWAGGER_OUT}"

    /bin/rm "${PRIMARY_SWAGGER}" "${COMBINED_SWAGGER}"
}

# clean up generated swagger files (should come after collect_swagger)
clean_swagger() {
    SWAGGER_ROOT="$1"
    /usr/bin/find "${SWAGGER_ROOT}" -name '*.swagger.json' -delete
}

collect_swagger server 35
clean_swagger server
clean_swagger reposerver
clean_swagger controller
