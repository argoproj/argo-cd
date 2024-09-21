package scm_provider

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"

	"github.com/microsoft/azure-devops-go-api/azuredevops"
	azureGit "github.com/microsoft/azure-devops-go-api/azuredevops/git"

	azureMock "github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider/azure_devops/git/mocks"
)

func s(input string) *string {
	return ptr.To(input)
}

func TestAzureDevopsRepoHasPath(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	path := "dir/subdir/item.yaml"
	branchName := "my/featurebranch"

	ctx := context.Background()
	uuid := uuid.New().String()

	testCases := []struct {
		name             string
		pathFound        bool
		azureDevopsError error
		returnError      bool
		errorMessage     string
		clientError      error
	}{
		{
			name:        "RepoHasPath when Azure DevOps client factory fails returns error",
			clientError: fmt.Errorf("Client factory error"),
		},
		{
			name:      "RepoHasPath when found returns true",
			pathFound: true,
		},
		{
			name:             "RepoHasPath when no path found returns false",
			pathFound:        false,
			azureDevopsError: azuredevops.WrappedError{TypeKey: s(AzureDevOpsErrorsTypeKeyValues.GitItemNotFound)},
		},
		{
			name:             "RepoHasPath when unknown Azure DevOps WrappedError occurs returns error",
			pathFound:        false,
			azureDevopsError: azuredevops.WrappedError{TypeKey: s("OtherAzureDevopsException")},
			returnError:      true,
			errorMessage:     "failed to check for path existence",
		},
		{
			name:             "RepoHasPath when unknown Azure DevOps error occurs returns error",
			pathFound:        false,
			azureDevopsError: fmt.Errorf("Undefined error from Azure Devops"),
			returnError:      true,
			errorMessage:     "failed to check for path existence",
		},
		{
			name:             "RepoHasPath when wrapped Azure DevOps error occurs without TypeKey returns error",
			pathFound:        false,
			azureDevopsError: azuredevops.WrappedError{},
			returnError:      true,
			errorMessage:     "failed to check for path existence",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, testCase.clientError)

			repoId := &uuid
			gitClientMock.On("GetItem", ctx, azureGit.GetItemArgs{Project: &teamProject, Path: &path, VersionDescriptor: &azureGit.GitVersionDescriptor{Version: &branchName}, RepositoryId: repoId}).Return(nil, testCase.azureDevopsError)

			provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock}

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuid, Branch: branchName}
			hasPath, err := provider.RepoHasPath(ctx, repo, path)

			if testCase.clientError != nil {
				require.ErrorContains(t, err, testCase.clientError.Error())
				gitClientMock.AssertNotCalled(t, "GetItem", ctx, azureGit.GetItemArgs{Project: &teamProject, Path: &path, VersionDescriptor: &azureGit.GitVersionDescriptor{Version: &branchName}, RepositoryId: repoId})

				return
			}

			if testCase.returnError {
				require.ErrorContains(t, err, testCase.errorMessage)
			}

			assert.Equal(t, testCase.pathFound, hasPath)

			gitClientMock.AssertCalled(t, "GetItem", ctx, azureGit.GetItemArgs{Project: &teamProject, Path: &path, VersionDescriptor: &azureGit.GitVersionDescriptor{Version: &branchName}, RepositoryId: repoId})
		})
	}
}

func TestGetDefaultBranchOnDisabledRepo(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	defaultBranch := "main"

	ctx := context.Background()

	testCases := []struct {
		name              string
		azureDevOpsError  error
		shouldReturnError bool
	}{
		{
			name:              "azure devops error when disabled repo causes empty return value",
			azureDevOpsError:  azuredevops.WrappedError{TypeKey: s(AzureDevOpsErrorsTypeKeyValues.GitRepositoryNotFound)},
			shouldReturnError: false,
		},
		{
			name:              "azure devops error with unknown error type returns error",
			azureDevOpsError:  azuredevops.WrappedError{TypeKey: s("OtherError")},
			shouldReturnError: true,
		},
		{
			name:              "other error when calling azure devops returns error",
			azureDevOpsError:  fmt.Errorf("some unknown error"),
			shouldReturnError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			uuid := uuid.New().String()

			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)

			gitClientMock.On("GetBranch", ctx, azureGit.GetBranchArgs{RepositoryId: &repoName, Project: &teamProject, Name: &defaultBranch}).Return(nil, testCase.azureDevOpsError)

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuid, Branch: defaultBranch}

			provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock, allBranches: false}
			branches, err := provider.GetBranches(ctx, repo)

			if testCase.shouldReturnError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Empty(t, branches)

			gitClientMock.AssertExpectations(t)
		})
	}
}

