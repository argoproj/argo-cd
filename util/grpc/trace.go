package grpc

import (
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"
)

var (
	otelDialOption          grpc.DialOption
	interceptorsInitialized = sync.Once{}
)

// otel interceptors must be created once to avoid memory leak
// see https://github.com/open-telemetry/opentelemetry-go-contrib/issues/4226 for details
func ensureInitialized() {
	interceptorsInitialized.Do(func() {
		otelDialOption = grpc.WithStatsHandler(otelgrpc.NewClientHandler())
	})
}

func OTELDialOption() grpc.DialOption {
	ensureInitialized()
	return otelDialOption
}
