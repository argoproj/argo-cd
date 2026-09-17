package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
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

type trackingListener struct {
	net.Listener
	accepted chan net.Conn
}

func (l *trackingListener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err == nil {
		l.accepted <- conn
	}
	return conn, err
}

func startHealthServer(t *testing.T) *trackingListener {
	t.Helper()

	lis, err := new(net.ListenConfig).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	trackingLis := &trackingListener{
		Listener: lis,
		accepted: make(chan net.Conn, 2),
	}

	srv := grpc.NewServer()
	hsvc := health.NewServer()
	hsvc.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(srv, hsvc)

	go func() {
		_ = srv.Serve(trackingLis)
	}()

	t.Cleanup(func() {
		srv.Stop()
		_ = lis.Close()
	})

	return trackingLis
}

func TestBlockingNewClientReconnectsAfterConnectionClose(t *testing.T) {
	clearProxyEnv(t)
	lis := startHealthServer(t)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	conn, err := BlockingNewClient(ctx, "tcp", lis.Addr().String(), nil)
	require.NoError(t, err)
	defer conn.Close()

	healthClient := grpc_health_v1.NewHealthClient(conn)

	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)

	var firstConn net.Conn
	select {
	case firstConn = <-lis.accepted:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for initial connection")
	}

	require.NoError(t, firstConn.Close())

	require.Eventually(t, func() bool {
		return conn.GetState() == connectivity.Idle
	}, time.Second, 10*time.Millisecond)

	conn.Connect()

	var secondConn net.Conn
	select {
	case secondConn = <-lis.accepted:
		defer secondConn.Close()
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for reconnect")
	}

	_, err = healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	require.NoError(t, err)
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
