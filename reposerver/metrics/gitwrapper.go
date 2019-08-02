package metrics

import "github.com/argoproj/argo-cd/util/git"

type gitClientWrapper struct {
	repo          string
	client        git.Client
	metricsServer *MetricsServer
}

func wrapGitClient(repo string, metricsServer *MetricsServer, client git.Client) git.Client {
	return &gitClientWrapper{repo: repo, client: client, metricsServer: metricsServer}
}

func (w *gitClientWrapper) Fetch() error {
	w.metricsServer.IncGitRequest(w.repo, GitRequestTypeFetch)
	return w.client.Fetch()
}

func (w *gitClientWrapper) LsRemote(revision string) (string, error) {
	sha, err := w.client.LsRemote(revision)
	if sha != revision {
		// This is true only if specified revision is a tag, branch or HEAD and client had to use 'ls-remote'
		w.metricsServer.IncGitRequest(w.repo, GitRequestTypeLsRemote)
	}
	return sha, err
}

func (w *gitClientWrapper) LsFiles(path string) ([]string, error) {
	return w.client.LsFiles(path)
}

func (w *gitClientWrapper) LsLargeFiles() ([]string, error) {
	return w.client.LsLargeFiles()
}

func (w *gitClientWrapper) Checkout(revision string) error {
	return w.client.Checkout(revision)
}

func (w *gitClientWrapper) CommitSHA() (string, error) {
	return w.client.CommitSHA()
}

func (w *gitClientWrapper) Root() string {
	return w.client.Root()
}

func (w *gitClientWrapper) Init() error {
	return w.client.Init()
}

func (w *gitClientWrapper) RevisionMetadata(revision string) (*git.RevisionMetadata, error) {
	return w.client.RevisionMetadata(revision)
}
