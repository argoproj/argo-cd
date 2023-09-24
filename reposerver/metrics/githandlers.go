package metrics

import (
	"context"
	"math"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/argoproj/argo-cd/v2/util/env"
	"github.com/argoproj/argo-cd/v2/util/git"
)

var (
	lsRemoteParallelismLimit          = env.ParseInt64FromEnv("ARGOCD_GIT_LS_REMOTE_PARALLELISM_LIMIT", 0, 0, math.MaxInt64)
	lsRemoteParallelismLimitSemaphore *semaphore.Weighted
)

func init() {
	if lsRemoteParallelismLimit > 0 {
		lsRemoteParallelismLimitSemaphore = semaphore.NewWeighted(lsRemoteParallelismLimit)
	}
}

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
			if lsRemoteParallelismLimitSemaphore != nil {
				_ = lsRemoteParallelismLimitSemaphore.Acquire(context.Background(), 1)
			}
			return func() {
				if lsRemoteParallelismLimitSemaphore != nil {
					lsRemoteParallelismLimitSemaphore.Release(1)
				}
				metricsServer.ObserveGitRequestDuration(repo, GitRequestTypeLsRemote, time.Since(startTime))
			}
		},
	}
}
