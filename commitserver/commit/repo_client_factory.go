package commit

import (
	"github.com/argoproj/argo-cd/v2/commitserver/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
)

// RepoClientFactory is a factory for creating git clients for a repository.
type RepoClientFactory interface {
	NewClient(repo *v1alpha1.Repository, rootPath string) (git.Client, error)
}

type repoClientFactory struct {
	gitCredsStore git.CredsStore
	metricsServer *metrics.Server
}

// NewRepoClientFactory returns a new instance of the repo client factory.
func NewRepoClientFactory(gitCredsStore git.CredsStore, metricsServer *metrics.Server) RepoClientFactory {
	return &repoClientFactory{
		gitCredsStore: gitCredsStore,
		metricsServer: metricsServer,
	}
}

// NewClient creates a new git client for the repository.
func (r *repoClientFactory) NewClient(repo *v1alpha1.Repository, rootPath string) (git.Client, error) {
	gitCreds := repo.GetGitCreds(r.gitCredsStore)
	opts := git.WithEventHandlers(metrics.NewGitClientEventHandlers(r.metricsServer))
	return git.NewClientExt(repo.Repo, rootPath, gitCreds, repo.IsInsecure(), repo.IsLFSEnabled(), repo.Proxy, repo.NoProxy, opts)
}
