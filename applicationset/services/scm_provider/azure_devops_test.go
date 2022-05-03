package scm_provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	azureMock "github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider/azure_devops/git/mocks"
	"github.com/microsoft/azure-devops-go-api/azuredevops"
	azureGit "github.com/microsoft/azure-devops-go-api/azuredevops/git"
)

type uuidHolder struct {
	uuid uuid.UUID
	err  error
}

func getUuidHolder() uuidHolder {
	uuid, err := uuid.NewUUID()
	return uuidHolder{uuid: uuid, err: err}
}

func s(input string) *string {
	return &input
}

func TestAzureDevopsRepoHasPath(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	path := "dir/subdir/item.yaml"
	branchName := "my/featurebranch"

	ctx := context.TODO()
	uuidHolder := getUuidHolder()

	testCases := []struct {
		name             string
		pathFound        bool
		azureDevopsError error
		returnError      bool
	}{
		{
			name:      "repo_has_path_found_returns_true",
			pathFound: true,
		},
		{
			name:             "repo_has_path_not_found_empty_search_result_returns_false",
			pathFound:        false,
			azureDevopsError: azuredevops.WrappedError{TypeKey: s("GitItemNotFoundException")},
		},
		{
			name:             "repo_has_path_not_found_other_azure_devops_error_returns_error",
			pathFound:        false,
			azureDevopsError: azuredevops.WrappedError{TypeKey: s("OtherAzureDevopsException")},
			returnError:      true,
		},
		{
			name:             "repo_has_path_not_found_other_error_returns_error",
			pathFound:        false,
			azureDevopsError: fmt.Errorf("Undefined error from Azure Devops"),
			returnError:      true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)

			repoIdAsString := fmt.Sprintf("%v", uuidHolder.uuid)
			gitClientMock.On("GetItem", ctx, azureGit.GetItemArgs{Project: &teamProject, Path: &path, VersionDescriptor: &azureGit.GitVersionDescriptor{Version: &branchName}, RepositoryId: &repoIdAsString}).Return(nil, testCase.azureDevopsError)

			provider := AzureDevopsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock}

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuidHolder.uuid, Branch: branchName}
			hasPath, err := provider.RepoHasPath(ctx, repo, path)

			if testCase.returnError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, testCase.pathFound, hasPath)

			gitClientMock.AssertExpectations(t)
		})
	}
}

func TestAzureDevopsGetBranches(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"

	ctx := context.TODO()
	uuidHolder := getUuidHolder()

	testCases := []struct {
		name                string
		expectedBranches    []azureGit.GitBranchStats
		getBranchesApiError error
		expectedError       error
	}{
		{
			name:             "get_branches_single_branch_returns_branch",
			expectedBranches: []azureGit.GitBranchStats{{Name: s("feature-feat1"), Commit: &azureGit.GitCommitRef{CommitId: s("abc123233223")}}},
		},
		{
			name:                "get_branches_fails_return_empty_result",
			getBranchesApiError: fmt.Errorf("Remote Azure Devops GetBranches error"),
		},
		{
			name: "get_branches_no_branches_returned",
		},
		{
			name:          "get_branches_get_client_fails",
			expectedError: fmt.Errorf("Could not get Azure Devops API client"),
		},
		{
			name: "get_branches_multiple_branches_returns_branches",
			expectedBranches: []azureGit.GitBranchStats{
				{Name: s("feature-feat1"), Commit: &azureGit.GitCommitRef{CommitId: s("abc123233223")}},
				{Name: s("feature/feat2"), Commit: &azureGit.GitCommitRef{CommitId: s("4334")}},
				{Name: s("feature/feat2"), Commit: &azureGit.GitCommitRef{CommitId: s("53863052ADF24229AB72154B4D83DAB7")}},
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, testCase.expectedError)

			setup := gitClientMock.On("GetBranches", ctx, azureGit.GetBranchesArgs{RepositoryId: &repoName, Project: &teamProject}).Return(&testCase.expectedBranches, testCase.getBranchesApiError)
			if testCase.expectedError != nil {
				setup.Maybe()
			}

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuidHolder.uuid}

			provider := AzureDevopsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock}
			branches, err := provider.GetBranches(ctx, repo)

			if testCase.expectedError != nil {
				assert.Error(t, err, "Expected error to occur on test case %s", testCase.name)
			} else {
				if testCase.getBranchesApiError != nil {
					assert.Empty(t, branches)
				} else {
					if len(testCase.expectedBranches) > 0 {
						assert.NotEmpty(t, branches)
					}
					assert.Equal(t, len(testCase.expectedBranches), len(branches))
					for _, branch := range branches {
						assert.NotEmpty(t, branch.RepositoryId)
						assert.Equal(t, repo.RepositoryId, branch.RepositoryId)
					}
				}
			}

			gitClientMock.AssertExpectations(t)
		})
	}
}