func TestGetAllBranchesOnDisabledRepo(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	defaultBranch := "main"

	ctx := context.Background()

	testCases := []struct {
		name              string
		azureDevOpsError  error
		shouldReturnError bool
	}{
		{
			name:              "azure devops error when disabled repo causes empty return value",
			azureDevOpsError:  azuredevops.WrappedError{TypeKey: s(AzureDevOpsErrorsTypeKeyValues.GitRepositoryNotFound)},
			shouldReturnError: false,
		},
		{
			name:              "azure devops error with unknown error type returns error",
			azureDevOpsError:  azuredevops.WrappedError{TypeKey: s("OtherError")},
			shouldReturnError: true,
		},
		{
			name:              "other error when calling azure devops returns error",
			azureDevOpsError:  fmt.Errorf("some unknown error"),
			shouldReturnError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			uuid := uuid.New().String()

			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)

			gitClientMock.On("GetBranches", ctx, azureGit.GetBranchesArgs{RepositoryId: &repoName, Project: &teamProject}).Return(nil, testCase.azureDevOpsError)

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuid, Branch: defaultBranch}

			provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock, allBranches: true}
			branches, err := provider.GetBranches(ctx, repo)

			if testCase.shouldReturnError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.Empty(t, branches)

			gitClientMock.AssertExpectations(t)
		})
	}
}

func TestAzureDevOpsGetDefaultBranchStripsRefsName(t *testing.T) {
	t.Run("Get branches only default branch removes characters before querying azure devops", func(t *testing.T) {
		organization := "myorg"
		teamProject := "myorg_project"
		repoName := "myorg_project_repo"

		ctx := context.Background()
		uuid := uuid.New().String()
		strippedBranchName := "somebranch"
		defaultBranch := fmt.Sprintf("refs/heads/%v", strippedBranchName)

		branchReturn := &azureGit.GitBranchStats{Name: &strippedBranchName, Commit: &azureGit.GitCommitRef{CommitId: s("abc123233223")}}
		repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuid, Branch: defaultBranch}

		gitClientMock := azureMock.Client{}

		clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
		clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)

		gitClientMock.On("GetBranch", ctx, azureGit.GetBranchArgs{RepositoryId: &repoName, Project: &teamProject, Name: &strippedBranchName}).Return(branchReturn, nil)

		provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock, allBranches: false}
		branches, err := provider.GetBranches(ctx, repo)

		require.NoError(t, err)
		assert.Len(t, branches, 1)
		assert.Equal(t, strippedBranchName, branches[0].Branch)

		gitClientMock.AssertCalled(t, "GetBranch", ctx, azureGit.GetBranchArgs{RepositoryId: &repoName, Project: &teamProject, Name: &strippedBranchName})
	})
}

