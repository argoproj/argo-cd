package services

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/db"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

type argoCDService struct {
	getRepository                   func(ctx context.Context, url, project string) (*v1alpha1.Repository, error)
	submoduleEnabled                bool
	newFileGlobbingEnabled          bool
	getGitFilesFromRepoServer       func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error)
	getGitDirectoriesFromRepoServer func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error)
	getOciFilesFromRepoServer       func(ctx context.Context, req *apiclient.OciFilesRequest) (*apiclient.OciFilesResponse, error)
	getOciDirectoriesFromRepoServer func(ctx context.Context, req *apiclient.OciDirectoriesRequest) (*apiclient.OciDirectoriesResponse, error)
}

type Repos interface {
	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache, verifyCommit bool) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache, verifyCommit bool) ([]string, error)

	// GetOciFiles returns content of files (not directories) within the target OCI artifact
	GetOciFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool) (map[string][]byte, error)

	// GetOciDirectories returns a list of directories (not files) within the target OCI artifact
	GetOciDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool) ([]string, error)
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
			defer utilio.Close(closer)
			return client.GetGitFiles(ctx, fileRequest)
		},
		getGitDirectoriesFromRepoServer: func(ctx context.Context, dirRequest *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initialising new repo server client: %w", err)
			}
			defer utilio.Close(closer)
			return client.GetGitDirectories(ctx, dirRequest)
		},
		getOciFilesFromRepoServer: func(ctx context.Context, fileRequest *apiclient.OciFilesRequest) (*apiclient.OciFilesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initializing new repo server client: %w", err)
			}
			defer utilio.Close(closer)
			return client.GetOciFiles(ctx, fileRequest)
		},
		getOciDirectoriesFromRepoServer: func(ctx context.Context, dirRequest *apiclient.OciDirectoriesRequest) (*apiclient.OciDirectoriesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initialising new repo server client: %w", err)
			}
			defer utilio.Close(closer)
			return client.GetOciDirectories(ctx, dirRequest)
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

func (a *argoCDService) GetOciFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool) (map[string][]byte, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	fileRequest := &apiclient.OciFilesRequest{
		Repo:            repo,
		Revision:        revision,
		Path:            pattern,
		NoRevisionCache: noRevisionCache,
	}
	fileResponse, err := a.getOciFilesFromRepoServer(ctx, fileRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving OCI files: %w", err)
	}
	return fileResponse.GetMap(), nil
}

func (a *argoCDService) GetOciDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool) ([]string, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	dirRequest := &apiclient.OciDirectoriesRequest{
		Repo:            repo,
		Revision:        revision,
		NoRevisionCache: noRevisionCache,
	}

	dirResponse, err := a.getOciDirectoriesFromRepoServer(ctx, dirRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving OCI Directories: %w", err)
	}
	return dirResponse.GetPaths(), nil
}
