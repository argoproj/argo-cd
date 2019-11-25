package tracer

import (
	"os"
	"time"

	"github.com/opentracing/opentracing-go"
	log "github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	jaegerlog "github.com/uber/jaeger-client-go/log"
	"github.com/uber/jaeger-lib/metrics"
)

var Tracer opentracing.Tracer

func init() () {
	serviceName := os.Getenv("JAEGER_SERVICE_NAME")

	if serviceName == "" {
		serviceName = "unnamed"
	}

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
	tracer, _, err := cfg.NewTracer(
		jaegercfg.Logger(jaegerlog.StdLogger),
		jaegercfg.Metrics(metrics.NewLocalFactory(10*time.Second)),
	)
	if err != nil {
		log.Fatal(err)
	}
	opentracing.SetGlobalTracer(tracer)
	Tracer = tracer
	log.Infof("tracing enabled for %s", serviceName)
}
