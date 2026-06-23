package trace

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.6.1"
	"google.golang.org/grpc/credentials"
)

// InitTracer initializes the trace provider and the otel grpc exporter.
//
// sampleRatio controls head-based sampling: 1.0 samples every trace, 0.0 samples
// none, and values in between sample that fraction of traces. The sampler is
// parent-based, so a sampling decision made upstream (e.g. the controller) is
// honored by every downstream service the trace context propagates to
// (repo-server, commit-server, ...), keeping each trace whole rather than
// partially sampled across process boundaries.
func InitTracer(ctx context.Context, serviceName, otlpAddress string, otlpInsecure bool, otlpHeaders map[string]string, otlpAttrs []string, sampleRatio float64) (func(), error) {
	attrs := make([]attribute.KeyValue, 0, len(otlpAttrs))
	for i := range otlpAttrs {
		attr := otlpAttrs[i]
		slice := strings.Split(attr, ":")
		if len(slice) != 2 {
			log.Warnf("OTLP attr '%s' split with ':' length not 2", attr)
			continue
		}
		attrs = append(attrs, attribute.String(slice[0], slice[1]))
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// set up grpc options based on secure/insecure connection
	var secureOption otlptracegrpc.Option
	if otlpInsecure {
		secureOption = otlptracegrpc.WithInsecure()
	} else {
		secureOption = otlptracegrpc.WithTLSCredentials(credentials.NewClientTLSFromCert(nil, ""))
	}

	// set up a trace exporter
	exporter, err := otlptracegrpc.New(ctx,
		secureOption,
		otlptracegrpc.WithEndpoint(otlpAddress),
		otlptracegrpc.WithHeaders(otlpHeaders),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Register the trace exporter with a TracerProvider, using a batch
	// span processor to aggregate spans before export.
	bsp := sdktrace.NewBatchSpanProcessor(exporter)
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(sampleRatio))),
		sdktrace.WithResource(res),
		sdktrace.WithSpanProcessor(bsp),
	)

	// set global propagator to tracecontext (the default is no-op).
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(provider)

	return func() {
		if err := exporter.Shutdown(ctx); err != nil {
			log.Errorf("failed to stop exporter: %v", err)
		}
	}, nil
}
