package v1alpha1

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
)

func TestK8sClientQPSFromEnv(t *testing.T) {
	tests := []struct {
		name          string
		qps           string
		burst         string
		expectedQPS   float32
		expectedBurst int
	}{
		{name: "defaults", expectedQPS: 50, expectedBurst: 100},
		{name: "custom", qps: "200", expectedQPS: 200, expectedBurst: 400},
		{name: "custom burst", qps: "200", burst: "300", expectedQPS: 200, expectedBurst: 300},
		{name: "minus one disables rate limiting", qps: "-1", expectedQPS: -1, expectedBurst: 0},
		{name: "below minus one falls back to default", qps: "-2", expectedQPS: 50, expectedBurst: 100},
		{name: "unparsable falls back to default", qps: "fast", expectedQPS: 50, expectedBurst: 100},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(EnvK8sClientQPS, tt.qps)
			t.Setenv(EnvK8sClientBurst, tt.burst)
			qps := k8sClientQPSFromEnv()
			assert.InDelta(t, tt.expectedQPS, qps, 0.0001)
			assert.Equal(t, tt.expectedBurst, k8sClientBurstFromEnv(qps))
		})
	}
}

func TestNegativeQPSDisablesClientSideRateLimiter(t *testing.T) {
	// Guards the assumption behind allowing -1: client-go builds no rate limiter for QPS < 0
	cfg := &rest.Config{
		Host:          "https://localhost:6443",
		QPS:           -1,
		ContentConfig: rest.ContentConfig{GroupVersion: &corev1.SchemeGroupVersion, NegotiatedSerializer: scheme.Codecs.WithoutConversion()},
	}
	client, err := rest.RESTClientFor(cfg)
	require.NoError(t, err)
	assert.Nil(t, client.GetRateLimiter(), "client-go should not create a rate limiter for negative QPS")
}
