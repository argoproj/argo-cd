package grpc

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	_ "google.golang.org/grpc/balancer/grpclb"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

func startDelayingDNSServer(t *testing.T, delay time.Duration) (addr string) {
	t.Helper()
	lc := net.ListenConfig{}
	pc, err := lc.ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = pc.Close() })

	go func() {
		buf := make([]byte, 512)
		for {
			n, remote, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			req := append([]byte(nil), buf[:n]...)
			go func() {
				time.Sleep(delay)
				if len(req) < 12 {
					return
				}
				resp := append([]byte(nil), req...)
				resp[2] = 0x81 // response
				resp[3] = 0x82 // SERVFAIL
				resp[6], resp[7] = 0, 0
				resp[8], resp[9] = 0, 0
				resp[10], resp[11] = 0, 0
				_, _ = pc.WriteTo(resp, remote)
			}()
		}
	}()
	return pc.LocalAddr().String()
}

func withSlowGRPCLBDNS(t *testing.T, delay time.Duration) {
	t.Helper()
	dnsAddr := startDelayingDNSServer(t, delay)
	orig := net.DefaultResolver
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, dnsAddr)
		},
	}
	t.Cleanup(func() { net.DefaultResolver = orig })
}

func startHealthServer(t *testing.T, listenAddr string) (port int, stop func()) {
	t.Helper()
	lc := net.ListenConfig{}
	lis, err := lc.Listen(t.Context(), "tcp", listenAddr)
	if err != nil {
		if listenAddr == "[::1]:0" {
			t.Skipf("IPv6 loopback not available: %v", err)
		}
		require.NoError(t, err)
	}

	srv := grpc.NewServer()
	hs := health.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	hs.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = srv.Serve(lis)
	}()

	return lis.Addr().(*net.TCPAddr).Port, func() {
		srv.GracefulStop()
		<-done
	}
}

func healthCheckWithBudget(t *testing.T, target string, budget time.Duration) error {
	t.Helper()
	conn, err := grpc.NewClient(target, grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	ctx, cancel := context.WithTimeout(t.Context(), budget)
	defer cancel()

	_, err = healthpb.NewHealthClient(conn).Check(ctx, &healthpb.HealthCheckRequest{})
	return err
}

func TestSelfHealthCheckTarget_SurvivesSlowGRPCLBSRV(t *testing.T) {
	port, stop := startHealthServer(t, "127.0.0.1:0")
	defer stop()

	withSlowGRPCLBDNS(t, 3*time.Second)

	const probeBudget = 500 * time.Millisecond

	t.Run("legacy localhost target exceeds probe budget", func(t *testing.T) {
		err := healthCheckWithBudget(t, fmt.Sprintf("localhost:%d", port), probeBudget)
		require.Error(t, err, "slow _grpclb SRV must cause the health Check to fail under the probe budget")
		assert.Contains(t, err.Error(), "DeadlineExceeded")
	})

	t.Run("SelfHealthCheckTarget succeeds under probe budget", func(t *testing.T) {
		err := healthCheckWithBudget(t, SelfHealthCheckTarget("127.0.0.1", port), probeBudget)
		require.NoError(t, err)
	})
}

func TestSelfHealthCheckTarget_Format(t *testing.T) {
	assert.Equal(t, "passthrough:127.0.0.1:8081", SelfHealthCheckTarget("", 8081))
	assert.Equal(t, "passthrough:127.0.0.1:8081", SelfHealthCheckTarget("0.0.0.0", 8081))
	assert.Equal(t, "passthrough:[::1]:8081", SelfHealthCheckTarget("0000:0000:0000:0000:0000:0000:0000:0000", 8081))
	assert.Equal(t, "passthrough:[::1]:8081", SelfHealthCheckTarget("::", 8081))
	assert.Equal(t, "passthrough:[::1]:8081", SelfHealthCheckTarget("[::]", 8081))
	assert.Equal(t, "passthrough:[::1]:8081", SelfHealthCheckTarget("::0", 8081))
	assert.Equal(t, "passthrough:[::1]:8081", SelfHealthCheckTarget("::1", 8081))
	assert.Equal(t, "passthrough:127.0.0.1:8081", SelfHealthCheckTarget("127.0.0.1", 8081))
}

func TestSelfHealthCheckTarget_IPv6Listener(t *testing.T) {
	port, stop := startHealthServer(t, "[::1]:0")
	defer stop()

	err := healthCheckWithBudget(t, SelfHealthCheckTarget("::", port), time.Second)
	require.NoError(t, err)
}
