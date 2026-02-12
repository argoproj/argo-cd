//go:build tools
// +build tools

package tools

import (
	// Standard protobuf code generators for google.golang.org/protobuf
	_ "google.golang.org/grpc/cmd/protoc-gen-go-grpc"
	_ "google.golang.org/protobuf/cmd/protoc-gen-go"

	// grpc-ecosystem/grpc-gateway/v2 is vendored because the generated *.pb.gw.go code imports it.
	// Also, we need the .proto files under googleapis
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway"
	_ "github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2"

	// k8s.io/code-generator is vendored to get generate-groups.sh, and k8s codegen utilities
	_ "k8s.io/code-generator"
	_ "k8s.io/code-generator/cmd/client-gen"
	_ "k8s.io/code-generator/cmd/deepcopy-gen"
	_ "k8s.io/code-generator/cmd/defaulter-gen"
	_ "k8s.io/code-generator/cmd/go-to-protobuf"
	_ "k8s.io/code-generator/cmd/informer-gen"
	_ "k8s.io/code-generator/cmd/lister-gen"

	// openapi-gen is vendored because upstream does not have tagged releases
	_ "k8s.io/kube-openapi/cmd/openapi-gen"
)
