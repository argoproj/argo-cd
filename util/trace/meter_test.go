package trace

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	collectormetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

type exportCall struct {
	req *collectormetricspb.ExportMetricsServiceRequest
	md  metadata.MD
}

// fakeCollector is an in-process OTLP metrics receiver.
type fakeCollector struct {
	collectormetricspb.UnimplementedMetricsServiceServer
	calls chan exportCall
}

func (c *fakeCollector) Export(ctx context.Context, req *collectormetricspb.ExportMetricsServiceRequest) (*collectormetricspb.ExportMetricsServiceResponse, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	c.calls <- exportCall{req: req, md: md}
	return &collectormetricspb.ExportMetricsServiceResponse{}, nil
}

func startMetricsCollector(t *testing.T, opts ...grpc.ServerOption) (string, *fakeCollector) {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c := &fakeCollector{calls: make(chan exportCall, 64)}
	srv := grpc.NewServer(opts...)
	collectormetricspb.RegisterMetricsServiceServer(srv, c)
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String(), c
}

func (c *fakeCollector) next(t *testing.T) exportCall {
	t.Helper()
	select {
	case call := <-c.calls:
		return call
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for an OTLP export")
		return exportCall{}
	}
}

func metricNames(req *collectormetricspb.ExportMetricsServiceRequest) []string {
	var names []string
	for _, rm := range req.GetResourceMetrics() {
		for _, sm := range rm.GetScopeMetrics() {
			for _, m := range sm.GetMetrics() {
				names = append(names, m.GetName())
			}
		}
	}
	return names
}

func resourceAttrs(req *collectormetricspb.ExportMetricsServiceRequest) map[string]string {
	attrs := map[string]string{}
	for _, rm := range req.GetResourceMetrics() {
		for _, kv := range rm.GetResource().GetAttributes() {
			attrs[kv.GetKey()] = kv.GetValue().GetStringValue()
		}
	}
	return attrs
}

func newCounterRegistry(t *testing.T, name string) *prometheus.Registry {
	t.Helper()
	reg := prometheus.NewRegistry()
	c := prometheus.NewCounter(prometheus.CounterOpts{Name: name})
	require.NoError(t, reg.Register(c))
	c.Inc()
	return reg
}

func TestInitMeter_PushesBridgedMetrics(t *testing.T) {
	addr, collector := startMetricsCollector(t)
	before := otel.GetMeterProvider()

	closer, err := InitMeter(t.Context(), "argocd-test", addr, true,
		map[string]string{"x-test-header": "yes"}, []string{"env:ci", "malformed"},
		50*time.Millisecond, newCounterRegistry(t, "argocd_test_total"))
	require.NoError(t, err)
	t.Cleanup(closer)

	call := collector.next(t)
	assert.Contains(t, metricNames(call.req), "argocd_test_total")
	assert.Equal(t, []string{"yes"}, call.md.Get("x-test-header"))

	attrs := resourceAttrs(call.req)
	assert.Equal(t, "argocd-test", attrs["service.name"])
	hostname, err := os.Hostname()
	require.NoError(t, err)
	assert.Equal(t, hostname, attrs["service.instance.id"])
	assert.Equal(t, "ci", attrs["env"])
	assert.NotContains(t, attrs, "malformed")

	// One producer for all gatherers: one scope per push.
	require.Len(t, call.req.GetResourceMetrics(), 1)
	assert.Len(t, call.req.GetResourceMetrics()[0].GetScopeMetrics(), 1)

	// Global stays noop so otelgrpc/otelhttp emit nothing.
	assert.Same(t, before, otel.GetMeterProvider())
}

