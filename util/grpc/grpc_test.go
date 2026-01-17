package grpc

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

var proxyEnvKeys = []string{"ALL_PROXY", "HTTP_PROXY", "HTTPS_PROXY", "NO_PROXY"}

func clearProxyEnv(t *testing.T) {
	t.Helper()
	for _, k := range proxyEnvKeys {
		t.Setenv(k, "")
	}
}

func applyProxyEnv(t *testing.T, envs map[string]string) {
	t.Helper()
	for k, v := range envs {
		t.Setenv(k, v)
	}
}

func TestBlockingDial_ProxyEnvironmentHandling(t *testing.T) {
	tests := []struct {
		name        string
		proxyEnv    map[string]string
		address     string
		expectError bool
	}{
		{
			name:        "No proxy environment variables",
			proxyEnv:    map[string]string{},
			address:     "127.0.0.1:8080",
			expectError: true,
		},
		{
			name: "ALL_PROXY environment variable set",
			proxyEnv: map[string]string{
				"ALL_PROXY": "http://proxy.example.com:8080",
			},
			address:     "remote.example.com:443",
			expectError: true,
		},
		{
			name: "HTTP_PROXY environment variable set",
			proxyEnv: map[string]string{
				"HTTP_PROXY": "http://proxy.example.com:3128",
			},
			address:     "api.example.com:80",
			expectError: true,
		},
		{
			name: "HTTPS_PROXY environment variable set",
			proxyEnv: map[string]string{
				"HTTPS_PROXY": "https://secure-proxy.example.com:8080",
			},
			address:     "secure.example.com:443",
			expectError: true,
		},
		{
			name: "NO_PROXY bypass configuration",
			proxyEnv: map[string]string{
				"ALL_PROXY": "http://proxy.example.com:8080",
				"NO_PROXY":  "localhost,127.0.0.1,*.local",
			},
			address:     "127.0.0.1:8080",
			expectError: true,
		},
		{
			name: "Multiple proxy environment variables",
			proxyEnv: map[string]string{
				"ALL_PROXY":   "socks5://all-proxy.example.com:1080",
				"HTTP_PROXY":  "http://http-proxy.example.com:8080",
				"HTTPS_PROXY": "https://https-proxy.example.com:8080",
				"NO_PROXY":    "localhost,*.local",
			},
			address:     "external.example.com:443",
			expectError: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			clearProxyEnv(t)
			applyProxyEnv(t, tt.proxyEnv)

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			conn, err := BlockingNewClient(ctx, "tcp", tt.address, nil)

			if tt.expectError {
				require.Error(t, err)
				assert.Nil(t, conn)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, conn)
				require.NoError(t, conn.Close())
			}
		})
	}
}

