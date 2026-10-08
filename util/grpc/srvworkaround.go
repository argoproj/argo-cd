package grpc

import (
	_ "unsafe"
)

// Workaround to fix https://github.com/argoproj/argo-cd/issues/29989
//
//go:linkname enableSRVLookups google.golang.org/grpc/internal/resolver/dns.EnableSRVLookups
var enableSRVLookups bool

func DisableSRVLookups() {
	if enableSRVLookups {
		enableSRVLookups = false
	} else {
		panic("GRPC SRV requests workaround failed: enableSRVLookups expected to be true")
	}
}
