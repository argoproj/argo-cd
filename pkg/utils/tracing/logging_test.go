package tracing

import (
	"testing"

	"github.com/golang/mock/gomock"

	"github.com/argoproj/gitops-engine/pkg/utils/tracing/tracer_testing"
)

func TestLoggingTracer(t *testing.T) {
	c := gomock.NewController(t)
	l := tracer_testing.NewMockLogger(c)
	gomock.InOrder(
		l.EXPECT().WithValues("my-key", "my-value").Return(l),
		l.EXPECT().WithValues("operation_name", "my-operation", "time_ms", gomock.Any()).Return(l),
		l.EXPECT().Info("Trace"),
	)

	tr := NewLoggingTracer(l)

	span := tr.StartSpan("my-operation")
	span.SetBaggageItem("my-key", "my-value")
	span.Finish()
}
