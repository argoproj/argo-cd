package trace

import (
	"context"
	"errors"
	"net"
	"os"
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

func startCollector(t *testing.T) (string, *fakeCollector) {
	t.Helper()
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	c := &fakeCollector{calls: make(chan exportCall, 64)}
	srv := grpc.NewServer()
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
	addr, collector := startCollector(t)
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
	addr, collector := startCollector(t)
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
	addr, collector := startCollector(t)
	ctx, cancel := context.WithCancel(t.Context())

	// Interval outlives the test: the only export is the closer's flush.
	closer, err := InitMeter(ctx, "argocd-test", addr, true, nil, nil,
		time.Hour, newCounterRegistry(t, "argocd_flushed_total"))
	require.NoError(t, err)

	cancel()
	closer()

	assert.Contains(t, metricNames(collector.next(t).req), "argocd_flushed_total")
}
