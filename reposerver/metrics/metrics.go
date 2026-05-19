package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	handler                       http.Handler
	gitFetchFailCounter           *prometheus.CounterVec
	gitLsRemoteFailCounter        *prometheus.CounterVec
	gitRequestCounter             *prometheus.CounterVec
	gitRequestHistogram           *prometheus.HistogramVec
	repoPendingRequestsGauge      *prometheus.GaugeVec
	redisRequestCounter           *prometheus.CounterVec
	redisRequestHistogram         *prometheus.HistogramVec
	ociExtractFailCounter         *prometheus.CounterVec
	ociResolveRevisionFailCounter *prometheus.CounterVec
	ociDigestMetadataCounter      *prometheus.CounterVec
	ociGetTagsFailCounter         *prometheus.CounterVec
	ociTestRepoFailCounter        *prometheus.CounterVec
	ociRequestCounter             *prometheus.CounterVec
	ociRequestHistogram           *prometheus.HistogramVec
	PrometheusRegistry            *prometheus.Registry
}

type GitRequestType string

const (
	GitRequestTypeLsRemote = "ls-remote"
	GitRequestTypeFetch    = "fetch"
)

// NewMetricsServer returns a new prometheus server which collects application metrics.
func NewMetricsServer() *MetricsServer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	gitFetchFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_git_fetch_fail_total",
			Help: "Number of git fetch requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(gitFetchFailCounter)

	gitLsRemoteFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_git_lsremote_fail_total",
			Help: "Number of git ls-remote requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(gitLsRemoteFailCounter)

	gitRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_git_request_total",
			Help: "Number of git requests performed by repo server",
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestCounter)

	gitRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_git_request_duration_seconds",
			Help:    "Git requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestHistogram)

	repoPendingRequestsGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "argocd_repo_pending_request_total",
			Help: "Number of pending requests requiring repository lock",
		},
		[]string{"repo"},
	)
	registry.MustRegister(repoPendingRequestsGauge)

	redisRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_redis_request_total",
			Help: "Number of kubernetes requests executed during application reconciliation.",
		},
		[]string{"initiator", "failed"},
	)
	registry.MustRegister(redisRequestCounter)

	redisRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_redis_request_duration_seconds",
			Help:    "Redis requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2},
		},
		[]string{"initiator"},
	)
	registry.MustRegister(redisRequestHistogram)

	ociExtractFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_oci_extract_fail_total",
			Help: "Number of OCI extract requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(ociExtractFailCounter)

	ociResolveRevisionFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_oci_resolve_revision_fail_total",
			Help: "Number of OCI resolve revision requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(ociResolveRevisionFailCounter)

	ociDigestMetadataCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_oci_digest_metadata_fail_total",
			Help: "Number of OCI digest metadata requests failures by repo server",
		},
		[]string{"repo", "revision"},
	)
	registry.MustRegister(ociDigestMetadataCounter)

	ociGetTagsFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_oci_get_tags_fail_total",
			Help: "Number of OCI get tags failures by repo server",
		},
		[]string{"repo"},
	)
	registry.MustRegister(ociGetTagsFailCounter)

	ociTestRepoFailCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_oci_test_repo_fail_total",
			Help: "Number of OCI test repo requests failures by repo server",
		},
		[]string{"repo"},
	)
	registry.MustRegister(ociTestRepoFailCounter)

	ociRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_oci_request_total",
			Help: "Number of OCI requests performed by repo server",
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(ociRequestCounter)

	ociRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_oci_request_duration_seconds",
			Help:    "OCI requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(ociRequestHistogram)

	return &MetricsServer{
		handler:                       promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		gitFetchFailCounter:           gitFetchFailCounter,
		gitLsRemoteFailCounter:        gitLsRemoteFailCounter,
		gitRequestCounter:             gitRequestCounter,
		gitRequestHistogram:           gitRequestHistogram,
		repoPendingRequestsGauge:      repoPendingRequestsGauge,
		redisRequestCounter:           redisRequestCounter,
		redisRequestHistogram:         redisRequestHistogram,
		ociRequestCounter:             ociRequestCounter,
		ociRequestHistogram:           ociRequestHistogram,
		ociExtractFailCounter:         ociExtractFailCounter,
		ociResolveRevisionFailCounter: ociResolveRevisionFailCounter,
		ociGetTagsFailCounter:         ociGetTagsFailCounter,
		ociDigestMetadataCounter:      ociDigestMetadataCounter,
		ociTestRepoFailCounter:        ociTestRepoFailCounter,
		PrometheusRegistry:            registry,
	}
}

func (m *MetricsServer) GetHandler() http.Handler {
	return m.handler
}

func (m *MetricsServer) IncGitFetchFail(repo string, revision string) {
	m.gitFetchFailCounter.WithLabelValues(repo, revision).Inc()
}

func (m *MetricsServer) IncGitLsRemoteFail(repo string, revision string) {
	m.gitLsRemoteFailCounter.WithLabelValues(repo, revision).Inc()
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

// IncOCIRequest increments the OCI requests counter
func (m *MetricsServer) IncOCIRequest(repo string, requestType OCIRequestType) {
	m.ociRequestCounter.WithLabelValues(repo, string(requestType)).Inc()
}

func (m *MetricsServer) ObserveOCIRequestDuration(repo string, requestType OCIRequestType, duration time.Duration) {
	m.ociRequestHistogram.WithLabelValues(repo, string(requestType)).Observe(duration.Seconds())
}

// IncOCIExtractFailCounter increments the OCI failed extract requests counter
func (m *MetricsServer) IncOCIExtractFailCounter(repo string, revision string) {
	m.ociExtractFailCounter.WithLabelValues(repo, revision).Inc()
}

// IncOCIResolveRevisionFailCounter increments the OCI failed resolve revision requests counter
func (m *MetricsServer) IncOCIResolveRevisionFailCounter(repo string, revision string) {
	m.ociResolveRevisionFailCounter.WithLabelValues(repo, revision).Inc()
}

// IncOCIDigestMetadataCounter increments the OCI failed digest metadata requests counter
func (m *MetricsServer) IncOCIDigestMetadataCounter(repo string, revision string) {
	m.ociDigestMetadataCounter.WithLabelValues(repo, revision).Inc()
}

// IncOCIGetTagsFailCounter increments the OCI failed get tags requests counter
func (m *MetricsServer) IncOCIGetTagsFailCounter(repo string) {
	m.ociGetTagsFailCounter.WithLabelValues(repo).Inc()
}

// IncOCITestRepoFailCounter increments the OCI failed test repo requests counter
func (m *MetricsServer) IncOCITestRepoFailCounter(repo string) {
	m.ociTestRepoFailCounter.WithLabelValues(repo).Inc()
}
