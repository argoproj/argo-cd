package services

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/git"
	"github.com/argoproj/argo-cd/v2/util/io"
	"github.com/argoproj/argo-cd/v2/util/repository"
)

type argoCDService struct {
	listRepositories       func(ctx context.Context) ([]*v1alpha1.Repository, error)
	storecreds             git.CredsStore
	submoduleEnabled       bool
	repoServerClientSet    apiclient.Clientset
	newFileGlobbingEnabled bool
}

type Repos interface {
	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache, verifyCommit bool) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache, verifyCommit bool) ([]string, error)
}

func NewArgoCDService(listRepositories func(ctx context.Context) ([]*v1alpha1.Repository, error), submoduleEnabled bool, repoClientset apiclient.Clientset, newFileGlobbingEnabled bool) (Repos, error) {
	return &argoCDService{
		listRepositories:       listRepositories,
		submoduleEnabled:       submoduleEnabled,
		repoServerClientSet:    repoClientset,
		newFileGlobbingEnabled: newFileGlobbingEnabled,
	}, nil
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache, verifyCommit bool) (map[string][]byte, error) {
	repos, err := a.listRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in ListRepositories: %w", err)
	}

	repo, err := repository.FilterRepositoryByProjectAndURL(repos, repoURL, project)
	if err != nil {
		if errors.Is(err, status.Error(codes.PermissionDenied, "permission denied")) {
			repo = &v1alpha1.Repository{Repo: repoURL}
		} else {
			return nil, fmt.Errorf("error retrieving Git files: %w", err)
		}
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

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache, verifyCommit bool) ([]string, error) {
	repos, err := a.listRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in ListRepositories: %w", err)
	}

	repo, err := repository.FilterRepositoryByProjectAndURL(repos, repoURL, project)
	if err != nil {
		if errors.Is(err, status.Error(codes.PermissionDenied, "permission denied")) {
			repo = &v1alpha1.Repository{Repo: repoURL}
		} else {
			return nil, fmt.Errorf("error retrieving Git files: %w", err)
		}
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		NoRevisionCache:  noRevisionCache,
		VerifyCommit:     verifyCommit,
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
