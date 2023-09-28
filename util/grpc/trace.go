package grpc

import (
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

var (
	otelUnaryInterceptor    grpc.UnaryClientInterceptor
	otelStreamInterceptor   grpc.StreamClientInterceptor
	interceptorsInitialized = sync.Once{}
)

// otel interceptors must be created once to avoid memory leak
// see https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4226 for details
func ensureInitialized() {
	interceptorsInitialized.Do(func() {
		otelUnaryInterceptor = otelgrpc.UnaryClientInterceptor()
		otelStreamInterceptor = otelgrpc.StreamClientInterceptor()
	})
}

func OTELUnaryClientInterceptor() grpc.UnaryClientInterceptor {
	ensureInitialized()
	return otelUnaryInterceptor
}

func OTELStreamClientInterceptor() grpc.StreamClientInterceptor {
	ensureInitialized()
	return otelStreamInterceptor
}