func TestClientAddrFromContext(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		expected string
	}{
		{
			name: "Context with peer info",
			setupCtx: func() context.Context {
				addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:54321")
				p := &peer.Peer{Addr: addr}
				return peer.NewContext(context.Background(), p)
			},
			expected: "192.168.1.100:54321",
		},
		{
			name:     "Context without peer info",
			setupCtx: context.Background,
			expected: "unknown",
		},
		{
			name: "Context with nil peer address",
			setupCtx: func() context.Context {
				p := &peer.Peer{Addr: nil}
				return peer.NewContext(context.Background(), p)
			},
			expected: "unknown",
		},
		{
			name: "Context with IPv6 address",
			setupCtx: func() context.Context {
				addr, _ := net.ResolveTCPAddr("tcp", "[::1]:8080")
				p := &peer.Peer{Addr: addr}
				return peer.NewContext(context.Background(), p)
			},
			expected: "[::1]:8080",
		},
		{
			name: "Context with metadata (HTTP via grpc-gateway)",
			setupCtx: func() context.Context {
				md := metadata.Pairs(MetadataKeyClientAddr, "10.0.0.1:12345")
				return metadata.NewIncomingContext(context.Background(), md)
			},
			expected: "10.0.0.1:12345",
		},
		{
			name: "Context with both peer and metadata (peer takes precedence)",
			setupCtx: func() context.Context {
				addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:54321")
				p := &peer.Peer{Addr: addr}
				ctx := peer.NewContext(context.Background(), p)
				md := metadata.Pairs(MetadataKeyClientAddr, "10.0.0.1:12345")
				return metadata.NewIncomingContext(ctx, md)
			},
			expected: "192.168.1.100:54321",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := ClientAddrFromContext(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClientIPFromContext(t *testing.T) {
	tests := []struct {
		name     string
		setupCtx func() context.Context
		expected string
	}{
		{
			name: "Context with IPv4 peer info",
			setupCtx: func() context.Context {
				addr, _ := net.ResolveTCPAddr("tcp", "192.168.1.100:54321")
				p := &peer.Peer{Addr: addr}
				return peer.NewContext(context.Background(), p)
			},
			expected: "192.168.1.100",
		},
		{
			name:     "Context without peer info",
			setupCtx: context.Background,
			expected: "unknown",
		},
		{
			name: "Context with nil peer address",
			setupCtx: func() context.Context {
				p := &peer.Peer{Addr: nil}
				return peer.NewContext(context.Background(), p)
			},
			expected: "unknown",
		},
		{
			name: "Context with IPv6 address",
			setupCtx: func() context.Context {
				addr, _ := net.ResolveTCPAddr("tcp", "[::1]:8080")
				p := &peer.Peer{Addr: addr}
				return peer.NewContext(context.Background(), p)
			},
			expected: "::1",
		},
		{
			name: "Context with localhost",
			setupCtx: func() context.Context {
				addr, _ := net.ResolveTCPAddr("tcp", "127.0.0.1:12345")
				p := &peer.Peer{Addr: addr}
				return peer.NewContext(context.Background(), p)
			},
			expected: "127.0.0.1",
		},
		{
			name: "Context with metadata (HTTP via grpc-gateway)",
			setupCtx: func() context.Context {
				md := metadata.Pairs(MetadataKeyClientAddr, "10.0.0.1:12345")
				return metadata.NewIncomingContext(context.Background(), md)
			},
			expected: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := ClientIPFromContext(ctx)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestClientIPFromHTTPRequest(t *testing.T) {
	tests := []struct {
		name     string
		setupReq func() *http.Request
		expected string
	}{
		{
			name: "X-Forwarded-For header with single IP",
			setupReq: func() *http.Request {
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Forwarded-For", "203.0.113.195")
				req.RemoteAddr = "10.0.0.1:12345"
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Forwarded-For header with multiple IPs",
			setupReq: func() *http.Request {
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Forwarded-For", "203.0.113.195, 70.41.3.18, 150.172.238.178")
				req.RemoteAddr = "10.0.0.1:12345"
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Real-IP header",
			setupReq: func() *http.Request {
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Real-IP", "203.0.113.195")
				req.RemoteAddr = "10.0.0.1:12345"
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "X-Forwarded-For takes precedence over X-Real-IP",
			setupReq: func() *http.Request {
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Forwarded-For", "203.0.113.195")
				req.Header.Set("X-Real-IP", "70.41.3.18")
				req.RemoteAddr = "10.0.0.1:12345"
				return req
			},
			expected: "203.0.113.195",
		},
		{
			name: "Fallback to RemoteAddr",
			setupReq: func() *http.Request {
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
				req.RemoteAddr = "10.0.0.1:12345"
				return req
			},
			expected: "10.0.0.1:12345",
		},
		{
			name: "X-Forwarded-For with spaces",
			setupReq: func() *http.Request {
				req, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody)
				req.Header.Set("X-Forwarded-For", "  203.0.113.195  ")
				req.RemoteAddr = "10.0.0.1:12345"
				return req
			},
			expected: "203.0.113.195",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := tt.setupReq()
			result := ClientIPFromHTTPRequest(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}
