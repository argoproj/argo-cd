package pull_request

import (
	"context"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/webapi"

	"github.com/microsoft/azure-devops-go-api/azuredevops/core"
	git "github.com/microsoft/azure-devops-go-api/azuredevops/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	azureMock "github.com/argoproj/argo-cd/v2/applicationset/services/scm_provider/azure_devops/git/mocks"
)

func createBoolPtr(x bool) *bool {
	return &x
}

func createStringPtr(x string) *string {
	return &x
}

func createIntPtr(x int) *int {
	return &x
}

func createLabelsPtr(x []core.WebApiTagDefinition) *[]core.WebApiTagDefinition {
	return &x
}

func createUniqueNamePtr(x string) *string {
	return &x
}

type AzureClientFactoryMock struct {
	mock *mock.Mock
}

func (m *AzureClientFactoryMock) GetClient(ctx context.Context) (git.Client, error) {
	args := m.mock.Called(ctx)

	var client git.Client
	c := args.Get(0)
	if c != nil {
		client = c.(git.Client)
	}

	var err error
	if len(args) > 1 {
		if e, ok := args.Get(1).(error); ok {
			err = e
		}
	}

	return client, err
}

func TestListPullRequest(t *testing.T) {
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	pr_id := 123
	pr_title := "feat(123)"
	pr_head_sha := "cd4973d9d14a08ffe6b641a89a68891d6aac8056"
	ctx := context.Background()
	uniqueName := "testName"

	pullRequestMock := []git.GitPullRequest{
		{
			PullRequestId: createIntPtr(pr_id),
			Title:         createStringPtr(pr_title),
			SourceRefName: createStringPtr("refs/heads/feature-branch"),
			TargetRefName: createStringPtr("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: createStringPtr(pr_head_sha),
			},
			Labels: &[]core.WebApiTagDefinition{},
			Repository: &git.GitRepository{
				Name: createStringPtr(repoName),
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: createUniqueNamePtr(uniqueName + "@example.com"),
			},
		},
	}

	args := git.GetPullRequestsByProjectArgs{
		Project:        &teamProject,
		SearchCriteria: &git.GitPullRequestSearchCriteria{},
	}

	gitClientMock := azureMock.Client{}
	clientFactoryMock := &AzureClientFactoryMock{mock: &mock.Mock{}}
	clientFactoryMock.mock.On("GetClient", mock.Anything).Return(&gitClientMock, nil)
	gitClientMock.On("GetPullRequestsByProject", ctx, args).Return(&pullRequestMock, nil)

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
	assert.Equal(t, pr_head_sha, list[0].HeadSHA)
	assert.Equal(t, "feat(123)", list[0].Title)
	assert.Equal(t, pr_id, list[0].Number)
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
			gotLabels:      createLabelsPtr([]core.WebApiTagDefinition{}),
			expectedLabels: []string{},
		},
		{
			name:           "nil labels",
			gotLabels:      createLabelsPtr(nil),
			expectedLabels: []string{},
		},
		{
			name: "one label",
			gotLabels: createLabelsPtr([]core.WebApiTagDefinition{
				{Name: createStringPtr("label1"), Active: createBoolPtr(true)},
			}),
			expectedLabels: []string{"label1"},
		},
		{
			name: "two label",
			gotLabels: createLabelsPtr([]core.WebApiTagDefinition{
				{Name: createStringPtr("label1"), Active: createBoolPtr(true)},
				{Name: createStringPtr("label2"), Active: createBoolPtr(true)},
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
