package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prombridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc/credentials"
)

// InitMeter pushes the Prometheus metrics in gatherers (what /metrics serves) to
// the OTLP collector every interval via the Prometheus bridge. OTLP options mirror
// InitTracer.
//
// The provider is not installed as the global: nothing records through the OTel
// metrics API, and a global would make otelgrpc/otelhttp emit rpc.*/http.* series
// absent from /metrics.
//
// Pass registries separately, not pre-combined in a prometheus.Gatherers: the
// bridge drops a whole gatherer on error, so one bad family would discard every
// registry. The error is logged via the OTel error handler.
func InitMeter(ctx context.Context, serviceName, otlpAddress string, otlpInsecure bool, otlpHeaders map[string]string, otlpAttrs []string, interval time.Duration, gatherers ...prometheus.Gatherer) (func(), error) {
	res, err := newResource(ctx, serviceName, otlpAttrs)
	if err != nil {
		return nil, err
	}
	installErrorHandler()

	// set up grpc options based on secure/insecure connection
	var secureOption otlpmetricgrpc.Option
	if otlpInsecure {
		secureOption = otlpmetricgrpc.WithInsecure()
	} else {
		secureOption = otlpmetricgrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	exporter, err := otlpmetricgrpc.New(ctx,
		secureOption,
		otlpmetricgrpc.WithEndpoint(otlpAddress),
		otlpmetricgrpc.WithHeaders(otlpHeaders),
		// No retry: metrics are cumulative so the next push covers a miss, and retry
		// backoff would stall the closer's final flush (run on every argocd-server
		// graceful restart) when the collector is down.
		otlpmetricgrpc.WithRetry(otlpmetricgrpc.RetryConfig{Enabled: false}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// One producer, many gatherers: failures stay isolated, one scope per push.
	bridgeOpts := make([]prombridge.Option, 0, len(gatherers))
	for _, g := range gatherers {
		bridgeOpts = append(bridgeOpts, prombridge.WithGatherer(g))
	}
	reader := sdkmetric.NewPeriodicReader(exporter,
		sdkmetric.WithProducer(prombridge.NewMetricProducer(bridgeOpts...)),
		sdkmetric.WithInterval(interval),
	)
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)

	return func() {
		// Not ctx: its cancellation usually triggers shutdown and would fail the flush.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Errorf("failed to stop meter provider: %v", err)
		}
	}, nil
}
