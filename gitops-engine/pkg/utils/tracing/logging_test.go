package tracing

import (
	"testing"

	"github.com/go-logr/logr"
	"go.uber.org/mock/gomock"

	"github.com/argoproj/gitops-engine/pkg/utils/tracing/tracer_testing"
)

func TestLoggingTracer(t *testing.T) {
	c := gomock.NewController(t)
	l := tracer_testing.NewMockLogSink(c)
	gomock.InOrder(
		l.EXPECT().Init(gomock.Any()),
		l.EXPECT().WithValues("my-key", "my-value").Return(l),
		l.EXPECT().WithValues("operation_name", "my-operation", "time_ms", gomock.Any()).Return(l),
		l.EXPECT().Enabled(gomock.Any()).Return(true),
		l.EXPECT().Info(0, "Trace"),
	)

	tr := NewLoggingTracer(logr.New(l))

	span := tr.StartSpan("my-operation")
	span.SetBaggageItem("my-key", "my-value")
	span.Finish()
}
