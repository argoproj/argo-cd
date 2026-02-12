#! /usr/bin/env bash

# This script auto-generates protobuf related files.
# It is intended to be run manually when either API types are added/modified,
# or server gRPC calls are added. The generated files should then be checked into source control.
#
# This script uses:
# - protoc with LOCAL plugins for code generation (avoids network dependencies)
# - go-to-protobuf v0.35.1 for v1alpha1 CRD types (handles dual type definitions)
#
# All protoc plugins (protoc-gen-go, protoc-gen-go-grpc, protoc-gen-grpc-gateway, 
# protoc-gen-openapiv2) must be installed locally in dist/ before running.

set -x
set -o errexit
set -o nounset
set -o pipefail

# shellcheck disable=SC2128
PROJECT_ROOT=$(
    cd "$(dirname "${BASH_SOURCE}")"/..
    pwd
)
PATH="${PROJECT_ROOT}/dist:${PATH}"
GOPATH=$(go env GOPATH)
GOPATH_PROJECT_ROOT="${GOPATH}/src/github.com/argoproj/argo-cd"

# output tool versions
go version
protoc --version
swagger version
jq --version

export GO111MODULE=off

# Generate pkg/apis/<group>/<apiversion>/(generated.proto,generated.pb.go)
# NOTE: any dependencies of our types to the k8s.io apimachinery types should be added to the
# --apimachinery-packages= option so that go-to-protobuf can locate the types, but prefixed with a
# '-' so that go-to-protobuf will not generate .proto files for it.
PACKAGES=(
    github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1
)
APIMACHINERY_PKGS=(
    +k8s.io/apimachinery/pkg/util/intstr
    +k8s.io/apimachinery/pkg/api/resource
    +k8s.io/apimachinery/pkg/runtime/schema
    +k8s.io/apimachinery/pkg/runtime
    k8s.io/apimachinery/pkg/apis/meta/v1
    k8s.io/api/core/v1
    k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1
)

export GO111MODULE=on
[ -e ./v3 ] || ln -s . v3
[ -e "${GOPATH_PROJECT_ROOT}" ] || (mkdir -p "$(dirname "${GOPATH_PROJECT_ROOT}")" && ln -s "${PROJECT_ROOT}" "${GOPATH_PROJECT_ROOT}")

# protoc_include is the include directory containing the .proto files distributed with protoc binary
if [ -d /dist/protoc-include ]; then
    # containerized codegen build
    protoc_include=/dist/protoc-include
else
    # local codegen build
    protoc_include=${PROJECT_ROOT}/dist/protoc-include
fi

# go-to-protobuf expects dependency proto files to be in $GOPATH/src. Copy them there.
# Note: gogo/protobuf is no longer needed after migration to google.golang.org/protobuf
rm -rf "${GOPATH}/src/k8s.io/apimachinery" && mkdir -p "${GOPATH}/src/k8s.io" && cp -r "${PROJECT_ROOT}/vendor/k8s.io/apimachinery" "${GOPATH}/src/k8s.io"
rm -rf "${GOPATH}/src/k8s.io/api" && mkdir -p "${GOPATH}/src/k8s.io" && cp -r "${PROJECT_ROOT}/vendor/k8s.io/api" "${GOPATH}/src/k8s.io"
rm -rf "${GOPATH}/src/k8s.io/apiextensions-apiserver" && mkdir -p "${GOPATH}/src/k8s.io" && cp -r "${PROJECT_ROOT}/vendor/k8s.io/apiextensions-apiserver" "${GOPATH}/src/k8s.io"

go-to-protobuf \
    --go-header-file="${PROJECT_ROOT}"/hack/custom-boilerplate.go.txt \
    --packages="$(
        IFS=,
        echo "${PACKAGES[*]}"
    )" \
    --apimachinery-packages="$(
        IFS=,
        echo "${APIMACHINERY_PKGS[*]}"
    )" \
    --proto-import="${PROJECT_ROOT}"/vendor \
    --proto-import="${protoc_include}" \
    --output-dir="${GOPATH}/src/" \
    --only-idl

# go-to-protobuf modifies vendored code. Re-vendor code so it's available for subsequent steps.
go mod vendor

