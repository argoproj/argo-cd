package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	grpcgo "google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
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

func TestBlockingNewClient_CancelledContextAbortsDial(t *testing.T) {
	clearProxyEnv(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := BlockingNewClient(ctx, "tcp", "10.255.255.1:443", nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestBlockingNewClientReconnectsAfterIdle(t *testing.T) {
	clearProxyEnv(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	server := grpcgo.NewServer()
	healthpb.RegisterHealthServer(server, health.NewServer())

	go func() {
		_ = server.Serve(listener)
	}()
	t.Cleanup(server.Stop)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	conn, err := BlockingNewClient(
		ctx,
		"tcp",
		listener.Addr().String(),
		nil,
		// Default is 30 minutes
		grpcgo.WithIdleTimeout(200*time.Millisecond),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = conn.Close()
	})

	client := healthpb.NewHealthClient(conn)

	// The initial transport works.
	_, err = client.Check(ctx, &healthpb.HealthCheckRequest{})
	require.NoError(t, err)

	// Wait for idle.
	require.Eventually(t, func() bool {
		return conn.GetState() == connectivity.Idle
	}, 2*time.Second, 10*time.Millisecond)

	// This must create a new transport using the same ClientConn.
	rpcCtx, rpcCancel := context.WithTimeout(
		t.Context(),
		2*time.Second,
	)
	defer rpcCancel()

	_, err = client.Check(
		rpcCtx,
		&healthpb.HealthCheckRequest{},
		grpcgo.WaitForReady(true),
	)
	require.NoError(t, err)
}
