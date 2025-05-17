package services

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"
	"github.com/argoproj/argo-cd/v3/util/io"
)

type argoCDService struct {
	getRepository                   func(ctx context.Context, url, project string) (*v1alpha1.Repository, error)
	submoduleEnabled                bool
	newFileGlobbingEnabled          bool
	getGitFilesFromRepoServer       func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error)
	getGitDirectoriesFromRepoServer func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error)
}

type Repos interface {
	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache, verifyCommit bool) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache, verifyCommit bool) ([]string, error)
}

func NewArgoCDService(db db.ArgoDB, submoduleEnabled bool, repoClientset apiclient.Clientset, newFileGlobbingEnabled bool) Repos {
	return &argoCDService{
		getRepository:          db.GetRepository,
		submoduleEnabled:       submoduleEnabled,
		newFileGlobbingEnabled: newFileGlobbingEnabled,
		getGitFilesFromRepoServer: func(ctx context.Context, fileRequest *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initializing new repo server client: %w", err)
			}
			defer io.Close(closer)
			return client.GetGitFiles(ctx, fileRequest)
		},
		getGitDirectoriesFromRepoServer: func(ctx context.Context, dirRequest *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initialising new repo server client: %w", err)
			}
			defer io.Close(closer)
			return client.GetGitDirectories(ctx, dirRequest)
		},
	}
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache, verifyCommit bool) (map[string][]byte, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
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
		VerifyCommit:              verifyCommit,
	}
	fileResponse, err := a.getGitFilesFromRepoServer(ctx, fileRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git files: %w", err)
	}
	return fileResponse.GetMap(), nil
}

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache, verifyCommit bool) ([]string, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		NoRevisionCache:  noRevisionCache,
		VerifyCommit:     verifyCommit,
	}

	dirResponse, err := a.getGitDirectoriesFromRepoServer(ctx, dirRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git Directories: %w", err)
	}
	return dirResponse.GetPaths(), nil
}
