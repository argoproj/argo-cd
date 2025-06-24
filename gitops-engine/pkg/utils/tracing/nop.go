package tracing

var (
	_ Tracer = NopTracer{}
	_ Span   = nopSpan{}
)

type NopTracer struct {
}

func (n NopTracer) StartSpan(operationName string) Span {
	return nopSpan{}
}

type nopSpan struct {
}

func (n nopSpan) SetBaggageItem(key string, value interface{}) {
}

func (n nopSpan) Finish() {
}
