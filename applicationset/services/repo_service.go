package services

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	repoapiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/io"
)

// RepositoryDB Is a lean facade for ArgoDB,
// Using a lean interface makes it easier to test the functionality of the git generator
type RepositoryDB interface {
	GetRepository(ctx context.Context, url string) (*v1alpha1.Repository, error)
}

type argoCDService struct {
	repositoriesDB   RepositoryDB
	storecreds       git.CredsStore
	submoduleEnabled bool
	repoServerClient apiclient.RepoServerServiceClient
	closer           io.Closer
}

type Repos interface {

	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error)

	// Close any open connections
	Close()
}

func NewArgoCDService(db db.ArgoDB, gitCredStore git.CredsStore, submoduleEnabled bool, repoClientset repoapiclient.Clientset) (Repos, error) {
	closer, repoClient, err := repoClientset.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	return &argoCDService{
		repositoriesDB:   db.(RepositoryDB),
		storecreds:       gitCredStore,
		submoduleEnabled: submoduleEnabled,
		repoServerClient: repoClient,
		closer:           closer,
	}, nil
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error) {
	repo, err := a.repositoriesDB.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("Error in GetRepository: %w", err)
	}

	fileRequest := &apiclient.GitFilesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		Path:             pattern,
	}

	fileResponse, err := a.repoServerClient.GetGitFiles(ctx, fileRequest)
	if err != nil {
		return nil, err
	}
	return fileResponse.GetMap(), nil
}

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error) {

	repo, err := a.repositoriesDB.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("Error in GetRepository: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
	}

	dirResponse, err := a.repoServerClient.GetGitDirectories(ctx, dirRequest)
	if err != nil {
		return nil, err
	}
	return dirResponse.GetPaths(), nil

}

func (a *argoCDService) Close() {
	io.Close(a.closer)
}

func checkoutRepo(gitRepoClient git.Client, revision string, submoduleEnabled bool) error {
	err := gitRepoClient.Init()
	if err != nil {
		return fmt.Errorf("Error during initializing repo: %w", err)
	}

	err = gitRepoClient.Fetch(revision)
	if err != nil {
		return fmt.Errorf("Error during fetching repo: %w", err)
	}

	commitSHA, err := gitRepoClient.LsRemote(revision)
	if err != nil {
		return fmt.Errorf("Error during fetching commitSHA: %w", err)
	}
	err = gitRepoClient.Checkout(commitSHA, submoduleEnabled)
	if err != nil {
		return fmt.Errorf("Error during repo checkout: %w", err)
	}
	return nil
}
