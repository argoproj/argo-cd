package grpc

import (
	"context"

	"google.golang.org/grpc"
)

// gRPC health protocol method names are not exported as constants by
// google.golang.org/grpc/health, so we define them explicitly here.
// These values are part of the stable gRPC health checking specification.
const (
	healthCheckMethod = "/grpc.health.v1.Health/Check"
	healthWatchMethod = "/grpc.health.v1.Health/Watch"
)

// IsHealthMethod returns true if the gRPC full method name corresponds
// to a standard gRPC health check RPC.
func IsHealthMethod(fullMethod string) bool {
	return fullMethod == healthCheckMethod ||
		fullMethod == healthWatchMethod
}

// ServerStreamWithContext allows replacing the context associated with
// a grpc.ServerStream (used by interceptors).
type ServerStreamWithContext struct {
	grpc.ServerStream
	Ctx context.Context
}

func (s *ServerStreamWithContext) Context() context.Context {
	return s.Ctx
}
