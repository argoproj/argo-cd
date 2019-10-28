// +build tools

package argocd

import (
	_ "github.com/gobuffalo/packr/packr"
	_ "k8s.io/code-generator/cmd/go-to-protobuf"
	_ "k8s.io/code-generator/cmd/go-to-protobuf/protoc-gen-gogo"
	_ "github.com/gogo/protobuf/protoc-gen-gogofast"
)
