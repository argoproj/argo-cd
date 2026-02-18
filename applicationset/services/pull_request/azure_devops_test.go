package pull_request

import (
	"errors"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	azureMock "github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/azure_devops/git/mocks"
	"github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/mocks"
)

func TestListPullRequest(t *testing.T) {
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	prID := 123
	prTitle := "feat(123)"
	prHeadSha := "cd4973d9d14a08ffe6b641a89a68891d6aac8056"
	ctx := t.Context()
	uniqueName := "testName"

	pullRequestMock := []git.GitPullRequest{
		{
			PullRequestId: new(prID),
			Title:         new(prTitle),
			SourceRefName: new("refs/heads/feature-branch"),
			TargetRefName: new("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: new(prHeadSha),
			},
			Labels: &[]core.WebApiTagDefinition{},
			Repository: &git.GitRepository{
				Name: new(repoName),
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: new(uniqueName + "@example.com"),
			},
		},
	}

	args := git.GetPullRequestsByProjectArgs{
		Project:        &teamProject,
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
	}

	gitClientMock := &azureMock.Client{}
	clientFactoryMock := &mocks.AzureDevOpsClientFactory{}
	clientFactoryMock.EXPECT().GetClient(mock.Anything).Return(gitClientMock, nil)
	gitClientMock.EXPECT().GetPullRequestsByProject(mock.Anything, args).Return(&pullRequestMock, nil)

	provider := AzureDevOpsService{
		clientFactory: clientFactoryMock,
		project:       teamProject,
		repo:          repoName,
		labels:        nil,
	}

	list, err := provider.List(ctx)
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "feature-branch", list[0].Branch)
	assert.Equal(t, "main", list[0].TargetBranch)
	assert.Equal(t, prHeadSha, list[0].HeadSHA)
	assert.Equal(t, "feat(123)", list[0].Title)
	assert.Equal(t, int64(prID), list[0].Number)
	assert.Equal(t, uniqueName, list[0].Author)
}

func TestConvertLabes(t *testing.T) {
	testCases := []struct {
		name           string
		gotLabels      *[]core.WebApiTagDefinition
		expectedLabels []string
	}{
		{
			name:           "empty labels",
			gotLabels:      new([]core.WebApiTagDefinition{}),
			expectedLabels: []string{},
		},
		{
			name:           "nil labels",
			gotLabels:      (*[]core.WebApiTagDefinition)(nil),
			expectedLabels: []string{},
		},
		{
			name: "one label",
			gotLabels: new([]core.WebApiTagDefinition{
				{Name: new("label1"), Active: new(true)},
			}),
			expectedLabels: []string{"label1"},
		},
		{
			name: "two label",
			gotLabels: new([]core.WebApiTagDefinition{
				{Name: new("label1"), Active: new(true)},
				{Name: new("label2"), Active: new(true)},
			}),
			expectedLabels: []string{"label1", "label2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertLabels(tc.gotLabels)
			assert.Equal(t, tc.expectedLabels, got)
		})
	}
}

func TestContainAzureDevOpsLabels(t *testing.T) {
	testCases := []struct {
		name           string
		expectedLabels []string
		gotLabels      []string
		expectedResult bool
	}{
		{
			name:           "empty labels",
			expectedLabels: []string{},
			gotLabels:      []string{},
			expectedResult: true,
		},
		{
			name:           "no matching labels",
			expectedLabels: []string{"label1", "label2"},
			gotLabels:      []string{"label3", "label4"},
			expectedResult: false,
		},
		{
			name:           "some matching labels",
			expectedLabels: []string{"label1", "label2"},
			gotLabels:      []string{"label1", "label3"},
			expectedResult: false,
		},
		{
			name:           "all matching labels",
			expectedLabels: []string{"label1", "label2"},
			gotLabels:      []string{"label1", "label2"},
			expectedResult: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := containAzureDevOpsLabels(tc.expectedLabels, tc.gotLabels)
			assert.Equal(t, tc.expectedResult, got)
		})
	}
}

func TestBuildURL(t *testing.T) {
	testCases := []struct {
		name         string
		url          string
		organization string
		expected     string
	}{
		{
			name:         "Provided default URL and organization",
			url:          "https://dev.azure.com/",
			organization: "myorganization",
			expected:     "https://dev.azure.com/myorganization",
		},
		{
			name:         "Provided default URL and organization without trailing slash",
			url:          "https://dev.azure.com",
			organization: "myorganization",
			expected:     "https://dev.azure.com/myorganization",
		},
		{
			name:         "Provided no URL and organization",
			url:          "",
			organization: "myorganization",
			expected:     "https://dev.azure.com/myorganization",
		},
		{
			name:         "Provided custom URL and organization",
			url:          "https://azuredevops.example.com/",
			organization: "myorganization",
			expected:     "https://azuredevops.example.com/myorganization",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := buildURL(tc.url, tc.organization)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAzureDevOpsListReturnsRepositoryNotFoundError(t *testing.T) {
	args := git.GetPullRequestsByProjectArgs{
		Project:        new("nonexistent"),
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
	}

	pullRequestMock := []git.GitPullRequest{}

	gitClientMock := &azureMock.Client{}
	clientFactoryMock := &mocks.AzureDevOpsClientFactory{}
	clientFactoryMock.EXPECT().GetClient(mock.Anything).Return(gitClientMock, nil)

	// Mock the GetPullRequestsByProject to return an error containing "404"
	gitClientMock.EXPECT().GetPullRequestsByProject(mock.Anything, args).Return(&pullRequestMock,
		errors.New("The following project does not exist:"))

	provider := AzureDevOpsService{
		clientFactory: clientFactoryMock,
		project:       "nonexistent",
		repo:          "nonexistent",
		labels:        nil,
	}

	prs, err := provider.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}
