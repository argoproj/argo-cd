package metrics

import (
	"time"

	"github.com/argoproj/argo-cd/v2/util/git"
)

// NewGitClientEventHandlers creates event handlers that update Git related metrics
func NewGitClientEventHandlers(metricsServer *MetricsServer) git.EventHandlers {
	return git.EventHandlers{
		OnFetch: func(repo string) func() {
			startTime := time.Now()
			metricsServer.IncGitRequest(repo, GitRequestTypeFetch)
			return func() {
				metricsServer.ObserveGitRequestDuration(repo, GitRequestTypeFetch, time.Since(startTime))
			}
		},
		OnLsRemote: func(repo string) func() {
			startTime := time.Now()
			metricsServer.IncGitRequest(repo, GitRequestTypeLsRemote)
			return func() {
				metricsServer.ObserveGitRequestDuration(repo, GitRequestTypeLsRemote, time.Since(startTime))
			}
		},
	}
}
