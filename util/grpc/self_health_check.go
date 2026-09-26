package grpc

import (
	"net"
	"strconv"
)

// SelfHealthCheckTarget returns how the process should dial its own gRPC port for
// /healthz?full=true. It uses the passthrough resolver so the endpoint bypasses
// gRPC's dns resolver, which issues _grpclb SRV queries when grpclb is linked.
// Unspecified listen addresses are rewritten to loopback (127.0.0.1 or ::1) so
// the probe does not dial a wildcard.
func SelfHealthCheckTarget(listenHost string, port int) string {
	return "passthrough:" + net.JoinHostPort(selfHealthCheckHost(listenHost), strconv.Itoa(port))
}

// selfHealthCheckHost returns a bare host for net.JoinHostPort. Bracketed IPv6
// literals (e.g. "[::]") are unwrapped. Empty or unspecified addresses
// (0.0.0.0, ::) become the matching loopback (127.0.0.1 or ::1), since a client
// cannot dial a wildcard. Any other host is returned unchanged.
func selfHealthCheckHost(listenHost string) string {
	host := listenHost
	if len(host) >= 2 && host[0] == '[' && host[len(host)-1] == ']' {
		host = host[1 : len(host)-1]
	}
	if host == "" {
		return "127.0.0.1"
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsUnspecified() {
		return host
	}
	if ip.To4() != nil {
		return "127.0.0.1"
	}
	return "::1"
}
