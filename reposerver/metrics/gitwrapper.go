package metrics

import (
	"context"

	"github.com/argoproj/argo-cd/util/git"
)

type gitClientWrapper struct {
	repo          string
	client        git.Client
	metricsServer *MetricsServer
}

func WrapGitClient(repo string, metricsServer *MetricsServer, client git.Client) git.Client {
	return &gitClientWrapper{repo: repo, client: client, metricsServer: metricsServer}
}

func (w *gitClientWrapper) Fetch(ctx context.Context) error {
	w.metricsServer.IncGitRequest(w.repo, GitRequestTypeFetch)
	return w.client.Fetch(ctx)
}

func (w *gitClientWrapper) LsRemote(revision string) (string, error) {
	sha, err := w.client.LsRemote(revision)
	if sha != revision {
		// This is true only if specified revision is a tag, branch or HEAD and client had to use 'ls-remote'
		w.metricsServer.IncGitRequest(w.repo, GitRequestTypeLsRemote)
	}
	return sha, err
}

func (w *gitClientWrapper) LsFiles(ctx context.Context, path string) ([]string, error) {
	return w.client.LsFiles(ctx, path)
}

func (w *gitClientWrapper) LsLargeFiles(ctx context.Context) ([]string, error) {
	return w.client.LsLargeFiles(ctx)
}

func (w *gitClientWrapper) Checkout(ctx context.Context, revision string) error {
	return w.client.Checkout(ctx, revision)
}

func (w *gitClientWrapper) CommitSHA(ctx context.Context) (string, error) {
	return w.client.CommitSHA(ctx)
}

func (w *gitClientWrapper) Root() string {
	return w.client.Root()
}

func (w *gitClientWrapper) Init(ctx context.Context) error {
	return w.client.Init(ctx)
}

func (w *gitClientWrapper) RevisionMetadata(ctx context.Context, revision string) (*git.RevisionMetadata, error) {
	return w.client.RevisionMetadata(ctx, revision)
}
