package trace

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prombridge "go.opentelemetry.io/contrib/bridges/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"google.golang.org/grpc/credentials"
)

// InitMeter initializes the global OpenTelemetry meter provider and pushes the
// existing Prometheus metrics held by gatherers to the OTLP collector at
// otlpAddress. The Prometheus bridge lets us reuse the component's existing
// Prometheus instrumentation (the same registries scraped at /metrics) instead of
// re-instrumenting every metric with the OTel API; a periodic reader gathers them
// every interval and the OTLP gRPC exporter pushes them to the collector. A
// non-positive interval falls back to the SDK default. The OTLP options mirror
// InitTracer so metrics and traces share the same --otlp-* configuration.
//
// Pass registries as separate gatherers rather than pre-combining them into a
// prometheus.Gatherers: the bridge drops a gatherer's entire output when it
// returns an error, so combining them means one inconsistent metric family
// discards every registry for that cycle.
func InitMeter(ctx context.Context, serviceName, otlpAddress string, otlpInsecure bool, otlpHeaders map[string]string, otlpAttrs []string, interval time.Duration, gatherers ...prometheus.Gatherer) (func(), error) {
	res, err := newResource(ctx, serviceName, otlpAttrs)
	if err != nil {
		return nil, err
	}

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
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Bridge the existing Prometheus registries into the OTel metric pipeline. The
	// periodic reader collects from the bridge every interval and the exporter
	// pushes the result to the collector.
	readerOpts := make([]sdkmetric.PeriodicReaderOption, 0, len(gatherers)+1)
	for _, g := range gatherers {
		readerOpts = append(readerOpts, sdkmetric.WithProducer(prombridge.NewMetricProducer(prombridge.WithGatherer(g))))
	}
	if interval > 0 {
		readerOpts = append(readerOpts, sdkmetric.WithInterval(interval))
	}
	reader := sdkmetric.NewPeriodicReader(exporter, readerOpts...)
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
	)
	otel.SetMeterProvider(provider)

	return func() {
		// Not ctx: cancelling it is usually what triggers shutdown, so reusing it
		// here would fail the final flush and drop the last interval of metrics.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Errorf("failed to stop meter provider: %v", err)
		}
	}, nil
}
