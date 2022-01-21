package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	handler                  http.Handler
	gitRequestCounter        *prometheus.CounterVec
	gitRequestHistogram      *prometheus.HistogramVec
	repoPendingRequestsGauge *prometheus.GaugeVec
	redisRequestCounter      *prometheus.CounterVec
	redisRequestHistogram    *prometheus.HistogramVec
}

type GitRequestType string

const (
	GitRequestTypeLsRemote = "ls-remote"
	GitRequestTypeFetch    = "fetch"
)

// NewMetricsServer returns a new prometheus server which collects application metrics.
func NewMetricsServer() *MetricsServer {
	registry := prometheus.NewRegistry()
	handler := promhttp.HandlerFor(prometheus.Gatherers{
		registry,
		// contains process, golang and grpc server metrics
		prometheus.DefaultGatherer,
	}, promhttp.HandlerOpts{})

	return &MetricsServer{
		handler: handler,
		gitRequestCounter: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "argocd_git_request_total",
				Help: "Number of git requests performed by repo server",
			},
			[]string{"repo", "request_type"},
		),
		gitRequestHistogram: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "argocd_git_request_duration_seconds",
				Help:    "Git requests duration seconds.",
				Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
			},
			[]string{"repo", "request_type"},
		),
		repoPendingRequestsGauge: promauto.With(registry).NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "argocd_repo_pending_request_total",
				Help: "Number of pending requests requiring repository lock",
			},
			[]string{"repo"},
		),
		redisRequestCounter: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "argocd_redis_request_total",
				Help: "Number of kubernetes requests executed during application reconciliation.",
			},
			[]string{"initiator", "failed"},
		),
		redisRequestHistogram: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "argocd_redis_request_duration_seconds",
				Help:    "Redis requests duration seconds.",
				Buckets: []float64{0.1, 0.25, .5, 1, 2},
			},
			[]string{"initiator"},
		),
	}
}

func (m *MetricsServer) GetHandler() http.Handler {
	return m.handler
}

// IncGitRequest increments the git requests counter
func (m *MetricsServer) IncGitRequest(repo string, requestType GitRequestType) {
	m.gitRequestCounter.WithLabelValues(repo, string(requestType)).Inc()
}

func (m *MetricsServer) IncPendingRepoRequest(repo string) {
	m.repoPendingRequestsGauge.WithLabelValues(repo).Inc()
}

func (m *MetricsServer) ObserveGitRequestDuration(repo string, requestType GitRequestType, duration time.Duration) {
	m.gitRequestHistogram.WithLabelValues(repo, string(requestType)).Observe(duration.Seconds())
}

func (m *MetricsServer) DecPendingRepoRequest(repo string) {
	m.repoPendingRequestsGauge.WithLabelValues(repo).Dec()
}

func (m *MetricsServer) IncRedisRequest(failed bool) {
	m.redisRequestCounter.WithLabelValues("argocd-repo-server", strconv.FormatBool(failed)).Inc()
}

func (m *MetricsServer) ObserveRedisRequestDuration(duration time.Duration) {
	m.redisRequestHistogram.WithLabelValues("argocd-repo-server").Observe(duration.Seconds())
}
