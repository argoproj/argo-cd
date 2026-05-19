//go:build darwin && !cgo

// Package workloadidentity
// This file is used when the GOOS is darwin and CGO is not enabled.
// It provides a no-op implementation of the WorkloadIdentityTokenProvider to allow goreleaser to build
// a darwin binary on a linux machine.
package workloadidentity

import (
	"errors"
)

type WorkloadIdentityTokenProvider struct {
}

const CGOError = "CGO is not enabled, cannot use workload identity token provider"

// Code that does not require CGO
func NewWorkloadIdentityTokenProvider() TokenProvider {
	panic(CGOError)
}

func (c WorkloadIdentityTokenProvider) GetToken(scope string) (*Token, error) {
	return nil, errors.New(CGOError)
}
