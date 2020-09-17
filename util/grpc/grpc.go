package grpc

import (
	"crypto/tls"
	"net"
	"runtime/debug"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// PanicLoggerUnaryServerInterceptor returns a new unary server interceptor for recovering from panics and returning error
func PanicLoggerUnaryServerInterceptor(log *logrus.Entry) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
				err = status.Errorf(codes.Internal, "%s", r)
			}
		}()
		return handler(ctx, req)
	}
}

// PanicLoggerStreamServerInterceptor returns a new streaming server interceptor for recovering from panics and returning error
func PanicLoggerStreamServerInterceptor(log *logrus.Entry) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
				err = status.Errorf(codes.Internal, "%s", r)
			}
		}()
		return handler(srv, stream)
	}
}

// BlockingDial is a helper method to dial the given address, using optional TLS credentials,
// and blocking until the returned connection is ready. If the given credentials are nil, the
// connection will be insecure (plain-text).
// Lifted from: https://github.com/fullstorydev/grpcurl/blob/master/grpcurl.go
func BlockingDial(ctx context.Context, network, address string, creds credentials.TransportCredentials, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	// grpc.Dial doesn't provide any information on permanent connection errors (like
	// TLS handshake failures). So in order to provide good error messages, we need a
	// custom dialer that can provide that info. That means we manage the TLS handshake.
	result := make(chan interface{}, 1)
	writeResult := func(res interface{}) {
		// non-blocking write: we only need the first result
		select {
		case result <- res:
		default:
		}
	}

	dialer := func(address string, timeout time.Duration) (net.Conn, error) {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		conn, err := (&net.Dialer{Cancel: ctx.Done()}).Dial(network, address)
		if err != nil {
			writeResult(err)
			return nil, err
		}
		if creds != nil {
			conn, _, err = creds.ClientHandshake(ctx, address, conn)
			if err != nil {
				writeResult(err)
				return nil, err
			}
		}
		return conn, nil
	}

	// Even with grpc.FailOnNonTempDialError, this call will usually timeout in
	// the face of TLS handshake errors. So we can't rely on grpc.WithBlock() to
	// know when we're done. So we run it in a goroutine and then use result
	// channel to either get the channel or fail-fast.
	go func() {
		opts = append(opts,
			grpc.WithBlock(),
			grpc.FailOnNonTempDialError(true),
			grpc.WithDialer(dialer),
			grpc.WithInsecure(), // we are handling TLS, so tell grpc not to
			grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 10 * time.Second}),
		)
		conn, err := grpc.DialContext(ctx, address, opts...)
		var res interface{}
		if err != nil {
			res = err
		} else {
			res = conn
		}
		writeResult(res)
	}()

	select {
	case res := <-result:
		if conn, ok := res.(*grpc.ClientConn); ok {
			return conn, nil
		}
		return nil, res.(error)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type TLSTestResult struct {
	TLS         bool
	InsecureErr error
}

func TestTLS(address string) (*TLSTestResult, error) {
	if parts := strings.Split(address, ":"); len(parts) == 1 {
		// If port is unspecified, assume the most likely port
		address += ":443"
	}
	var testResult TLSTestResult
	var tlsConfig tls.Config
	tlsConfig.InsecureSkipVerify = true
	creds := credentials.NewTLS(&tlsConfig)
	conn, err := BlockingDial(context.Background(), "tcp", address, creds)
	if err == nil {
		_ = conn.Close()
		testResult.TLS = true
		creds := credentials.NewTLS(&tls.Config{})
		conn, err := BlockingDial(context.Background(), "tcp", address, creds)
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
	conn, err = BlockingDial(context.Background(), "tcp", address, nil)
	if err == nil {
		_ = conn.Close()
		testResult.TLS = false
		return &testResult, nil
	}
	return nil, err
}

func WithTimeout(duration time.Duration) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		clientDeadline := time.Now().Add(duration)
		ctx, cancel := context.WithDeadline(ctx, clientDeadline)
		defer cancel()
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}
