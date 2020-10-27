package tracing

/*
	Poor Mans OpenTracing.

	Standardizes logging of operation duration.
*/

type Tracer interface {
	StartSpan(operationName string) Span
}

type Span interface {
	SetBaggageItem(key string, value interface{})
	Finish()
}
