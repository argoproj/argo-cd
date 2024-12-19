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
	newFileGlobbingEnabled bool
	getGitFiles            func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error)
	getGitDirectories      func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error)
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
		newFileGlobbingEnabled: newFileGlobbingEnabled,
		getGitFiles: func(ctx context.Context, fileRequest *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initializing new repo server client: %w", err)
			}
			defer io.Close(closer)
			return client.GetGitFiles(ctx, fileRequest)
		},
		getGitDirectories: func(ctx context.Context, dirRequest *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error) {
			closer, client, err := repoClientset.NewRepoServerClient()
			if err != nil {
				return nil, fmt.Errorf("error initialising new repo server client: %w", err)
			}
			defer io.Close(closer)
			return client.GetGitDirectories(ctx, dirRequest)
		},
	}, nil
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache, verifyCommit bool) (map[string][]byte, error) {
	repos, err := a.listRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("error in ListRepositories: %w", err)
	}

	repo, err := getRepo(repos, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git files: %w", err)
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
	fileResponse, err := a.getGitFiles(ctx, fileRequest)
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

	repo, err := getRepo(repos, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git Directories: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		NoRevisionCache:  noRevisionCache,
		VerifyCommit:     verifyCommit,
	}

	dirResponse, err := a.getGitDirectories(ctx, dirRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git Directories: %w", err)
	}
	return dirResponse.GetPaths(), nil
}

func getRepo(repos []*v1alpha1.Repository, repoURL string, project string) (*v1alpha1.Repository, error) {
	repo, err := repository.FilterRepositoryByProjectAndURL(repos, repoURL, project)
	if err != nil {
		if errors.Is(err, status.Error(codes.PermissionDenied, "permission denied")) {
			// No repo found with a matching URL - attempt fallback without any actual credentials
			return &v1alpha1.Repository{Repo: repoURL}, nil
		} else if project == "" {
			for _, r := range repos {
				if git.SameURL(r.Repo, repoURL) {
					// Prioritize using a repository with an unset project.
					if r.Project == "" {
						return r, nil
					}

					if repo == nil {
						repo = r
					}
				}
			}

			// Try any repo matching the same repoURL
			if repo != nil {
				return repo, nil
			}

			// No repo found with a matching URL - attempt fallback without any actual credentials
			return &v1alpha1.Repository{Repo: repoURL}, nil
		}
	}
	return repo, err
}
