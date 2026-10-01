package trace

import (
	"context"
	"crypto/tls"
	"crypto/x509"
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

	"github.com/argoproj/argo-cd/v3/util/cert"
)

// collectorTLSCredentials returns the TLS credentials for a secure collector
// connection: the CA configured for the collector host in argocd-tls-certs-cm,
// else the system roots. It returns nil when the OTEL_EXPORTER_OTLP_*CERTIFICATE
// env vars are set, so the exporter builds TLS from them (custom CA, mTLS).
// signal is the exporter's env var infix ("TRACES" or "METRICS"); like the SDK,
// other signals' vars and empty values are ignored.
//
// Credentials are always explicit otherwise: without them, the exporter would
// honor OTEL_EXPORTER_OTLP_INSECURE or an http:// OTEL_EXPORTER_OTLP_ENDPOINT
// and silently send plaintext despite otlp.insecure=false.
func collectorTLSCredentials(otlpAddress, signal string) (credentials.TransportCredentials, error) {
	for _, name := range []string{"CERTIFICATE", "CLIENT_CERTIFICATE", signal + "_CERTIFICATE", signal + "_CLIENT_CERTIFICATE"} {
		if strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_"+name)) != "" {
			return nil, nil
		}
	}
	certs, err := cert.GetCertificateForConnect(otlpAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to load TLS certificates for OTLP collector %s: %w", otlpAddress, err)
	}
	var rootCAs *x509.CertPool // nil: system roots
	if len(certs) > 0 {
		rootCAs = cert.GetCertPoolFromPEMData(certs)
	}
	return credentials.NewTLS(&tls.Config{RootCAs: rootCAs}), nil
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

	// Explicit options override the standard OTEL_EXPORTER_OTLP_* env vars, so
	// headers are only set when configured; see collectorTLSCredentials for TLS.
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(otlpAddress)}
	if otlpInsecure {
		opts = append(opts, otlptracegrpc.WithInsecure())
	} else {
		creds, err := collectorTLSCredentials(otlpAddress, "TRACES")
		if err != nil {
			return nil, err
		}
		if creds != nil {
			opts = append(opts, otlptracegrpc.WithTLSCredentials(creds))
		}
	}
	if len(otlpHeaders) > 0 {
		opts = append(opts, otlptracegrpc.WithHeaders(otlpHeaders))
	}

	// set up a trace exporter
	exporter, err := otlptracegrpc.New(ctx, opts...)
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
