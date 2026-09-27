package auditcontroller

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/argoproj/argo-cd/v3/util/audit"
)

// Metrics are the Prometheus metrics of the audit controller.
type Metrics struct {
	registry        *prometheus.Registry
	records         *prometheus.CounterVec
	writeErrors     prometheus.Counter
	ingestRejected  *prometheus.CounterVec
	eventsProcessed *prometheus.CounterVec
}

// NewMetrics creates and registers the audit controller metrics.
func NewMetrics() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		records: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "argocd_audit_records_total",
			Help: "Number of audit records written, by source, action and result.",
		}, []string{"source", "action", "result"}),
		writeErrors: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "argocd_audit_record_write_errors_total",
			Help: "Number of audit records that could not be written to every output.",
		}),
		ingestRejected: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "argocd_audit_ingest_rejected_total",
			Help: "Number of ingest requests rejected, by reason.",
		}, []string{"reason"}),
		eventsProcessed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "argocd_audit_kubernetes_events_total",
			Help: "Number of Kubernetes Events seen by the audit controller, by outcome.",
		}, []string{"outcome"}),
	}
	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.records, m.writeErrors, m.ingestRejected, m.eventsProcessed,
	)
	return m
}

// Handler serves the metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) recordWritten(rec audit.Record, writeErr error) {
	m.records.WithLabelValues(rec.Source, rec.Action, rec.Result.Status).Inc()
	if writeErr != nil {
		m.writeErrors.Inc()
	}
}

func (m *Metrics) ingestRejectedInc(reason string) {
	if m != nil {
		m.ingestRejected.WithLabelValues(reason).Inc()
	}
}

func (m *Metrics) eventProcessed(outcome string) {
	if m != nil {
		m.eventsProcessed.WithLabelValues(outcome).Inc()
	}
}
