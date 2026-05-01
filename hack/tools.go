//go:build tools
// +build tools

package tools

import (
	// grpc-ecosystem/grpc-gateway/v2 is vendored because the generated *.pb.gw.go code imports it.
	// Also, we need the .proto files under grpc-gateway/v2/third_party/googleapis
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"

	// google.golang.org/protobuf/cmd/protoc-gen-go generates *.pb.go from .proto files
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"

	// google.golang.org/grpc/cmd/protoc-gen-go-grpc generates gRPC service stubs
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"

	// k8s.io/code-generator is vendored to get generate-groups.sh, and k8s codegen utilities
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/defaulter-gen"
	// go-to-protobuf is invoked with --only-idl to generate .proto IDL files from Go types.
	// It no longer invokes protoc-gen-gogo; the resulting .proto is then compiled with protoc-gen-go.
	_ "k8s.io/code-generator/cmd/go-to-protobuf"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"

	// openapi-gen is vendored because upstream does not have tagged releases
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
