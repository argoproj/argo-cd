#!/bin/bash
set -eux -o pipefail

SRCROOT="$( CDPATH='' cd -- "$(dirname "$0")/../.." && pwd -P )"
DIST="${SRCROOT}/dist"

# This script builds all our golang-based codegen utility CLIs into argo-cd/dist, which is added to
# the PATH during codegen. Versions are never spelled out here; they come from module files:
#
# 1. Tools which must agree with the libraries they generate against (k8s codegen, protobuf,
#    grpc-gateway) are `tool` directives in the root go.mod. They therefore track the exact versions
#    the rest of the code is built with, and `replace` directives are respected.
# 2. Tools with no such coupling are `tool` directives in hack/tools/go.mod, an isolated module, so
#    their dependencies stay out of the argo-cd build graph.
#
# Because the binaries are rebuilt from those module files on every run, they cannot go stale.

mkdir -p "${DIST}"

go build -C "${SRCROOT}" -mod=mod -o "${DIST}/" \
    github.com/gogo/protobuf/protoc-gen-gogofast \
    github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway \
    github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger \
    golang.org/x/tools/cmd/goimports \
    k8s.io/code-generator/cmd/applyconfiguration-gen \
    k8s.io/code-generator/cmd/client-gen \
    k8s.io/code-generator/cmd/conversion-gen \
    k8s.io/code-generator/cmd/deepcopy-gen \
    k8s.io/code-generator/cmd/defaulter-gen \
    k8s.io/code-generator/cmd/go-to-protobuf \
    k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo \
    k8s.io/code-generator/cmd/informer-gen \
    k8s.io/code-generator/cmd/lister-gen \
    k8s.io/code-generator/cmd/validation-gen \
    k8s.io/kube-openapi/cmd/openapi-gen

go build -C "${SRCROOT}/hack/tools" -mod=mod -o "${DIST}/" \
    github.com/go-swagger/go-swagger/cmd/swagger \
    github.com/vektra/mockery/v3 \
    sigs.k8s.io/controller-tools/cmd/controller-gen
