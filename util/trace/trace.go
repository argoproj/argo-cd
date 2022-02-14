package trace

import (
	"io"

	"github.com/opentracing/opentracing-go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"github.com/uber/jaeger-client-go"
	"github.com/uber/jaeger-client-go/config"
	jaeger_prometheus "github.com/uber/jaeger-lib/metrics/prometheus"
)

func InitTracer() (io.Closer, error) {
	var cfg *config.Configuration
	cfg, err := config.FromEnv()
	if err != nil {
		return nil, err
	}
	cfg.Gen128Bit = true
	cfg.Disabled = false
	cfg.Headers = new(jaeger.HeadersConfig)
	cfg.Headers.ApplyDefaults()
	tracer, closer, err := cfg.NewTracer(
		config.Metrics(jaeger_prometheus.New(jaeger_prometheus.WithRegisterer(prometheus.DefaultRegisterer))),
		config.Logger(&jaegerLogger{logrus.New()}),
	)
	opentracing.SetGlobalTracer(tracer)
	return closer, err
}

type jaegerLogger struct {
	*logrus.Logger
}

func (l *jaegerLogger) Error(msg string) {
	l.Logger.Error("msg", msg)
}
