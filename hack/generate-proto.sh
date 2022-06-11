#! /usr/bin/env bash

# This script auto-generates protobuf related files. It is intended to be run manually when either
# API types are added/modified, or server gRPC calls are added. The generated files should then
# be checked into source control.

set -x
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT=$(cd $(dirname ${BASH_SOURCE})/..; pwd)
PATH="${PROJECT_ROOT}/dist:${PATH}"

# output tool versions
protoc --version
swagger version
jq --version

export GO111MODULE=off

# Generate pkg/apis/<group>/<apiversion>/(generated.proto,generated.pb.go)
# NOTE: any dependencies of our types to the k8s.io apimachinery types should be added to the
# --apimachinery-packages= option so that go-to-protobuf can locate the types, but prefixed with a
# '-' so that go-to-protobuf will not generate .proto files for it.
PACKAGES=(
    github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1
)
APIMACHINERY_PKGS=(
    +k8s.io/apimachinery/pkg/util/intstr
    +k8s.io/apimachinery/pkg/api/resource
    +k8s.io/apimachinery/pkg/runtime/schema
    +k8s.io/apimachinery/pkg/runtime
    k8s.io/apimachinery/pkg/apis/meta/v1
    k8s.io/api/core/v1
)

export GO111MODULE=on
[ -e ./v2 ] || ln -s . v2

# protoc_include is the include directory containing the .proto files distributed with protoc binary
if [ -d /dist/protoc-include ]; then
    # containerized codegen build
    protoc_include=/dist/protoc-include
else
    # local codegen build 
    protoc_include=${PROJECT_ROOT}/dist/protoc-include
fi

go-to-protobuf \
    --go-header-file=${PROJECT_ROOT}/hack/custom-boilerplate.go.txt \
    --packages=$(IFS=, ; echo "${PACKAGES[*]}") \
    --apimachinery-packages=$(IFS=, ; echo "${APIMACHINERY_PKGS[*]}") \
    --proto-import=./vendor \
    --proto-import=${protoc_include}

# Either protoc-gen-go, protoc-gen-gofast, or protoc-gen-gogofast can be used to build
# server/*/<service>.pb.go from .proto files. golang/protobuf and gogo/protobuf can be used
# interchangeably. The difference in the options are:
# 1. protoc-gen-go - official golang/protobuf
#GOPROTOBINARY=go
# 2. protoc-gen-gofast - fork of golang golang/protobuf. Faster code generation
#GOPROTOBINARY=gofast
# 3. protoc-gen-gogofast - faster code generation and gogo extensions and flexibility in controlling
# the generated go code (e.g. customizing field names, nullable fields)
GOPROTOBINARY=gogofast

# Generate server/<service>/(<service>.pb.go|<service>.pb.gw.go)
MOD_ROOT=${GOPATH}/pkg/mod
grpc_gateway_version=$(go list -m github.com/grpc-ecosystem/grpc-gateway | awk '{print $NF}' | head -1)
GOOGLE_PROTO_API_PATH=${MOD_ROOT}/github.com/grpc-ecosystem/grpc-gateway@${grpc_gateway_version}/third_party/googleapis
GOGO_PROTOBUF_PATH=${PROJECT_ROOT}/vendor/github.com/gogo/protobuf
PROTO_FILES=$(find $PROJECT_ROOT \( -name "*.proto" -and -path '*/server/*' -or -path '*/reposerver/*' -and -name "*.proto" -or -path '*/cmpserver/*' -and -name "*.proto" \) | sort)
for i in ${PROTO_FILES}; do
    protoc \
        -I${PROJECT_ROOT} \
        -I${protoc_include} \
        -I./vendor \
        -I$GOPATH/src \
        -I${GOOGLE_PROTO_API_PATH} \
        -I${GOGO_PROTOBUF_PATH} \
        --${GOPROTOBINARY}_out=plugins=grpc:$GOPATH/src \
        --grpc-gateway_out=logtostderr=true:$GOPATH/src \
        --swagger_out=logtostderr=true:. \
        $i
done
[ -e ./v2 ] && rm -rf v2

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

    rm -f "${SWAGGER_OUT}"

    find "${SWAGGER_ROOT}" -name '*.swagger.json' -exec swagger mixin -c "${EXPECTED_COLLISIONS}" "${PRIMARY_SWAGGER}" '{}' \+ > "${COMBINED_SWAGGER}"
    jq -r 'del(.definitions[].properties[]? | select(."$ref"!=null and .description!=null).description) | del(.definitions[].properties[]? | select(."$ref"!=null and .title!=null).title)' "${COMBINED_SWAGGER}" > "${SWAGGER_OUT}"

    /bin/rm "${PRIMARY_SWAGGER}" "${COMBINED_SWAGGER}"
}

# clean up generated swagger files (should come after collect_swagger)
clean_swagger() {
    SWAGGER_ROOT="$1"
    find "${SWAGGER_ROOT}" -name '*.swagger.json' -delete
}

echo "If additional types are added, the number of expected collisions may need to be increased"
EXPECTED_COLLISION_COUNT=62
collect_swagger server ${EXPECTED_COLLISION_COUNT}
clean_swagger server
clean_swagger reposerver
clean_swagger controller
clean_swagger cmpserver