func TestAzureDevOpsGetBranchesDefultBranchOnly(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"

	ctx := context.Background()
	uuid := uuid.New().String()

	defaultBranch := "main"

	testCases := []struct {
		name                string
		expectedBranch      *azureGit.GitBranchStats
		getBranchesApiError error
		clientError         error
	}{
		{
			name:           "GetBranches AllBranches false when single branch returned returns branch",
			expectedBranch: &azureGit.GitBranchStats{Name: &defaultBranch, Commit: &azureGit.GitCommitRef{CommitId: s("abc123233223")}},
		},
		{
			name:                "GetBranches AllBranches false when request fails returns error and empty result",
			getBranchesApiError: fmt.Errorf("Remote Azure Devops GetBranches error"),
		},
		{
			name:        "GetBranches AllBranches false when Azure DevOps client fails returns error",
			clientError: fmt.Errorf("Could not get Azure Devops API client"),
		},
		{
			name:           "GetBranches AllBranches false when branch returned with long commit SHA",
			expectedBranch: &azureGit.GitBranchStats{Name: &defaultBranch, Commit: &azureGit.GitCommitRef{CommitId: s("53863052ADF24229AB72154B4D83DAB7")}},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, testCase.clientError)

			gitClientMock.On("GetBranch", ctx, azureGit.GetBranchArgs{RepositoryId: &repoName, Project: &teamProject, Name: &defaultBranch}).Return(testCase.expectedBranch, testCase.getBranchesApiError)

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuid, Branch: defaultBranch}

			provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock, allBranches: false}
			branches, err := provider.GetBranches(ctx, repo)

			if testCase.clientError != nil {
				require.ErrorContains(t, err, testCase.clientError.Error())
				gitClientMock.AssertNotCalled(t, "GetBranch", ctx, azureGit.GetBranchArgs{RepositoryId: &repoName, Project: &teamProject, Name: &defaultBranch})

				return
			}

			if testCase.getBranchesApiError != nil {
				assert.Empty(t, branches)
				require.ErrorContains(t, err, testCase.getBranchesApiError.Error())
			} else {
				if testCase.expectedBranch != nil {
					assert.NotEmpty(t, branches)
				}
				assert.Len(t, branches, 1)
				assert.Equal(t, repo.RepositoryId, branches[0].RepositoryId)
			}

			gitClientMock.AssertCalled(t, "GetBranch", ctx, azureGit.GetBranchArgs{RepositoryId: &repoName, Project: &teamProject, Name: &defaultBranch})
		})
	}
}

func TestAzureDevopsGetBranches(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"

	ctx := context.Background()
	uuid := uuid.New().String()

	testCases := []struct {
		name                       string
		expectedBranches           *[]azureGit.GitBranchStats
		getBranchesApiError        error
		clientError                error
		allBranches                bool
		expectedProcessingErrorMsg string
	}{
		{
			name:             "GetBranches when single branch returned returns this branch info",
			expectedBranches: &[]azureGit.GitBranchStats{{Name: s("feature-feat1"), Commit: &azureGit.GitCommitRef{CommitId: s("abc123233223")}}},
			allBranches:      true,
		},
		{
			name:                "GetBranches when Azure DevOps request fails returns error and empty result",
			getBranchesApiError: fmt.Errorf("Remote Azure Devops GetBranches error"),
			allBranches:         true,
		},
		{
			name:                       "GetBranches when no branches returned returns error",
			allBranches:                true,
			expectedProcessingErrorMsg: "empty branch result",
		},
		{
			name:        "GetBranches when git client retrievel fails returns error",
			clientError: fmt.Errorf("Could not get Azure Devops API client"),
			allBranches: true,
		},
		{
			name: "GetBranches when multiple branches returned returns branch info for all branches",
			expectedBranches: &[]azureGit.GitBranchStats{
				{Name: s("feature-feat1"), Commit: &azureGit.GitCommitRef{CommitId: s("abc123233223")}},
				{Name: s("feature/feat2"), Commit: &azureGit.GitCommitRef{CommitId: s("4334")}},
				{Name: s("feature/feat2"), Commit: &azureGit.GitCommitRef{CommitId: s("53863052ADF24229AB72154B4D83DAB7")}},
			},
			allBranches: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gitClientMock := azureMock.Client{}

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, testCase.clientError)

			gitClientMock.On("GetBranches", ctx, azureGit.GetBranchesArgs{RepositoryId: &repoName, Project: &teamProject}).Return(testCase.expectedBranches, testCase.getBranchesApiError)

			repo := &Repository{Organization: organization, Repository: repoName, RepositoryId: uuid}

			provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock, allBranches: testCase.allBranches}
			branches, err := provider.GetBranches(ctx, repo)

			if testCase.expectedProcessingErrorMsg != "" {
				require.ErrorContains(t, err, testCase.expectedProcessingErrorMsg)
				assert.Nil(t, branches)

				return
			}
			if testCase.clientError != nil {
				require.ErrorContains(t, err, testCase.clientError.Error())
				gitClientMock.AssertNotCalled(t, "GetBranches", ctx, azureGit.GetBranchesArgs{RepositoryId: &repoName, Project: &teamProject})
				return
			}

			if testCase.getBranchesApiError != nil {
				assert.Empty(t, branches)
				require.ErrorContains(t, err, testCase.getBranchesApiError.Error())
			} else {
				if len(*testCase.expectedBranches) > 0 {
					assert.NotEmpty(t, branches)
				}
				assert.Len(t, branches, len(*testCase.expectedBranches))
				for _, branch := range branches {
					assert.NotEmpty(t, branch.RepositoryId)
					assert.Equal(t, repo.RepositoryId, branch.RepositoryId)
				}
			}

			gitClientMock.AssertCalled(t, "GetBranches", ctx, azureGit.GetBranchesArgs{RepositoryId: &repoName, Project: &teamProject})
		})
	}
}

