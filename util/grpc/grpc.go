package grpc

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"runtime/debug"
	"strings"
	"sync"
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
	dial := func(dialCtx context.Context) (net.Conn, error) {
		rawConn, err := proxy.Dial(dialCtx, network, address)
		if err != nil {
			return nil, fmt.Errorf("error dial proxy: %w", err)
		}

		if creds != nil {
			conn, _, err := creds.ClientHandshake(dialCtx, address, rawConn)
			if err != nil {
				_ = rawConn.Close()
				return nil, fmt.Errorf("error creating connection: %w", err)
			}
			rawConn = conn
		}

		return rawConn, nil
	}

	// Establish the first connection before creating the client.
	// The custom dialer uses this connection for the initial dial.
	firstConn, err := dial(ctx)
	if err != nil {
		return nil, err
	}

	var firstDial sync.Once
	customDialer := func(dialCtx context.Context, _ string) (net.Conn, error) {
		var conn net.Conn
		firstDial.Do(func() {
			conn = firstConn
			firstConn = nil
		})
		if conn != nil {
			return conn, nil
		}
		return dial(dialCtx)
	}

	opts = append(opts,
		grpc.WithContextDialer(customDialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: common.GetGRPCKeepAliveTime()}),
	)

	conn, err := grpc.NewClient("passthrough:"+address, opts...)
	if err != nil {
		_ = firstConn.Close()
		return nil, fmt.Errorf("grpc.NewClient failed: %w", err)
	}

	conn.Connect()
	if err := waitForReady(ctx, conn); err != nil {
		_ = conn.Close()
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

func TestTLS(address string, dialTime time.Duration) (*TLSTestResult, error) {
	if parts := strings.Split(address, ":"); len(parts) == 1 {
		// If port is unspecified, assume the most likely port
		address += ":443"
	}
	var testResult TLSTestResult
	var tlsConfig tls.Config
	tlsConfig.InsecureSkipVerify = true
	creds := credentials.NewTLS(&tlsConfig)

	// Set timeout when dialing to the server
	// fix: https://github.com/argoproj/argo-cd/issues/9679
	ctx, cancel := context.WithTimeout(context.Background(), dialTime)
	defer cancel()

	conn, err := BlockingNewClient(ctx, "tcp", address, creds)
	if err == nil {
		_ = conn.Close()
		testResult.TLS = true
		creds := credentials.NewTLS(&tls.Config{})
		ctx, cancel := context.WithTimeout(context.Background(), dialTime)
		defer cancel()

		conn, err := BlockingNewClient(ctx, "tcp", address, creds)
		if err == nil {
			_ = conn.Close()
		} else {
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
	conn, err = BlockingNewClient(ctx, "tcp", address, nil)
	if err == nil {
		_ = conn.Close()
		testResult.TLS = false
		return &testResult, nil
	}
	return nil, err
}
