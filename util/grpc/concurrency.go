package grpc

import (
	"context"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// healthServiceMethodPrefix is the method prefix of the gRPC health service. Health-check RPCs are
// exempt from the limit: the liveness probe self-dials the repo-server, so rejecting its health check
// while the server is saturated would restart an otherwise healthy pod — the opposite of scale-out.
const healthServiceMethodPrefix = "/grpc.health.v1.Health/"

func isHealthCheckMethod(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, healthServiceMethodPrefix)
}

// ConcurrencyLimiter caps concurrent gRPC requests and tracks the in-flight count. One counter backs
// both interceptors, so the limit covers total gRPC concurrency rather than a budget per RPC kind.
// The limiter owns the counter; the HPA gauge reads it on scrape via ActiveRequests, so the reported
// value can't drift from what the limiter enforces.
//
// Requests over the limit are rejected with codes.ResourceExhausted so clients retry on another
// replica. Health-check and client-streaming RPCs bypass the limiter (see StreamServerInterceptor).
type ConcurrencyLimiter struct {
	maxConcurrentRequests int64
	active                atomic.Int64
}

// NewConcurrencyLimiter returns a limiter that rejects requests once maxConcurrentRequests are in
// flight; a value <= 0 disables enforcement while still tracking the active count.
func NewConcurrencyLimiter(maxConcurrentRequests int64) *ConcurrencyLimiter {
	return &ConcurrencyLimiter{maxConcurrentRequests: maxConcurrentRequests}
}

// ActiveRequests returns the number of gRPC requests currently in flight. Safe for concurrent use;
// intended to be read on demand by the Prometheus collector.
func (l *ConcurrencyLimiter) ActiveRequests() int64 {
	return l.active.Load()
}

// UnaryServerInterceptor returns a unary interceptor that enforces the limit and tracks the active
// count. Health-check RPCs bypass both.
func (l *ConcurrencyLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isHealthCheckMethod(info.FullMethod) {
			return handler(ctx, req)
		}
		if err := l.acquire(); err != nil {
			return nil, err
		}
		defer l.release()
		return handler(ctx, req)
	}
}

// StreamServerInterceptor limits and tracks streaming RPCs, sharing UnaryServerInterceptor's counter.
//
// Client-streaming RPCs (info.IsClientStream, which also covers bidi) are exempt: the client only
// retries server-streaming calls (RetryOnlyForServerStreamInterceptor), so rejecting a client stream
// like GenerateManifestWithFiles would be a hard failure rather than a retry on another replica.
// --parallelism-limit-fail-fast provides backpressure for that manifest path instead.
func (l *ConcurrencyLimiter) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isHealthCheckMethod(info.FullMethod) || info.IsClientStream {
			return handler(srv, ss)
		}
		if err := l.acquire(); err != nil {
			return err
		}
		defer l.release()
		return handler(srv, ss)
	}
}

// acquire reserves a slot, returning codes.ResourceExhausted when the limit is exceeded. On
// rejection the reserved slot is released so the active count reflects only requests being served.
func (l *ConcurrencyLimiter) acquire() error {
	current := l.active.Add(1)
	if l.maxConcurrentRequests > 0 && current > l.maxConcurrentRequests {
		l.active.Add(-1)
		return status.Errorf(codes.ResourceExhausted,
			"repo-server is overloaded: active requests (%d) exceed limit (%d); retry with backoff",
			current, l.maxConcurrentRequests)
	}
	return nil
}

func (l *ConcurrencyLimiter) release() {
	l.active.Add(-1)
}
