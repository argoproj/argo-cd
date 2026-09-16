package grpc

import (
	"context"
	"strings"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// healthServiceMethodPrefix is the fully-qualified method prefix of the gRPC health service.
// Health-check RPCs are exempt from the concurrency limit: the liveness probe self-dials the
// repo-server, and if its own health check were rejected while the server is saturated (exactly
// when this limiter engages), the probe would fail and kubelet would restart an otherwise healthy
// but busy pod — the opposite of graceful scale-out.
const healthServiceMethodPrefix = "/grpc.health.v1.Health/"

func isHealthCheckMethod(fullMethod string) bool {
	return strings.HasPrefix(fullMethod, healthServiceMethodPrefix)
}

// ConcurrencyLimiter caps the number of concurrent gRPC requests the repo-server handles and tracks
// the current in-flight count. A single shared counter backs both the unary and stream
// interceptors, so the limit and the tracked count reflect total gRPC concurrency rather than a
// separate budget per RPC kind. The same counter drives the Prometheus gauge used for HPA
// scale-out; because the gauge reads the counter on scrape (see ActiveRequests) rather than being
// pushed a snapshot, it can never drift from the value the limiter actually enforces.
//
// Requests that exceed the limit are rejected immediately with codes.ResourceExhausted so clients
// can retry on a different replica. Health-check RPCs bypass the limiter entirely.
type ConcurrencyLimiter struct {
	maxConcurrentRequests int64
	active                *atomic.Int64
}

// NewConcurrencyLimiter returns a limiter that rejects requests once maxConcurrentRequests are in
// flight; a value <= 0 disables enforcement while still tracking the active count. active is the
// counter shared with the metric gauge so both always agree; if nil, a private counter is used.
func NewConcurrencyLimiter(maxConcurrentRequests int64, active *atomic.Int64) *ConcurrencyLimiter {
	if active == nil {
		active = &atomic.Int64{}
	}
	return &ConcurrencyLimiter{maxConcurrentRequests: maxConcurrentRequests, active: active}
}

// ActiveRequests returns the number of gRPC requests currently in flight. It is safe for concurrent
// use and is intended to be read on demand (e.g. by a Prometheus collector), which keeps the
// reported value consistent with the counter the limiter enforces.
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

// StreamServerInterceptor returns a stream interceptor with the same limiting and tracking
// behaviour as UnaryServerInterceptor, sharing the same counter.
func (l *ConcurrencyLimiter) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isHealthCheckMethod(info.FullMethod) {
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
