package metrics

import (
	"github.com/argoproj/argo-cd/util/repo"
)

type clientWrapper struct {
	repo          string
	client        repo.Repo
	metricsServer *MetricsServer
}

func (w *clientWrapper) Test() error {
	return w.client.Test()
}

func wrapGitClient(repo string, metricsServer *MetricsServer, client repo.Repo) repo.Repo {
	return &clientWrapper{repo: repo, client: client, metricsServer: metricsServer}
}

func (w *clientWrapper) ResolveRevision(path, revision string) (string, error) {
	sha, err := w.client.ResolveRevision(path, revision)
	if sha != revision {
		// This is true only if specified revision is a tag, branch or HEAD and client had to use 'ls-remote'
		w.metricsServer.IncGitRequest(w.repo, GitRequestTypeLsRemote)
	}
	return sha, err
}

func (w *clientWrapper) LsFiles(path string) ([]string, error) {
	return w.client.LsFiles(path)
}

func (w *clientWrapper) Checkout(path, revision string) error {
	return w.client.Checkout(path, revision)
}

func (w *clientWrapper) Revision(path string) (string, error) {
	return w.client.Revision(path)
}

func (w *clientWrapper) Root() string {
	return w.client.Root()
}

func (w *clientWrapper) Init() error {
	return w.client.Init()
}

func (w *clientWrapper) RevisionMetadata(path, revision string) (*repo.RevisionMetadata, error) {
	return w.client.RevisionMetadata(path, revision)
}
