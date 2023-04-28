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
	repositoriesDB      RepositoryDB
	storecreds          git.CredsStore
	submoduleEnabled    bool
	repoServerClientSet repoapiclient.Clientset
}

type Repos interface {

	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error)
}

func NewArgoCDService(db db.ArgoDB, submoduleEnabled bool, repoClientset repoapiclient.Clientset) (Repos, error) {
	return &argoCDService{
		repositoriesDB:      db.(RepositoryDB),
		submoduleEnabled:    submoduleEnabled,
		repoServerClientSet: repoClientset,
	}, nil
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL string, revision string, pattern string) (map[string][]byte, error) {
	repo, err := a.repositoriesDB.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	fileRequest := &apiclient.GitFilesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		Path:             pattern,
	}
	closer, client, err := a.repoServerClientSet.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer io.Close(closer)

	fileResponse, err := client.GetGitFiles(ctx, fileRequest)
	if err != nil {
		return nil, err
	}
	return fileResponse.GetMap(), nil
}

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL string, revision string) ([]string, error) {
	repo, err := a.repositoriesDB.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
	}

	closer, client, err := a.repoServerClientSet.NewRepoServerClient()
	if err != nil {
		return nil, err
	}
	defer io.Close(closer)

	dirResponse, err := client.GetGitDirectories(ctx, dirRequest)
	if err != nil {
		return nil, err
	}
	return dirResponse.GetPaths(), nil

}
