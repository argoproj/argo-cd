package grpc

import (
	"context"
	"sync/atomic"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ActiveRequestsTracker is implemented by anything that can track active gRPC request counts.
type ActiveRequestsTracker interface {
	IncActiveGRPCRequests()
	DecActiveGRPCRequests()
}

// ConcurrencyLimiterUnaryServerInterceptor returns a gRPC unary server interceptor that:
//   - Tracks the number of in-flight requests via the provided tracker (for metrics/HPA).
//   - Returns codes.ResourceExhausted immediately if maxConcurrentRequests > 0 and the limit is reached.
//
// When maxConcurrentRequests <= 0 the interceptor only tracks metrics without enforcing a limit.
func ConcurrencyLimiterUnaryServerInterceptor(maxConcurrentRequests int64, tracker ActiveRequestsTracker) grpc.UnaryServerInterceptor {
	var active atomic.Int64
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if maxConcurrentRequests > 0 {
			current := active.Add(1)
			if current > maxConcurrentRequests {
				active.Add(-1)
				return nil, status.Errorf(codes.ResourceExhausted,
					"repo-server is overloaded: active requests (%d) exceed limit (%d); retry with backoff",
					current, maxConcurrentRequests)
			}
			defer active.Add(-1)
		}

		if tracker != nil {
			tracker.IncActiveGRPCRequests()
			defer tracker.DecActiveGRPCRequests()
		}

		return handler(ctx, req)
	}
}

// ConcurrencyLimiterStreamServerInterceptor returns a gRPC stream server interceptor with the
// same concurrency tracking and limiting behaviour as ConcurrencyLimiterUnaryServerInterceptor.
func ConcurrencyLimiterStreamServerInterceptor(maxConcurrentRequests int64, tracker ActiveRequestsTracker) grpc.StreamServerInterceptor {
	var active atomic.Int64
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if maxConcurrentRequests > 0 {
			current := active.Add(1)
			if current > maxConcurrentRequests {
				active.Add(-1)
				return status.Errorf(codes.ResourceExhausted,
					"repo-server is overloaded: active requests (%d) exceed limit (%d); retry with backoff",
					current, maxConcurrentRequests)
			}
			defer active.Add(-1)
		}

		if tracker != nil {
			tracker.IncActiveGRPCRequests()
			defer tracker.DecActiveGRPCRequests()
		}

		return handler(srv, ss)
	}
}
