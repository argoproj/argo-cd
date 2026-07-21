package services

import (
	"context"
	"fmt"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/db"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

type ArgoCDService struct {
	getRepository  func(ctx context.Context, url, project string) (*v1alpha1.Repository, error)
	configProvider configbus.Provider
	// Deprecated: use configProvider.ApplicationsetGitSubmoduleEnabled.
	submoduleEnabled bool
	// Deprecated: use configProvider.ApplicationsetEnableNewGitFileGlobbing.
	newFileGlobbingEnabled          bool
	getGitFilesFromRepoServer       func(ctx context.Context, req *apiclient.GitFilesRequest) (*apiclient.GitFilesResponse, error)
	getGitDirectoriesFromRepoServer func(ctx context.Context, req *apiclient.GitDirectoriesRequest) (*apiclient.GitDirectoriesResponse, error)
}

// SetConfigProvider attaches the configbus Provider used for git setting reads.
func (a *ArgoCDService) SetConfigProvider(p configbus.Provider) {
	a.configProvider = p
}

func (a *ArgoCDService) submoduleEnabledResolved() (bool, error) {
	if a.configProvider != nil {
		return a.configProvider.ApplicationsetGitSubmoduleEnabled(context.Background())
	}
	return a.LegacySubmoduleEnabled(), nil
}

func (a *ArgoCDService) newFileGlobbingEnabledResolved() (bool, error) {
	if a.configProvider != nil {
		return a.configProvider.ApplicationsetEnableNewGitFileGlobbing(context.Background())
	}
	return a.LegacyNewFileGlobbingEnabled(), nil
}

type Repos interface {
	// GetFiles returns content of files (not directories) within the target repo
	GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) (map[string][]byte, error)

	// GetDirectories returns a list of directories (not files) within the target repo
	GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) ([]string, error)
}

func NewArgoCDService(db db.ArgoDB, submoduleEnabled bool, repoClientset apiclient.Clientset, newFileGlobbingEnabled bool) *ArgoCDService {
	return &ArgoCDService{
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
	}
}

func (a *ArgoCDService) GetFiles(ctx context.Context, repoURL, revision, project, pattern string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) (map[string][]byte, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	submoduleEnabled, err := a.submoduleEnabledResolved()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve git submodule enabled: %w", err)
	}
	newFileGlobbingEnabled, err := a.newFileGlobbingEnabledResolved()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve new git file globbing enabled: %w", err)
	}

	fileRequest := &apiclient.GitFilesRequest{
		Repo:                      repo,
		SubmoduleEnabled:          submoduleEnabled,
		Revision:                  revision,
		Path:                      pattern,
		NewGitFileGlobbingEnabled: newFileGlobbingEnabled,
		NoRevisionCache:           noRevisionCache,
		SourceIntegrity:           sourceIntegrity,
		VerifyCommit:              sourceIntegrity != nil, // nolint:staticcheck
	}

	fileResponse, err := a.getGitFilesFromRepoServer(ctx, fileRequest)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Git files: %w", err)
	}
	return fileResponse.GetMap(), nil
}

func (a *ArgoCDService) GetDirectories(ctx context.Context, repoURL, revision, project string, noRevisionCache bool, sourceIntegrity *v1alpha1.SourceIntegrity) ([]string, error) {
	repo, err := a.getRepository(ctx, repoURL, project)
	if err != nil {
		return nil, fmt.Errorf("error in GetRepository: %w", err)
	}

	submoduleEnabled, err := a.submoduleEnabledResolved()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve git submodule enabled: %w", err)
	}

	dirRequest := &apiclient.GitDirectoriesRequest{
		Repo:             repo,
		SubmoduleEnabled: submoduleEnabled,
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
