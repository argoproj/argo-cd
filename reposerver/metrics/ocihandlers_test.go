package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

func TestOCIClientEventHandlers(t *testing.T) {
	tests := []struct {
		name     string
		setup    func()
		teardown func()
		testFunc func(t *testing.T)
	}{
		{
			name: "test event handlers",
			testFunc: func(t *testing.T) {
				t.Helper()
				assert.NotPanics(t, func() {
					metricsServer := NewMetricsServer()
					eventHandlers := NewOCIClientEventHandlers(metricsServer)
					eventHandlers.OnExtract("test")
					eventHandlers.OnTestRepo("test")
					eventHandlers.OnGetTags("test")
					eventHandlers.OnResolveRevision("test")
					eventHandlers.OnDigestMetadata("test")
					c := metricsServer.ociRequestCounter
					assert.Equal(t, 5, testutil.CollectAndCount(c))
					assert.InDelta(t, float64(1), testutil.ToFloat64(c.WithLabelValues("test", OCIRequestTypeExtract)), 0.01)
					assert.InDelta(t, float64(1), testutil.ToFloat64(c.WithLabelValues("test", OCIRequestTypeResolveRevision)), 0.01)
					assert.InDelta(t, float64(1), testutil.ToFloat64(c.WithLabelValues("test", OCIRequestTypeDigestMetadata)), 0.01)
					assert.InDelta(t, float64(1), testutil.ToFloat64(c.WithLabelValues("test", OCIRequestTypeTestRepo)), 0.01)
					assert.InDelta(t, float64(1), testutil.ToFloat64(c.WithLabelValues("test", OCIRequestTypeTestRepo)), 0.01)
				})
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			if tt.teardown != nil {
				defer tt.teardown()
			}
			tt.testFunc(t)
		})
	}
}
