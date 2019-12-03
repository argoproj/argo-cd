package tracer

import (
	"context"
	"fmt"
	"net/http"

	"github.com/opentracing/opentracing-go"
	"k8s.io/client-go/rest"
)

type tracingRoundTripper struct {
	ctx          context.Context
	roundTripper http.RoundTripper
}

func (rt tracingRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	span, _ := opentracing.StartSpanFromContext(rt.ctx, fmt.Sprintf("%v", r.URL))
	defer span.Finish()
	return rt.roundTripper.RoundTrip(r)
}

func SpanWrapper(ctx context.Context, config *rest.Config) *rest.Config {
	delegateWrap := config.WrapTransport
	config.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if delegateWrap != nil {
			rt = delegateWrap(rt)
		}
		return &tracingRoundTripper{ctx, rt}
	}
	return config
}
