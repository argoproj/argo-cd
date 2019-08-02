package metrics

import (
	"net/http"

	"github.com/argoproj/argo-cd/util/git"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type MetricsServer struct {
	handler           http.Handler
	gitRequestCounter *prometheus.CounterVec
	gitClientFactory  git.ClientFactory
}

type GitRequestType string

const (
	GitRequestTypeLsRemote = "ls-remote"
	GitRequestTypeFetch    = "fetch"
)

// NewMetricsServer returns a new prometheus server which collects application metrics
func NewMetricsServer(gitClientFactory git.ClientFactory) *MetricsServer {
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
		gitClientFactory:  gitClientFactory,
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

func (m *MetricsServer) NewClient(repoURL string, path string, creds git.Creds, insecureIgnoreHostKey bool, lfsEnabled bool) (git.Client, error) {
	client, err := m.gitClientFactory.NewClient(repoURL, path, creds, insecureIgnoreHostKey, lfsEnabled)
	if err != nil {
		return nil, err
	}
	return wrapGitClient(repoURL, m, client), nil
}
