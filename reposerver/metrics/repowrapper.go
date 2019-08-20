package metrics

import (
	"github.com/argoproj/argo-cd/util/repo"
)

type repoWrapper struct {
	url           string
	repo          repo.Repo
	metricsServer *MetricsServer
}

func wrapRepo(repo string, metricsServer *MetricsServer, client repo.Repo) repo.Repo {
	return &repoWrapper{url: repo, repo: client, metricsServer: metricsServer}
}

func (w *repoWrapper) Init() error {
	return w.repo.Init()
}

func (w *repoWrapper) LockKey() string {
	return w.repo.LockKey()
}

func (w *repoWrapper) ListApps(revision string) (apps map[string]string, resolvedRevision string, err error) {
	return w.repo.ListApps(revision)
}

func (w *repoWrapper) ResolveRevision(app, revision string) (resolvedRevision string, err error) {
	return w.repo.ResolveRevision(app, revision)
}

func (w *repoWrapper) GetApp(app, revision string) (path string, err error) {
	return w.repo.GetApp(app, revision)
}

func (w *repoWrapper) RevisionMetadata(app, revision string) (*repo.RevisionMetadata, error) {
	return w.repo.RevisionMetadata(app, revision)
}