func TestGetAzureDevopsRepositories(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"

	uuidHolder := getUuidHolder()
	ctx := context.TODO()

	testCases := []struct {
		name                  string
		getRepositoriesError  error
		repositories          []azureGit.GitRepository
		expectEmptyRepoList   bool
		expectedNumberOfRepos int
	}{
		{
			name:         "get_repositories_single_repo_returns_repo",
			repositories: []azureGit.GitRepository{{Name: s("repo1"), DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u"), Id: &uuidHolder.uuid}},
		},
		{
			name:                "get_repositories_repo_without_default_branch_returns_empty",
			repositories:        []azureGit.GitRepository{{Name: s("repo2"), RemoteUrl: s("https://remoteurl.u"), Id: &uuidHolder.uuid}},
			expectEmptyRepoList: true,
		},
		{
			name:                 "get_repositories_fails_returns_error",
			getRepositoriesError: fmt.Errorf("Could not get repos"),
		},
		{
			name:                "get_repositories_repo_without_name_returns_empty",
			repositories:        []azureGit.GitRepository{{DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u"), Id: &uuidHolder.uuid}},
			expectEmptyRepoList: true,
		},
		{
			name:                "get_repositories_repo_without_remoteurl_returns_empty",
			repositories:        []azureGit.GitRepository{{DefaultBranch: s("main"), Name: s("repo_name"), Id: &uuidHolder.uuid}},
			expectEmptyRepoList: true,
		},
		{
			name:                "get_repositories_repo_without_id_returns_empty",
			repositories:        []azureGit.GitRepository{{DefaultBranch: s("main"), Name: s("repo_name"), RemoteUrl: s("https://remoteurl.u")}},
			expectEmptyRepoList: true,
		},
		{
			name: "get_repositories_multiple_results_returns_eligible",
			repositories: []azureGit.GitRepository{
				{Name: s("returned1"), DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u"), Id: &uuidHolder.uuid},
				{Name: s("missing_default_branch"), RemoteUrl: s("https://remoteurl.u"), Id: &uuidHolder.uuid},
				{DefaultBranch: s("missing_name"), RemoteUrl: s("https://remoteurl.u"), Id: &uuidHolder.uuid},
				{Name: s("missing_remote_url"), DefaultBranch: s("main"), Id: &uuidHolder.uuid},
				{Name: s("missing_id"), DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u")}},
			expectedNumberOfRepos: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			gitClientMock := azureMock.Client{}
			gitClientMock.On("GetRepositories", ctx, azureGit.GetRepositoriesArgs{Project: s(teamProject)}).Return(&testCase.repositories, testCase.getRepositoriesError)

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock)

			provider := AzureDevopsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock}

			repositories, err := provider.ListRepos(ctx, "https")

			if err != nil {
				if testCase.getRepositoriesError != nil {
					assert.Error(t, fmt.Errorf("Error getting repos in test %s, %v", testCase.name, err))
				}
			} else {
				if testCase.expectEmptyRepoList {
					assert.Empty(t, repositories)
				} else {
					assert.NotEmpty(t, repositories)
					if testCase.expectedNumberOfRepos > 0 {
						assert.Equal(t, testCase.expectedNumberOfRepos, len(repositories))
					}
				}
			}
			gitClientMock.AssertExpectations(t)
		})
	}
}

type AzureClientFactoryMock struct {
	mock *mock.Mock
}

func (m *AzureClientFactoryMock) GetClient(ctx context.Context) (azureGit.Client, error) {
	args := m.mock.Called(ctx)

	var client azureGit.Client
	c := args.Get(0)
	if c != nil {
		client = c.(azureGit.Client)
	}

	var err error
	if len(args) > 1 {
		if e, ok := args.Get(1).(error); ok {
			err = e
		}
	}

	return client, err
}

type AzureGitClientMock struct {
	mock *mock.Mock
}
