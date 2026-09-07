package grpc

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// reportActive is called with the current number of in-flight requests whenever it changes.
// It is used to keep a metric gauge in sync with the limiter's own counter. It may be nil.
type reportActiveFunc func(active int64)

func report(fn reportActiveFunc, active int64) {
	if fn != nil {
		fn(active)
	}
}

// ConcurrencyLimiterUnaryServerInterceptor returns a gRPC unary server interceptor that:
//   - Tracks the number of in-flight requests and reports it via reportActive (for metrics/HPA).
//   - Returns codes.ResourceExhausted immediately if maxConcurrentRequests > 0 and the limit is reached.
//
// A single atomic counter is the source of truth for both limit enforcement and the value passed to
// reportActive, so the metric can never drift from the count the limiter actually acts on. When
// maxConcurrentRequests <= 0 the interceptor only reports the active count without enforcing a limit.
// reportActive may be nil, in which case no value is reported.
func ConcurrencyLimiterUnaryServerInterceptor(maxConcurrentRequests int64, reportActive reportActiveFunc) grpc.UnaryServerInterceptor {
	var active atomic.Int64
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		current := active.Add(1)
		if maxConcurrentRequests > 0 && current > maxConcurrentRequests {
			report(reportActive, active.Add(-1))
			return nil, status.Errorf(codes.ResourceExhausted,
				"repo-server is overloaded: active requests (%d) exceed limit (%d); retry with backoff",
				current, maxConcurrentRequests)
		}
		report(reportActive, current)
		defer func() { report(reportActive, active.Add(-1)) }()

		return handler(ctx, req)
	}
}

// ConcurrencyLimiterStreamServerInterceptor returns a gRPC stream server interceptor with the
// same concurrency tracking and limiting behaviour as ConcurrencyLimiterUnaryServerInterceptor.
func ConcurrencyLimiterStreamServerInterceptor(maxConcurrentRequests int64, reportActive reportActiveFunc) grpc.StreamServerInterceptor {
	var active atomic.Int64
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		current := active.Add(1)
		if maxConcurrentRequests > 0 && current > maxConcurrentRequests {
			report(reportActive, active.Add(-1))
			return status.Errorf(codes.ResourceExhausted,
				"repo-server is overloaded: active requests (%d) exceed limit (%d); retry with backoff",
				current, maxConcurrentRequests)
		}
		report(reportActive, current)
		defer func() { report(reportActive, active.Add(-1)) }()

		return handler(srv, ss)
	}
}
