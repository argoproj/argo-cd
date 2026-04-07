package grpc

import (
	"context"

	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"google.golang.org/grpc"
)

func RetryOnlyForServerStreamInterceptor(retryOpts ...grpc_retry.CallOption) grpc.StreamClientInterceptor {
	return func(
		ctx context.Context,
		desc *grpc.StreamDesc,
		cc *grpc.ClientConn,
		method string,
		streamer grpc.Streamer,
		opts ...grpc.CallOption,
	) (grpc.ClientStream, error) {
		if desc.ServerStreams && !desc.ClientStreams {
			return grpc_retry.StreamClientInterceptor(retryOpts...)(ctx, desc, cc, method, streamer, opts...)
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
