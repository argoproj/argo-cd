package reposerver

import (
	"context"

	"google.golang.org/grpc"
)

// cancelOnShutdownUnaryInterceptor ties every request context to shutdownCtx, so that cancelling it
// aborts in-flight work. grpc.GracefulStop() only waits for handlers to return, which for repo-server
// means waiting out a Git operation that may run for minutes - long past the container's termination
// grace period, at which point the kubelet SIGKILLs the whole cgroup and Git leaves its lock files
// behind. Cancelling instead lets Git terminate gracefully and clean up.
func cancelOnShutdownUnaryInterceptor(shutdownCtx context.Context) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		defer context.AfterFunc(shutdownCtx, cancel)()
		return handler(ctx, req)
	}
}

// cancelOnShutdownStreamInterceptor is cancelOnShutdownUnaryInterceptor for streaming calls.
func cancelOnShutdownStreamInterceptor(shutdownCtx context.Context) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, _ *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx, cancel := context.WithCancel(stream.Context())
		defer cancel()
		defer context.AfterFunc(shutdownCtx, cancel)()
		return handler(srv, &cancellableServerStream{ServerStream: stream, ctx: ctx})
	}
}

type cancellableServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *cancellableServerStream) Context() context.Context {
	return s.ctx
}
