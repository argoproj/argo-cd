package reposerver

import (
	"context"

	"google.golang.org/grpc"
)

// cancelOnShutdownUnaryInterceptor ties request contexts to shutdownCtx, so cancelling it aborts
// in-flight work. GracefulStop only waits for handlers, so a long git operation instead runs past the
// container's grace period and loses its cleanup to the kubelet's SIGKILL.
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
