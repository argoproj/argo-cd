package grpc

import (
	"net"
	"strconv"
)

// SelfHealthCheckTarget returns how the process should dial its own gRPC port for
// /healthz?full=true. Unspecified IP addresses are replaced with loopback
// addresses (127.0.0.1 or ::1) to avoid dialing a wildcard address. The
// passthrough resolver is used to avoid service-name-based DNS resolution.
func SelfHealthCheckTarget(listenHost string, port int) string {
	return "passthrough:" + net.JoinHostPort(selfHealthCheckHost(listenHost), strconv.Itoa(port))
}

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
