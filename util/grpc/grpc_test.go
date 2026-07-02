package grpc

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
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

func TestBlockingNewClient(t *testing.T) {
	clearProxyEnv(t)

	t.Run("dial failure returns error", func(t *testing.T) {
		socketPath := filepath.Join(t.TempDir(), "nonexistent.sock")

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
		defer cancel()

		conn, err := BlockingNewClient(ctx, "unix", socketPath, nil)

		require.Error(t, err)
		assert.Nil(t, conn)
		assert.Contains(t, err.Error(), "TRANSIENT_FAILURE")
	})

	t.Run("success", func(t *testing.T) {
		socketPath := filepath.Join(t.TempDir(), "test.sock")

		ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
		require.NoError(t, err)

		srv := grpc.NewServer()
		go func() { _ = srv.Serve(ln) }()
		t.Cleanup(srv.GracefulStop)

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		conn, err := BlockingNewClient(ctx, "unix", socketPath, nil)

		require.NoError(t, err)
		require.NotNil(t, conn)
		t.Cleanup(func() { _ = conn.Close() })
		assert.Equal(t, connectivity.Ready, conn.GetState())
	})

	t.Run("reconnects after server restart", func(t *testing.T) {
		socketPath := filepath.Join(t.TempDir(), "test.sock")
		listen := func() *grpc.Server {
			ln, err := (&net.ListenConfig{}).Listen(context.Background(), "unix", socketPath)
			require.NoError(t, err)
			srv := grpc.NewServer()
			go func() { _ = srv.Serve(ln) }()
			return srv
		}

		srv1 := listen()

		// Short reconnect backoff so the test doesn't wait on gRPC's default
		// exponential schedule (starts at 1s).
		fastBackoff := grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  50 * time.Millisecond,
				Multiplier: 1.0,
				MaxDelay:   200 * time.Millisecond,
			},
			MinConnectTimeout: time.Second,
		})

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()

		conn, err := BlockingNewClient(ctx, "unix", socketPath, nil, fastBackoff)
		require.NoError(t, err)
		t.Cleanup(func() { _ = conn.Close() })
		require.Equal(t, connectivity.Ready, conn.GetState())

		// Kill the server and verify the channel leaves Ready
		srv1.Stop()
		require.Eventually(t, func() bool {
			return conn.GetState() != connectivity.Ready
		}, 5*time.Second, 20*time.Millisecond, "channel should leave Ready after server stop")

		// Bring the server back on the same socket.
		srv2 := listen()
		t.Cleanup(srv2.GracefulStop)

		// Nudge gRPC out of IDLE; with grpc.NewClient the channel is lazy and
		// won't try to reconnect on its own until an RPC happens.
		conn.Connect()
		require.Eventually(t, func() bool {
			return conn.GetState() == connectivity.Ready
		}, 5*time.Second, 50*time.Millisecond, "channel should reconnect after server restart")
	})
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
