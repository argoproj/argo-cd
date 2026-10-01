package trace

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

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
	"google.golang.org/grpc/credentials/insecure"

	"github.com/argoproj/argo-cd/v3/util/cert"
)

var errorHandlerOnce sync.Once

// installErrorHandler routes OTel SDK errors to logrus; the SDK default writes
// to the stdlib logger, bypassing --logformat.
func installErrorHandler() {
	errorHandlerOnce.Do(func() {
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
			log.WithError(err).Error("OpenTelemetry error")
		}))
	})
}

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

// collectorTLSCredentials builds the TLS credentials for a secure collector
// connection from the standard OTEL_EXPORTER_OTLP_[<signal>_]CERTIFICATE,
// CLIENT_CERTIFICATE and CLIENT_KEY env vars, where signal is "TRACES" or
// "METRICS" and the signal-specific vars win, as in the SDK. Without a CA
// there, the CA for the collector host in argocd-tls-certs-cm is used, else the
// system roots.
//
// The exporter could read these vars itself, but it silently skips an
// unreadable CA or an incomplete client cert/key pair, and without credentials
// it honors OTEL_EXPORTER_OTLP_INSECURE or an http:// endpoint. Building them
// here keeps otlp.insecure=false on TLS and turns bad TLS config into an error.
func collectorTLSCredentials(otlpAddress, signal string) (credentials.TransportCredentials, error) {
	env := func(name string) string {
		return strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_" + name))
	}
	cfg := &tls.Config{}

	caPath := env(signal + "_CERTIFICATE")
	if caPath == "" {
		caPath = env("CERTIFICATE")
	}
	if caPath != "" {
		data, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read OTLP collector CA: %w", err)
		}
		cfg.RootCAs = x509.NewCertPool()
		if !cfg.RootCAs.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("no certificates found in OTLP collector CA %s", caPath)
		}
	} else {
		certs, err := cert.GetCertificateForConnect(otlpAddress)
		if err != nil {
			return nil, fmt.Errorf("failed to load TLS certificates for OTLP collector %s: %w", otlpAddress, err)
		}
		if len(certs) > 0 {
			cfg.RootCAs = cert.GetCertPoolFromPEMData(certs)
		}
	}

	// Like the SDK, the cert and key are read as a pair: the signal-specific
	// pair if both are set, else the generic one. Unlike the SDK, a lone half is
	// an error rather than silently dropping the client cert.
	var certPath, keyPath string
	sc, sk := env(signal+"_CLIENT_CERTIFICATE"), env(signal+"_CLIENT_KEY")
	gc, gk := env("CLIENT_CERTIFICATE"), env("CLIENT_KEY")
	switch {
	case sc != "" && sk != "":
		certPath, keyPath = sc, sk
	case gc != "" && gk != "":
		certPath, keyPath = gc, gk
	case sc != "" || sk != "" || gc != "" || gk != "":
		return nil, errors.New("OTLP client certificate and key must both be set")
	}
	if certPath != "" {
		pair, err := tls.LoadX509KeyPair(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load OTLP client certificate: %w", err)
		}
		cfg.Certificates = []tls.Certificate{pair}
	}
	return credentials.NewTLS(cfg), nil
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
	installErrorHandler()

	// Explicit options override the standard OTEL_EXPORTER_OTLP_* env vars, so
	// headers are only set when configured; see collectorTLSCredentials for TLS.
	opts := []otlptracegrpc.Option{otlptracegrpc.WithEndpoint(otlpAddress)}
	if otlpInsecure {
		// Credentials, not WithInsecure: the exporter prefers TLS built from the
		// OTEL_EXPORTER_OTLP_*CERTIFICATE env vars over its insecure flag.
		opts = append(opts, otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()))
	} else {
		creds, err := collectorTLSCredentials(otlpAddress, "TRACES")
		if err != nil {
			return nil, err
		}
		opts = append(opts, otlptracegrpc.WithTLSCredentials(creds))
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
		// Provider, not exporter, so the batch processor drains. Not ctx: its
		// cancellation usually triggers shutdown and would fail the flush.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			log.Errorf("failed to stop tracer provider: %v", err)
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
