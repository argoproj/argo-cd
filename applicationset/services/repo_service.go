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
	// GetFiles returns content of files (not directories) within the target repo that match
	// includePatterns and are not matched by excludePatterns.
	GetFiles(ctx context.Context, repoURL, revision, project string, includePatterns, excludePatterns []string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) ([]string, error)

	// GetOciFiles returns content of files (not directories) within the target OCI artifact that
	// match includePatterns and are not matched by excludePatterns.
	GetOciFiles(ctx context.Context, repoURL, revision, project string, includePatterns, excludePatterns []string, noRevisionCache bool) (map[string][]byte, error)

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
				return nil, fmt.Errorf("error initializing new repo server client: %w", err)
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
				return nil, fmt.Errorf("error initializing new repo server client: %w", err)
			}
			defer utilio.Close(closer)
			return client.GetOciDirectories(ctx, dirRequest)
		},
	}
}

// firstPattern populates the single-pattern field kept on the request for repo-servers
// that predate includePatterns. Such a repo-server honours only this pattern: the other
// includes are dropped, and so are the excludes, so the files it returns are neither a
// subset nor a superset of the correct answer. An ApplicationSet can therefore generate
// the wrong set of Applications while the appset controller and the repo-server are on
// different versions. Sending the first include keeps that window as small as it can be
// made without version negotiation -- omitting the field entirely would make an old
// repo-server return every file in the repo instead.
func firstPattern(includePatterns []string) string {
	if len(includePatterns) == 0 {
		return ""
	}
	return includePatterns[0]
}

func (a *argoCDService) GetFiles(ctx context.Context, repoURL, revision, project string, includePatterns, excludePatterns []string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) (map[string][]byte, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	fileRequest := &apiclient.GitFilesRequest{
		Repo:                      repo,
		SubmoduleEnabled:          a.submoduleEnabled,
		Revision:                  revision,
		Path:                      firstPattern(includePatterns),
		NewGitFileGlobbingEnabled: a.newFileGlobbingEnabled,
		NoRevisionCache:           noRevisionCache,
		SourceIntegrity:           sourceIntegrity,
		VerifyCommit:              sourceIntegrity != nil, // nolint:staticcheck
		IncludePatterns:           includePatterns,
		ExcludePatterns:           excludePatterns,
	}

	fileResponse, err := a.getGitFilesFromRepoServer(ctx, fileRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git files: %w", err)
	}
	return fileResponse.GetMap(), nil
}

func (a *argoCDService) GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) ([]string, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: a.submoduleEnabled,
		Revision:         revision,
		NoRevisionCache:  noRevisionCache,
		SourceIntegrity:  sourceIntegrity,
		//nolint:staticcheck // SA1019: VerifyCommit is deprecated, but we still need to support it for backward compatibility.
		VerifyCommit: sourceIntegrity != nil,
	}

	dirResponse, err := a.getGitDirectoriesFromRepoServer(ctx, dirRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git Directories: %w", err)
	}
	return dirResponse.GetPaths(), nil
}

func (a *argoCDService) GetOciFiles(ctx context.Context, repoURL, revision, project string, includePatterns, excludePatterns []string, noRevisionCache bool) (map[string][]byte, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	fileRequest := &apiclient.OciFilesRequest{
		Repo:            repo,
		Revision:        revision,
		Glob:            firstPattern(includePatterns),
		NoRevisionCache: noRevisionCache,
		IncludePatterns: includePatterns,
		ExcludePatterns: excludePatterns,
	}
	fileResponse, err := a.getOciFilesFromRepoServer(ctx, fileRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving OCI files: %w", err)
	}
	return fileResponse.GetFiles(), nil
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
		return nil, fmt.Errorf("error retrieving OCI directories: %w", err)
	}
	return dirResponse.GetPaths(), nil
}
