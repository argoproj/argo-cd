package tracer

import (
	"io"

	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

func Init(serviceName string) io.Closer {
	cfg := jaegercfg.Configuration{
		ServiceName: serviceName,
		Sampler: &jaegercfg.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},
		Reporter: &jaegercfg.ReporterConfig{
			LogSpans: true,
		},
	}
	tracer, closer, err := cfg.NewTracer(
		jaegercfg.Logger(jaegerlog.StdLogger),
		jaegercfg.Metrics(prometheus.New()),
	)
	if err != nil {
		log.Fatal(err)
	}
	opentracing.SetGlobalTracer(tracer)
	log.Debugf("tracing enabled for %s", serviceName)
	return closer
}
