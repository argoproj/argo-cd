package tracing

import (
	"time"

	"github.com/go-logr/logr"
)

var (
	_ Tracer = LoggingTracer{}
	_ Span   = loggingSpan{}
)

type LoggingTracer struct {
	logger logr.Logger
}

func NewLoggingTracer(logger logr.Logger) *LoggingTracer {
	return &LoggingTracer{
		logger: logger,
	}
}

func (l LoggingTracer) StartSpan(operationName string) Span {
	return loggingSpan{
		logger:        l.logger,
		operationName: operationName,
		baggage:       make(map[string]any),
		start:         time.Now(),
	}
}

type loggingSpan struct {
	logger        logr.Logger
	operationName string
	baggage       map[string]any
	start         time.Time
}

func (s loggingSpan) Finish() {
	s.logger.WithValues(baggageToVals(s.baggage)...).
		WithValues("operation_name", s.operationName, "time_ms", time.Since(s.start).Seconds()*1e3).
		Info("Trace")
}

func (s loggingSpan) SetBaggageItem(key string, value any) {
	s.baggage[key] = value
}

func baggageToVals(baggage map[string]any) []any {
	result := make([]any, 0, len(baggage)*2)
	for k, v := range baggage {
		result = append(result, k, v)
	}
	return result
}
