package tracing

import (
	"testing"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/utils/tracing/mocks"
)

func TestLoggingTracer(t *testing.T) {
	l := mocks.NewLogSink(t)
	initCall := l.EXPECT().Init(mock.Anything).Return().Once()
	withBaggageCall := l.EXPECT().WithValues("my-key", "my-value").Return(l).Once()
	withOperationCall := l.EXPECT().WithValues("operation_name", "my-operation", "time_ms", mock.Anything).Return(l).Once()
	enabledCall := l.EXPECT().Enabled(mock.Anything).Return(true).Once()
	infoCall := l.EXPECT().Info(0, "Trace").Return().Once()
	mock.InOrder(initCall, withBaggageCall, withOperationCall, enabledCall, infoCall)

	tr := NewLoggingTracer(logr.New(l))

	span := tr.StartSpan("my-operation")
	span.SetBaggageItem("my-key", "my-value")
	span.Finish()
}
