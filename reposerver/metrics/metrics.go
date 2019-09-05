package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/repo"
	"github.com/argoproj/argo-cd/util/repo/factory"
	"github.com/argoproj/argo-cd/util/repo/metrics"
)

type MetricsServer struct {
	handler           http.Handler
	gitRequestCounter *prometheus.CounterVec
	factory           factory.Factory
}

type GitRequestType string

const (
	GitRequestTypeLsRemote = "ls-remote"
	GitRequestTypeFetch    = "fetch"
)

// NewMetricsServer returns a new prometheus server which collects application metrics
func NewMetricsServer(factory factory.Factory) *MetricsServer {
	registry := prometheus.NewRegistry()
	registry.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
	registry.MustRegister(prometheus.NewGoCollector())

	gitRequestCounter := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "argocd_git_request_total",
			Help: "Number of git requests performed by repo server",
		},
		[]string{"repo", "request_type"},
	)
	registry.MustRegister(gitRequestCounter)

	return &MetricsServer{
		factory:           factory,
		handler:           promhttp.HandlerFor(registry, promhttp.HandlerOpts{}),
		gitRequestCounter: gitRequestCounter,
	}
}

func (m *MetricsServer) GetHandler() http.Handler {
	return m.handler
}

// IncGitRequest increments the git requests counter
func (m *MetricsServer) IncGitRequest(repo string, requestType GitRequestType) {
	m.gitRequestCounter.WithLabelValues(repo, string(requestType)).Inc()
}

func (m *MetricsServer) Event(repo string, requestType string) {
	switch requestType {
	case "GitRequestTypeLsRemote":
		m.IncGitRequest(repo, GitRequestTypeLsRemote)
	case "GitRequestTypeFetch":
		m.IncGitRequest(repo, GitRequestTypeFetch)
	}
}

func (m *MetricsServer) NewRepo(repo *v1alpha1.Repository, _ metrics.Reporter) (repo.Repo, error) {
	return m.factory.NewRepo(repo, m)
}
