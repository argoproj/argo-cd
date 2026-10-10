package grpc

import (
	_ "unsafe"
)

// A workaround to disable DNS SRV _grpclb._tcp.<HOSTNAME> requests
// when opening GRPC connections, see
// https://github.com/argoproj/argo-cd/issues/29989 and
// https://github.com/argoproj/argo-cd/issues/29857
//
// This will make an alias for the flag in the internal library
// package that is set in init() and we cannot access it directly.
// Must be called before opening any GRPC connections.
//
//go:linkname enableSRVLookups google.golang.org/grpc/internal/resolver/dns.EnableSRVLookups
var enableSRVLookups bool

func DisableSRVLookups() {
	if !enableSRVLookups {
		// to know that something is wrong with the compiler toolchain or that
		// the after some upstream change the workaround no longer works
		// (the flag was renamed, removed or no longer set to true)
		panic("The GRPC SRV requests workaround failed: enableSRVLookups expected to be true, but is false")
	}
	enableSRVLookups = false
}