# Get gogo/protobuf path from module cache for proto imports
# gogo/protobuf is kept as a dependency for the .proto IDL files only
GOGO_PROTOBUF_VERSION=$(go list -m -f '{{.Version}}' github.com/gogo/protobuf)
GOGO_PROTOBUF_PATH="${GOPATH}/pkg/mod/github.com/gogo/protobuf@${GOGO_PROTOBUF_VERSION}"

# Create proper import structure for gogo/protobuf so protoc can find it
# The .proto files import "github.com/gogo/protobuf/gogoproto/gogo.proto"
# So we need to make that path resolvable
PROTO_IMPORTS_DIR="${PROJECT_ROOT}/dist/proto-imports"
rm -rf "${PROTO_IMPORTS_DIR}"
mkdir -p "${PROTO_IMPORTS_DIR}/github.com/gogo"
ln -sf "${GOGO_PROTOBUF_PATH}" "${PROTO_IMPORTS_DIR}/github.com/gogo/protobuf"

# Generate server/<service>/(<service>.pb.go|<service>.pb.gw.go)
# Using protoc with LOCAL plugins for compatibility with existing import structure
MOD_ROOT=${GOPATH}/pkg/mod
# Use official googleapis proto files from github.com/googleapis/googleapis
# Check if googleapis is in go.mod and get its version
googleapis_version=$(GO111MODULE=on go mod edit -json | jq -r '.Require[] | select(.Path == "github.com/googleapis/googleapis") | .Version')
if [ -z "$googleapis_version" ]; then
    # googleapis not in go.mod, add it with a pinned version
    # Use the same version that was last used in the project
    googleapis_version="v0.0.0-20260211014246-9eea40c74d97"
    GO111MODULE=on go get "github.com/googleapis/googleapis@${googleapis_version}"
    go mod vendor
else
    # googleapis exists in go.mod, use that version
    # Download if not already present in module cache
    if ! GO111MODULE=on go list -m "github.com/googleapis/googleapis@${googleapis_version}" &> /dev/null; then
        GO111MODULE=on go get "github.com/googleapis/googleapis@${googleapis_version}"
        go mod vendor
    fi
fi
GOOGLE_PROTO_API_PATH=${MOD_ROOT}/github.com/googleapis/googleapis@${googleapis_version}

# Ensure all required LOCAL plugins are available
for plugin in protoc-gen-go protoc-gen-go-grpc protoc-gen-grpc-gateway protoc-gen-openapiv2; do
    if ! command -v "$plugin" &> /dev/null; then
        echo "ERROR: $plugin not found in PATH"
        echo "Please ensure it's installed in dist/ directory"
        exit 1
    fi
done

PROTO_FILES=$(find "$PROJECT_ROOT" \( -name "*.proto" -and -path '*/server/*' -or -path '*/reposerver/*' -and -name "*.proto" -or -path '*/cmpserver/*' -and -name "*.proto" -or -path '*/commitserver/*' -and -name "*.proto" -or -path '*/util/askpass/*' -and -name "*.proto" \) | sort)
for i in ${PROTO_FILES}; do
    protoc \
        -I"${PROJECT_ROOT}" \
        -I"${protoc_include}" \
        -I./vendor \
        -I"$GOPATH"/src \
        -I"${GOOGLE_PROTO_API_PATH}" \
        -I"${PROTO_IMPORTS_DIR}" \
        --go_out="$GOPATH"/src \
        --go_opt=paths=source_relative \
        --go-grpc_out="$GOPATH"/src \
        --go-grpc_opt=paths=source_relative,require_unimplemented_servers=false \
        --grpc-gateway_out=logtostderr=true:"$GOPATH"/src \
        --openapiv2_out=logtostderr=true:. \
        "$i"
done

# Copy generated files from GOPATH/src back to project root
# The files are generated with paths=source_relative, so they're at $GOPATH/src/<relative-path>
# We need to copy them to the project root
rsync -av "$GOPATH"/src/server/ "$PROJECT_ROOT"/server/
rsync -av "$GOPATH"/src/reposerver/ "$PROJECT_ROOT"/reposerver/
rsync -av "$GOPATH"/src/cmpserver/ "$PROJECT_ROOT"/cmpserver/
rsync -av "$GOPATH"/src/commitserver/ "$PROJECT_ROOT"/commitserver/
rsync -av "$GOPATH"/src/util/ "$PROJECT_ROOT"/util/

