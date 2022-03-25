#!/bin/bash
set -eux -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/../.." && pwd -P )"

# This script installs all our golang-based codegen utility CLIs necessary for codegen.
# Some dependencies are vendored in go.mod (ones which are actually imported in our codebase).
# Other dependencies are only used as a CLI and do not need vendoring in go.mod (doing so adds
# unecessary dependencies to go.mod). We want to maintain a single source of truth for versioning
# our binaries (either go.mod or go install <pkg>@<version>), so we use two techniques to install
# our CLIs:
# 1. For CLIs which are NOT vendored in go.mod, we can run `go install <pkg>@<version>` with an explicit version
# 2. For packages which we *do* vendor in go.mod, we determine version from go.mod followed by `go install` with that version
go_mod_install() {
    module=$(go list -f '{{.Module}}' $1 | awk '{print $1}')
    module_version=$(go list -m $module | awk '{print $NF}' | head -1)
    go install $1@$module_version
}

# All binaries are compiled into the argo-cd/dist directory, which is added to the PATH during codegen
export GOBIN="${SRCROOT}/dist"
mkdir -p $GOBIN

# protoc-gen-go* is used to generate <service>.pb.go from .proto files
#go_mod_install github.com/golang/protobuf/protoc-gen-go
#go_mod_install github.com/gogo/protobuf/protoc-gen-gogo
go_mod_install github.com/gogo/protobuf/protoc-gen-gogofast

# protoc-gen-grpc-gateway is used to build <service>.pb.gw.go files from from .proto files
go_mod_install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway

# # protoc-gen-swagger is used to build swagger.json
go_mod_install github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger

# k8s tools to codegen .proto files, client libraries, and helpers from types.go
go_mod_install k8s.io/code-generator/cmd/go-to-protobuf
go_mod_install k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo
go_mod_install k8s.io/code-generator/cmd/client-gen
go_mod_install k8s.io/code-generator/cmd/deepcopy-gen
go_mod_install k8s.io/code-generator/cmd/defaulter-gen
go_mod_install k8s.io/code-generator/cmd/informer-gen
go_mod_install k8s.io/code-generator/cmd/lister-gen

# We still install openapi-gen from go.mod since upstream does not utilize release tags
go_mod_install k8s.io/kube-openapi/cmd/openapi-gen

# controller-gen is run by ./hack/gen-crd-spec to generate the CRDs
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.1

# swagger cli is used to generate swagger docs
go install github.com/go-swagger/go-swagger/cmd/swagger@v0.28.0

# goimports is used to auto-format generated code
go install golang.org/x/tools/cmd/goimports@v0.1.8
