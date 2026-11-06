package metrics

import (
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server is a prometheus server which collects application metrics.
type Server struct {
	handler                    http.Handler
	commitPendingRequestsGauge *prometheus.GaugeVec
	gitRequestCounter          *prometheus.CounterVec
	gitRequestHistogram        *prometheus.HistogramVec
	commitRequestHistogram     *prometheus.HistogramVec
	userInfoRequestHistogram   *prometheus.HistogramVec
	commitRequestCounter       *prometheus.CounterVec
	signingFailureCounter      *prometheus.CounterVec
}

// SigningFailureReason identifies why hydrated-commit signing did not succeed.
// Lets operators alert on "we are configured to sign but something is wrong"
// without having to grep logs.
type SigningFailureReason string

const (
	// SigningFailureReasonCommit indicates `git commit -S` itself failed.
	SigningFailureReasonCommit SigningFailureReason = "commit"
	// SigningFailureReasonVerify indicates the post-commit signature lookup failed.
	SigningFailureReasonVerify SigningFailureReason = "verify"
	// SigningFailureReasonBadStatus indicates the signature is missing or invalid.
	SigningFailureReasonBadStatus SigningFailureReason = "bad_status"
	// SigningFailureReasonWrongKey indicates the commit was signed by an unexpected key.
	SigningFailureReasonWrongKey SigningFailureReason = "wrong_key"
)

// GitRequestType is the type of git request
type GitRequestType string

const (
	// GitRequestTypeLsRemote is a request to list remote refs
	GitRequestTypeLsRemote = "ls-remote"
	// GitRequestTypeFetch is a request to fetch from remote
	GitRequestTypeFetch = "fetch"
	// GitRequestTypePush is a request to push to remote
	GitRequestTypePush = "push"
)

// CommitResponseType is the type of response for a commit request
type CommitResponseType string

const (
	// CommitResponseTypeSuccess is a successful commit request
	CommitResponseTypeSuccess CommitResponseType = "success"
	// CommitResponseTypeFailure is a failed commit request
	CommitResponseTypeFailure CommitResponseType = "failure"
)

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

	gitRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_commitserver_git_request_total",
			Help: "Number of git requests performed by repo server",
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestCounter)

	gitRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_commitserver_git_request_duration_seconds",
			Help:    "Git requests duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestHistogram)

	commitRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_commitserver_commit_request_duration_seconds",
			Help:    "Commit request duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "response_type"},
	)
	registry.MustRegister(commitRequestHistogram)

	userInfoRequestHistogram := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "argocd_commitserver_userinfo_request_duration_seconds",
			Help:    "Userinfo request duration seconds.",
			Buckets: []float64{0.1, 0.25, .5, 1, 2, 4, 10, 20},
		},
		[]string{"repo", "credential_type"},
	)
	registry.MustRegister(userInfoRequestHistogram)

	commitRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_commitserver_commit_request_total",
			Help: "Number of commit requests performed handled",
		},
		[]string{"repo", "response_type"},
	)
	registry.MustRegister(commitRequestCounter)

	signingFailureCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_commitserver_signing_failure_total",
			Help: "Number of hydrated-commit signing failures, by reason. A non-zero rate means signing is enabled but commits are not being pushed.",
		},
		[]string{"repo", "reason"},
	)
	registry.MustRegister(signingFailureCounter)

	return &Server{
		handler:                    promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		commitPendingRequestsGauge: commitPendingRequestsGauge,
		gitRequestCounter:          gitRequestCounter,
		gitRequestHistogram:        gitRequestHistogram,
		commitRequestHistogram:     commitRequestHistogram,
		userInfoRequestHistogram:   userInfoRequestHistogram,
		commitRequestCounter:       commitRequestCounter,
		signingFailureCounter:      signingFailureCounter,
	}
}

// IncSigningFailure increments the signing-failure counter for the given repo
// and reason. Use this only when hydrated-commit signing was attempted (i.e.
// the commit server is running with signing enabled).
func (m *Server) IncSigningFailure(repo string, reason SigningFailureReason) {
	m.signingFailureCounter.WithLabelValues(repo, string(reason)).Inc()
}

// GetHandler returns the http.Handler for the prometheus server
func (m *Server) GetHandler() http.Handler {
	return m.handler
}

// IncPendingCommitRequest increments the pending commit requests gauge
func (m *Server) IncPendingCommitRequest(repo string) {
	m.commitPendingRequestsGauge.WithLabelValues(repo).Inc()
}

// DecPendingCommitRequest decrements the pending commit requests gauge
func (m *Server) DecPendingCommitRequest(repo string) {
	m.commitPendingRequestsGauge.WithLabelValues(repo).Dec()
}

// IncGitRequest increments the git requests counter
func (m *Server) IncGitRequest(repo string, requestType GitRequestType) {
	m.gitRequestCounter.WithLabelValues(repo, string(requestType)).Inc()
}

// ObserveGitRequestDuration observes the duration of a git request
func (m *Server) ObserveGitRequestDuration(repo string, requestType GitRequestType, duration time.Duration) {
	m.gitRequestHistogram.WithLabelValues(repo, string(requestType)).Observe(duration.Seconds())
}

// ObserveCommitRequestDuration observes the duration of a commit request
func (m *Server) ObserveCommitRequestDuration(repo string, rt CommitResponseType, duration time.Duration) {
	m.commitRequestHistogram.WithLabelValues(repo, string(rt)).Observe(duration.Seconds())
}

// ObserveUserInfoRequestDuration observes the duration of a userinfo request
func (m *Server) ObserveUserInfoRequestDuration(repo string, credentialType string, duration time.Duration) {
	m.userInfoRequestHistogram.WithLabelValues(repo, credentialType).Observe(duration.Seconds())
}

// IncCommitRequest increments the commit request counter
func (m *Server) IncCommitRequest(repo string, rt CommitResponseType) {
	m.commitRequestCounter.WithLabelValues(repo, string(rt)).Inc()
}
