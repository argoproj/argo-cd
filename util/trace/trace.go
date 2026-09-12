package trace

import (
	"context"
	"fmt"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.6.1"
	oteltrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials"
)

// newResource builds the OTel resource shared by the trace and metric providers.
// otlpAttrs are colon-separated key:value pairs; malformed entries are skipped.
//
// service.instance.id is set to the hostname (the pod name under Kubernetes) so
// telemetry from separate replicas stays distinguishable. Without it every replica
// of a component reports an identical resource, and because metrics are pushed
// rather than scraped there is no per-target `instance` label to fall back on:
// cumulative series from different replicas collide and overwrite each other.
func newResource(ctx context.Context, serviceName string, otlpAttrs []string) (*resource.Resource, error) {
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

	baseAttrs := []attribute.KeyValue{semconv.ServiceNameKey.String(serviceName)}
	if hostname, err := os.Hostname(); err == nil {
		baseAttrs = append(baseAttrs, semconv.ServiceInstanceIDKey.String(hostname))
	} else {
		// Not fatal: every caller treats an init failure as fatal, and telemetry
		// missing one attribute beats the component refusing to start.
		log.Warnf("failed to determine hostname, telemetry from separate replicas may be indistinguishable: %v", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(baseAttrs...),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	return res, nil
}

// InitTracer initializes the trace provider and the otel grpc exporter.
//
// sampleRatio controls head-based sampling: 1.0 samples every trace, 0.0 samples
// none, and values in between sample that fraction of traces. Callers are expected to
// pass a value in [0.0, 1.0] (the --otlp-sample-ratio flag is range-validated at parse
// time via cli.BoundedFloat64Var). The sampler is parent-based, so a sampling decision
// made upstream (e.g. the controller) is honored by every downstream service the trace
// context propagates to (repo-server, commit-server, ...), keeping each trace whole
// rather than partially sampled across process boundaries.
//
// Because the sampler is parent-based, an incoming request that already carries a
// W3C traceparent marked "not sampled" is not recorded even when sampleRatio is 1.0:
// the upstream sampling decision wins. This differs from the previous always-on
// sampler, which recorded every request regardless of any inbound sampling flag.
func InitTracer(ctx context.Context, serviceName, otlpAddress string, otlpInsecure bool, otlpHeaders map[string]string, otlpAttrs []string, sampleRatio float64) (func(), error) {
	res, err := newResource(ctx, serviceName, otlpAttrs)
	if err != nil {
		return nil, err
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

// EndSpan ends span, recording an error status when err is non-nil. Defer it inside a
// closure so err is read at function exit rather than at defer-statement time (when a
// named return is still nil):
//
//	defer func() { trace.EndSpan(span, retErr) }()
func EndSpan(span oteltrace.Span, err error) {
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
	}
	span.End()
}
