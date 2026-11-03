package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/proxy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v3/common"
)

// LoggerRecoveryHandler return a handler for recovering from panics and returning error
func LoggerRecoveryHandler(log *logrus.Entry) recovery.RecoveryHandlerFunc {
	return func(p any) (err error) {
		log.Errorf("Recovered from panic: %+v\n%s", p, debug.Stack())
		return status.Errorf(codes.Internal, "%s", p)
	}
}

// BlockingNewClient is a helper method to dial the given address, using optional TLS credentials,
// and blocking until the returned connection is ready. If the given credentials are nil, the
// connection will be insecure (plain-text).
// Lifted from: https://github.com/fullstorydev/grpcurl/blob/master/grpcurl.go
func BlockingNewClient(ctx context.Context, network, address string, creds credentials.TransportCredentials, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	rawConn, err := proxy.Dial(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("error dial proxy: %w", err)
	}

	if creds != nil {
		rawConn, _, err = creds.ClientHandshake(ctx, address, rawConn)
		if err != nil {
			return nil, fmt.Errorf("error creating connection: %w", err)
		}
	}
	customDialer := func(_ context.Context, _ string) (net.Conn, error) {
		return rawConn, nil
	}

	opts = append(opts,
		grpc.WithContextDialer(customDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: common.GetGRPCKeepAliveTime()}),
	)

	conn, err := grpc.NewClient("passthrough:"+address, opts...)
	if err != nil {
		return nil, fmt.Errorf("grpc.NewClient failed: %w", err)
	}

	conn.Connect()
	if err := waitForReady(ctx, conn); err != nil {
		return nil, fmt.Errorf("gRPC connection not ready: %w", err)
	}

	return conn, nil
}

func waitForReady(ctx context.Context, conn *grpc.ClientConn) error {
	for {
		state := conn.GetState()
		if state == connectivity.Ready {
			return nil
		}
		if !conn.WaitForStateChange(ctx, state) {
			return ctx.Err() // context timeout or cancellation
		}
	}
}

type TLSTestResult struct {
	TLS         bool
	InsecureErr error
}

// tlsHandshake dials address the way BlockingNewClient does and completes a TLS handshake with it.
func tlsHandshake(ctx context.Context, address string, config *tls.Config) error {
	rawConn, err := proxy.Dial(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("error dial proxy: %w", err)
	}
	defer rawConn.Close()

	if config.ServerName == "" {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("error parsing address %q: %w", address, err)
		}
		config = config.Clone()
		config.ServerName = host
	}
	if err := tls.Client(rawConn, config).HandshakeContext(ctx); err != nil {
		return fmt.Errorf("error creating connection: %w", err)
	}
	return nil
}

func TestTLS(address string, dialTime time.Duration) (*TLSTestResult, error) {
	if parts := strings.Split(address, ":"); len(parts) == 1 {
		// If port is unspecified, assume the most likely port
		address += ":443"
	}
	var testResult TLSTestResult

	// Set timeout when dialing to the server
	// fix: https://github.com/argoproj/argo-cd/issues/9679
	ctx, cancel := context.WithTimeout(context.Background(), dialTime)
	defer cancel()

	// A plain TLS handshake, not a gRPC connection: a proxy that does not negotiate HTTP/2 still
	// serves TLS, and the client can reach the API through it with gRPC-web.
	err := tlsHandshake(ctx, address, &tls.Config{InsecureSkipVerify: true})
	if err == nil {
		testResult.TLS = true
		ctx, cancel := context.WithTimeout(context.Background(), dialTime)
		defer cancel()

		if err := tlsHandshake(ctx, address, &tls.Config{}); err != nil {
			// if connection was successful with InsecureSkipVerify true, but unsuccessful with
			// InsecureSkipVerify false, it means server is not configured securely
			testResult.InsecureErr = err
		}
		return &testResult, nil
	}
	// If we get here, we were unable to connect via TLS (even with InsecureSkipVerify: true)
	// It may be because server is running without TLS, or because of real issues (e.g. connection
	// refused). Test if server accepts plain-text connections
	ctx, cancel = context.WithTimeout(context.Background(), dialTime)
	defer cancel()
	conn, err := BlockingNewClient(ctx, "tcp", address, nil)
	if err == nil {
		_ = conn.Close()
		testResult.TLS = false
		return &testResult, nil
	}
	return nil, err
}