func TestInitMeter_FailingGathererDoesNotDropOthers(t *testing.T) {
	addr, collector := startMetricsCollector(t)
	hook := logtest.NewGlobal()
	t.Cleanup(hook.Reset)
	failing := prometheus.GathererFunc(func() ([]*dto.MetricFamily, error) {
		return nil, errors.New("gather failed")
	})

	closer, err := InitMeter(t.Context(), "argocd-test", addr, true, nil, nil,
		50*time.Millisecond, failing, newCounterRegistry(t, "argocd_healthy_total"))
	require.NoError(t, err)
	t.Cleanup(closer)

	assert.Contains(t, metricNames(collector.next(t).req), "argocd_healthy_total")

	// Gather error reaches logrus, not the SDK's stdlib logger.
	require.Eventually(t, func() bool {
		for _, e := range hook.AllEntries() {
			if e.Level == log.ErrorLevel && e.Data["error"] != nil && strings.Contains(e.Data["error"].(error).Error(), "gather failed") {
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond)
}

func TestInitMeter_CloserReturnsPromptlyWhenCollectorUnreachable(t *testing.T) {
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := lis.Addr().String()
	require.NoError(t, lis.Close())

	closer, err := InitMeter(t.Context(), "argocd-test", addr, true, nil, nil,
		time.Hour, newCounterRegistry(t, "argocd_unreachable_total"))
	require.NoError(t, err)

	// Retry is off, so the final flush fails fast rather than eating the 10s budget.
	start := time.Now()
	closer()
	assert.Less(t, time.Since(start), 5*time.Second)
}

func TestInitMeter_CloserFlushesAfterContextCancel(t *testing.T) {
	addr, collector := startMetricsCollector(t)
	ctx, cancel := context.WithCancel(t.Context())

	// Interval outlives the test: the only export is the closer's flush.
	closer, err := InitMeter(ctx, "argocd-test", addr, true, nil, nil,
		time.Hour, newCounterRegistry(t, "argocd_flushed_total"))
	require.NoError(t, err)

	cancel()
	closer()

	assert.Contains(t, metricNames(collector.next(t).req), "argocd_flushed_total")
}

func TestInitMeter_HonorsStandardOTLPEnv(t *testing.T) {
	serverCert, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
	addr, collector := startMetricsCollector(t, grpc.Creds(credentials.NewServerTLSFromCert(&serverCert)))
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", caPath)
	t.Setenv("OTEL_EXPORTER_OTLP_HEADERS", "authorization=Bearer from-env")

	closer, err := InitMeter(t.Context(), "argocd-test", addr, false, nil, nil,
		50*time.Millisecond, newCounterRegistry(t, "argocd_env_total"))
	require.NoError(t, err)
	t.Cleanup(closer)

	assert.Equal(t, []string{"Bearer from-env"}, collector.next(t).md.Get("authorization"))
}

func TestInitMeter_UsesCAFromTLSCertsConfigMap(t *testing.T) {
	serverCert, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
	addr, collector := startMetricsCollector(t, grpc.Creds(credentials.NewServerTLSFromCert(&serverCert)))
	// argocd-tls-certs-cm is mounted as one file per hostname.
	tlsDataPath := t.TempDir()
	ca, err := os.ReadFile(caPath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(tlsDataPath, "127.0.0.1"), ca, 0o600))
	t.Setenv("ARGOCD_TLS_DATA_PATH", tlsDataPath)
	// Read only by the trace exporter, so it must not disable the ConfigMap CA here.
	t.Setenv("OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE", "/nonexistent/ca.crt")

	closer, err := InitMeter(t.Context(), "argocd-test", addr, false, nil, nil,
		50*time.Millisecond, newCounterRegistry(t, "argocd_cm_ca_total"))
	require.NoError(t, err)
	t.Cleanup(closer)

	assert.Contains(t, metricNames(collector.next(t).req), "argocd_cm_ca_total")
}

func TestInitMeter_SecureIgnoresInsecureEnv(t *testing.T) {
	addr, collector := startMetricsCollector(t) // plaintext
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")

	closer, err := InitMeter(t.Context(), "argocd-test", addr, false, nil, nil,
		50*time.Millisecond, newCounterRegistry(t, "argocd_downgrade_total"))
	require.NoError(t, err)
	t.Cleanup(closer)

	select {
	case <-collector.calls:
		t.Fatal("otlp.insecure=false must not be downgraded to plaintext by OTEL_EXPORTER_OTLP_INSECURE")
	case <-time.After(time.Second):
	}
}

func TestInitMeter_MutualTLS(t *testing.T) {
	for name, prefix := range map[string]string{
		"generic vars": "OTEL_EXPORTER_OTLP_",
		"metrics vars": "OTEL_EXPORTER_OTLP_METRICS_",
	} {
		t.Run(name, func(t *testing.T) {
			serverCert, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
			_, clientCertPath, clientKeyPath, clientCAs := selfSignedCert(t, x509.ExtKeyUsageClientAuth)
			addr, collector := startMetricsCollector(t, grpc.Creds(credentials.NewTLS(&tls.Config{
				Certificates: []tls.Certificate{serverCert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    clientCAs,
			})))
			t.Setenv(prefix+"CERTIFICATE", caPath)
			t.Setenv(prefix+"CLIENT_CERTIFICATE", clientCertPath)
			t.Setenv(prefix+"CLIENT_KEY", clientKeyPath)

			closer, err := InitMeter(t.Context(), "argocd-test", addr, false, nil, nil,
				50*time.Millisecond, newCounterRegistry(t, "argocd_mtls_total"))
			require.NoError(t, err)
			t.Cleanup(closer)

			assert.Contains(t, metricNames(collector.next(t).req), "argocd_mtls_total")
		})
	}
}

func TestInitMeter_InvalidTLSEnvFails(t *testing.T) {
	_, clientCertPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageClientAuth)
	// Without credentials, this would let the exporter fall back to plaintext.
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	t.Setenv("OTEL_EXPORTER_OTLP_METRICS_CLIENT_CERTIFICATE", clientCertPath)

	_, err := InitMeter(t.Context(), "argocd-test", "127.0.0.1:4317", false, nil, nil,
		time.Second, newCounterRegistry(t, "argocd_invalid_tls_total"))
	assert.Error(t, err)
}

func TestInitMeter_InsecureIgnoresTLSEnv(t *testing.T) {
	addr, collector := startMetricsCollector(t) // plaintext
	_, caPath, _, _ := selfSignedCert(t, x509.ExtKeyUsageServerAuth)
	t.Setenv("OTEL_EXPORTER_OTLP_CERTIFICATE", caPath)

	closer, err := InitMeter(t.Context(), "argocd-test", addr, true, nil, nil,
		50*time.Millisecond, newCounterRegistry(t, "argocd_insecure_total"))
	require.NoError(t, err)
	t.Cleanup(closer)

	assert.Contains(t, metricNames(collector.next(t).req), "argocd_insecure_total")
}
