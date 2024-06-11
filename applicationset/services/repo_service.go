package services

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/io"
)

type argoCDService struct {
	getRepository          func(ctx context.Context, url, project string) (*v1alpha1.Repository, error)
	storecreds             git.CredsStore
	submoduleEnabled       bool
	repoServerClientSet    apiclient.Clientset
	newFileGlobbingEnabled bool
}

//go:generate go run github.com/vektra/mockery/v2@v2.40.2 --name=Repos

type Repos interface {

	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL string, revision string, pattern string, noRevisionCache bool) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL string, revision string, noRevisionCache bool) ([]string, error)
}

func NewArgoCDService(getRepository func(ctx context.Context, url, project string) (*v1alpha1.Repository, error), submoduleEnabled bool, repoClientset apiclient.Clientset, newFileGlobbingEnabled bool) (Repos, error) {
	return &argoCDService{
		getRepository:          getRepository,
		submoduleEnabled:       submoduleEnabled,
		repoServerClientSet:    repoClientset,
		newFileGlobbingEnabled: newFileGlobbingEnabled,
	}, nil
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL string, revision string, pattern string, noRevisionCache bool) (map[string][]byte, error) {
	repo, err := a.getRepository(ctx, repoURL, "")
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	fileRequest := &apiclient.GitFilesRequest{
		Repo:                      repo,
		SubmoduleEnabled:          a.submoduleEnabled,
		Revision:                  revision,
		Path:                      pattern,
		NewGitFileGlobbingEnabled: a.newFileGlobbingEnabled,
		NoRevisionCache:           noRevisionCache,
	}
	closer, client, err := a.repoServerClientSet.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error initialising new repo server client: %w", err)
	}
	defer io.Close(closer)

	fileResponse, err := client.GetGitFiles(ctx, fileRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git files: %w", err)
	}
	return fileResponse.GetMap(), nil
}

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL string, revision string, noRevisionCache bool) ([]string, error) {
	repo, err := a.getRepository(ctx, repoURL, "")
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		NoRevisionCache:  noRevisionCache,
	}

	closer, client, err := a.repoServerClientSet.NewRepoServerClient()
	if err != nil {
		return nil, fmt.Errorf("error initialising new repo server client: %w", err)
	}
	defer io.Close(closer)

	dirResponse, err := client.GetGitDirectories(ctx, dirRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git Directories: %w", err)
	}
	return dirResponse.GetPaths(), nil

}
