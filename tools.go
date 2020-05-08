// +build tools

// Package tools contains code generation and build utilities
// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	_ "github.com/jstemmer/go-junit-report"
	_ "github.com/vektra/mockery"
	_ "k8s.io/code-generator/cmd/client-gen"
)