# NOTE: We do NOT compile pkg/apis/application/v1alpha1/generated.proto with standard protoc
# because it would redefine the types that are already defined in the manual Go files.
# The Kubernetes CRD types require gogo/protobuf's special protoc-gen-gogo which can generate
# protobuf methods without redefining types. For now, we keep the old generated.pb.go for v1alpha1.
# The grpc-gateway generated code will use the gogo-based types from v1alpha1.

# This file is generated but should not be checked in.
rm util/askpass/askpass.swagger.json

[ -L "${GOPATH_PROJECT_ROOT}" ] && rm -rf "${GOPATH_PROJECT_ROOT}"
[ -L ./v3 ] && rm -rf v3

# collect_swagger gathers swagger files into a subdirectory
collect_swagger() {
    SWAGGER_ROOT="$1"
    SWAGGER_OUT="${PROJECT_ROOT}/assets/swagger.json"
    PRIMARY_SWAGGER=$(mktemp)
    COMBINED_SWAGGER=$(mktemp)

    cat <<EOF >"${PRIMARY_SWAGGER}"
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

    find "${SWAGGER_ROOT}" -name '*.swagger.json' -exec swagger mixin --ignore-conflicts "${PRIMARY_SWAGGER}" '{}' \+ >"${COMBINED_SWAGGER}"
    jq -r 'del(.definitions[].properties[]? | select(."$ref"!=null and .description!=null).description) | del(.definitions[].properties[]? | select(."$ref"!=null and .title!=null).title) |
      # The "array" and "map" fields have custom unmarshaling. Modify the swagger to reflect this.
      .definitions.v1alpha1ApplicationSourcePluginParameter.properties.array = {"description":"Array is the value of an array type parameter.","type":"array","items":{"type":"string"}} |
      del(.definitions.v1alpha1OptionalArray) |
      .definitions.v1alpha1ApplicationSourcePluginParameter.properties.map = {"description":"Map is the value of a map type parameter.","type":"object","additionalProperties":{"type":"string"}} |
      del(.definitions.v1alpha1OptionalMap) |
      # Output for int64 is incorrect, because it is based on proto definitions, where int64 is a string. In our JSON API, we expect int64 to be an integer. https://github.com/grpc-ecosystem/grpc-gateway/issues/219
      (.definitions[]?.properties[]? | select(.type == "string" and .format == "int64")) |= (.type = "integer")
    ' "${COMBINED_SWAGGER}" |
        jq '.definitions.v1Time.type = "string" | .definitions.v1Time.format = "date-time" | del(.definitions.v1Time.properties)' |
        jq '.definitions.v1alpha1ResourceNode.allOf = [{"$ref": "#/definitions/v1alpha1ResourceRef"}] | del(.definitions.v1alpha1ResourceNode.properties.resourceRef) ' |
        jq '
          # Clean Kubernetes code generation markers from descriptions and titles
          # Remove lines starting with + (e.g., +optional, +genclient, +kubebuilder:resource, etc.)
          walk(
            if type == "object" then
              if .description and (.description | type) == "string" then
                .description |= (split("\n") | map(select(startswith("+") | not)) | join("\n") | sub("\\n+$"; ""))
              else . end |
              if .title and (.title | type) == "string" then
                .title |= (split("\n") | map(select(startswith("+") | not)) | join("\n") | sub("\\n+$"; ""))
              else . end
            else . end
          )
        ' \
            >"${SWAGGER_OUT}"

    /bin/rm "${PRIMARY_SWAGGER}" "${COMBINED_SWAGGER}"
}

# clean up generated swagger files (should come after collect_swagger)
clean_swagger() {
    SWAGGER_ROOT="$1"
    find "${SWAGGER_ROOT}" -name '*.swagger.json' -delete
}

collect_swagger server
clean_swagger server
clean_swagger reposerver
clean_swagger controller
clean_swagger cmpserver
clean_swagger commitserver