func TestGetAzureDevopsRepositories(t *testing.T) {
	organization := "myorg"
	teamProject := "myorg_project"

	uuid := uuid.New()
	ctx := context.Background()

	repoId := &uuid

	testCases := []struct {
		name                  string
		getRepositoriesError  error
		repositories          []azureGit.GitRepository
		expectedNumberOfRepos int
	}{
		{
			name:                  "ListRepos when single repo found returns repo info",
			repositories:          []azureGit.GitRepository{{Name: s("repo1"), DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u"), Id: repoId}},
			expectedNumberOfRepos: 1,
		},
		{
			name:         "ListRepos when repo has no default branch returns empty list",
			repositories: []azureGit.GitRepository{{Name: s("repo2"), RemoteUrl: s("https://remoteurl.u"), Id: repoId}},
		},
		{
			name:                 "ListRepos when Azure DevOps request fails returns error",
			getRepositoriesError: fmt.Errorf("Could not get repos"),
		},
		{
			name:         "ListRepos when repo has no name returns empty list",
			repositories: []azureGit.GitRepository{{DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u"), Id: repoId}},
		},
		{
			name:         "ListRepos when repo has no remote URL returns empty list",
			repositories: []azureGit.GitRepository{{DefaultBranch: s("main"), Name: s("repo_name"), Id: repoId}},
		},
		{
			name:         "ListRepos when repo has no ID returns empty list",
			repositories: []azureGit.GitRepository{{DefaultBranch: s("main"), Name: s("repo_name"), RemoteUrl: s("https://remoteurl.u")}},
		},
		{
			name: "ListRepos when multiple repos returned returns list of eligible repos only",
			repositories: []azureGit.GitRepository{
				{Name: s("returned1"), DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u"), Id: repoId},
				{Name: s("missing_default_branch"), RemoteUrl: s("https://remoteurl.u"), Id: repoId},
				{DefaultBranch: s("missing_name"), RemoteUrl: s("https://remoteurl.u"), Id: repoId},
				{Name: s("missing_remote_url"), DefaultBranch: s("main"), Id: repoId},
				{Name: s("missing_id"), DefaultBranch: s("main"), RemoteUrl: s("https://remoteurl.u")},
			},
			expectedNumberOfRepos: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			gitClientMock := azureMock.Client{}
			gitClientMock.On("GetRepositories", ctx, azureGit.GetRepositoriesArgs{Project: s(teamProject)}).Return(&testCase.repositories, testCase.getRepositoriesError)

			clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
			clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock)

			provider := AzureDevOpsProvider{organization: organization, teamProject: teamProject, clientFactory: clientFactoryMock}

			repositories, err := provider.ListRepos(ctx, "https")

			if testCase.getRepositoriesError != nil {
				require.Error(t, err, "Expected an error from test case %v", testCase.name)
			}

			if testCase.expectedNumberOfRepos == 0 {
				assert.Empty(t, repositories)
			} else {
				assert.NotEmpty(t, repositories)
				assert.Len(t, repositories, testCase.expectedNumberOfRepos)
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
