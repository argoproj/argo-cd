package trace

import (
	"testing"

	"github.com/stretchr/testify/assert"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestNewSampler(t *testing.T) {
	tests := []struct {
		name        string
		ratio       float64
		description string
	}{
		{
			name:        "ratio of 1.0 always samples",
			ratio:       1.0,
			description: sdktrace.AlwaysSample().Description(),
		},
		{
			name:        "ratio above 1.0 always samples",
			ratio:       2.0,
			description: sdktrace.AlwaysSample().Description(),
		},
		{
			name:        "ratio of 0.0 never samples",
			ratio:       0.0,
			description: sdktrace.NeverSample().Description(),
		},
		{
			name:        "negative ratio never samples",
			ratio:       -1.0,
			description: sdktrace.NeverSample().Description(),
		},
		{
			name:        "fractional ratio uses parent-based ratio sampler",
			ratio:       0.5,
			description: sdktrace.ParentBased(sdktrace.TraceIDRatioBased(0.5)).Description(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sampler := newSampler(tt.ratio)
			assert.Equal(t, tt.description, sampler.Description())
		})
	}
}
