package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Server struct {
	handler                    http.Handler
	commitPendingRequestsGauge *prometheus.GaugeVec
}

// NewMetricsServer returns a new prometheus server which collects application metrics.
func NewMetricsServer() *Server {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	commitPendingRequestsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argocd_commitserver_commit_pending_request_total",
			Help: "Number of pending commit requests",
		},
		[]string{"repo"},
	)
	registry.MustRegister(commitPendingRequestsGauge)

	return &Server{
		handler:                    promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		commitPendingRequestsGauge: commitPendingRequestsGauge,
	}
}

func (m *Server) GetHandler() http.Handler {
	return m.handler
}

func (m *Server) IncPendingCommitRequest(repo string) {
	m.commitPendingRequestsGauge.WithLabelValues(repo).Inc()
}

func (m *Server) DecPendingCommitRequest(repo string) {
	m.commitPendingRequestsGauge.WithLabelValues(repo).Dec()
}
