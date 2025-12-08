package pull_request

import (
	"errors"
	"fmt"
	"testing"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	azureMock "github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/azure_devops/git/mocks"
	"github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/mocks"
	argoprojiov1alpha1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func createBoolPtr(x bool) *bool {
	return &x
}

// Helper function to create test Azure DevOps config
func createTestAzureDevOpsConfig(api, organization, project, repo string, labels []string) *argoprojiov1alpha1.PullRequestGeneratorAzureDevOps {
	return &argoprojiov1alpha1.PullRequestGeneratorAzureDevOps{
		API:          api,
		Organization: organization,
		Project:      project,
		Repo:         repo,
		Labels:       labels,
	}
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
			PullRequestId: createIntPtr(prID),
			Title:         createStringPtr(prTitle),
			SourceRefName: createStringPtr("refs/heads/feature-branch"),
			TargetRefName: createStringPtr("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: createStringPtr(prHeadSha),
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

func TestAzureDevOpsListReturnsRepositoryNotFoundError(t *testing.T) {
	args := git.GetPullRequestsByProjectArgs{
		Project:        createStringPtr("nonexistent"),
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

func TestPatAuthProvider(t *testing.T) {
	t.Run("PatAuthProvider with token", func(t *testing.T) {
		provider := NewPatAuthProvider("test-token")
		assert.NotNil(t, provider)
		assert.Equal(t, "test-token", provider.token)

		factory := provider.CreateClientFactory("https://dev.azure.com/testorg")
		assert.NotNil(t, factory)

		// Verify it's the right type
		_, ok := factory.(*devopsFactoryImpl)
		assert.True(t, ok, "Expected devopsFactoryImpl")
	})

	t.Run("PatAuthProvider with empty token", func(t *testing.T) {
		provider := NewPatAuthProvider("")
		assert.NotNil(t, provider)
		assert.Empty(t, provider.token)

		factory := provider.CreateClientFactory("https://dev.azure.com/testorg")
		assert.NotNil(t, factory)

		// Verify it's the right type
		_, ok := factory.(*devopsFactoryImpl)
		assert.True(t, ok, "Expected devopsFactoryImpl")
	})
}

func TestAnonymousAuthProvider(t *testing.T) {
	provider := NewAnonymousAuthProvider()
	assert.NotNil(t, provider)

	factory := provider.CreateClientFactory("https://dev.azure.com/testorg")
	assert.NotNil(t, factory)

	// Verify it's the right type
	_, ok := factory.(*devopsFactoryImpl)
	assert.True(t, ok, "Expected devopsFactoryImpl")
}

func TestWorkloadIdentityAuthProvider(t *testing.T) {
	provider := NewWorkloadIdentityAuthProvider()
	assert.NotNil(t, provider)

	factory := provider.CreateClientFactory("https://dev.azure.com/testorg")
	assert.NotNil(t, factory)

	// Verify it's the right type
	_, ok := factory.(*devopsWorkloadIdentityFactory)
	assert.True(t, ok, "Expected devopsWorkloadIdentityFactory")
}

func TestNewAzureDevOpsServiceWithAuthProvider(t *testing.T) {
	t.Run("with PatAuthProvider", func(t *testing.T) {
		provider := NewPatAuthProvider("test-token")
		config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
		service, err := NewAzureDevOpsServiceWithAuthProvider(config, provider)

		require.NoError(t, err)
		assert.NotNil(t, service)

		azureService, ok := service.(*AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.Equal(t, "testproject", azureService.project)
		assert.Equal(t, "testrepo", azureService.repo)
		assert.NotNil(t, azureService.clientFactory)
	})

	t.Run("with AnonymousAuthProvider", func(t *testing.T) {
		provider := NewAnonymousAuthProvider()
		config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
		service, err := NewAzureDevOpsServiceWithAuthProvider(config, provider)

		require.NoError(t, err)
		assert.NotNil(t, service)

		azureService, ok := service.(*AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.Equal(t, "testproject", azureService.project)
		assert.Equal(t, "testrepo", azureService.repo)
		assert.NotNil(t, azureService.clientFactory)
	})

	t.Run("with WorkloadIdentityAuthProvider", func(t *testing.T) {
		provider := NewWorkloadIdentityAuthProvider()
		config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
		service, err := NewAzureDevOpsServiceWithAuthProvider(config, provider)

		require.NoError(t, err)
		assert.NotNil(t, service)

		azureService, ok := service.(*AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.Equal(t, "testproject", azureService.project)
		assert.Equal(t, "testrepo", azureService.repo)
		assert.NotNil(t, azureService.clientFactory)
	})

	t.Run("with custom URL", func(t *testing.T) {
		provider := NewPatAuthProvider("test-token")
		config := createTestAzureDevOpsConfig("https://custom.azure.com", "testorg", "testproject", "testrepo", []string{"label1"})
		service, err := NewAzureDevOpsServiceWithAuthProvider(config, provider)

		require.NoError(t, err)
		assert.NotNil(t, service)

		azureService, ok := service.(*AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.Equal(t, "testproject", azureService.project)
		assert.Equal(t, "testrepo", azureService.repo)
		assert.Equal(t, []string{"label1"}, azureService.labels)
		assert.NotNil(t, azureService.clientFactory)
	})
}

func TestNewAzureDevOpsServiceUpdatedBehavior(t *testing.T) {
	t.Run("with token uses PatAuthProvider", func(t *testing.T) {
		config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
		service, err := NewAzureDevOpsService("test-token", config)

		require.NoError(t, err)
		assert.NotNil(t, service)

		azureService, ok := service.(*AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.Equal(t, "testproject", azureService.project)
		assert.Equal(t, "testrepo", azureService.repo)
		assert.NotNil(t, azureService.clientFactory)
	})

	t.Run("without token uses AnonymousAuthProvider", func(t *testing.T) {
		config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
		service, err := NewAzureDevOpsService("", config)

		require.NoError(t, err)
		assert.NotNil(t, service)

		azureService, ok := service.(*AzureDevOpsService)
		assert.True(t, ok, "Expected AzureDevOpsService")
		assert.Equal(t, "testproject", azureService.project)
		assert.Equal(t, "testrepo", azureService.repo)
		assert.NotNil(t, azureService.clientFactory)
	})
}

func TestNewAzureDevOpsServiceWithWorkloadIdentityUpdated(t *testing.T) {
	config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
	service, err := NewAzureDevOpsServiceWithWorkloadIdentity(config)

	require.NoError(t, err)
	assert.NotNil(t, service)

	azureService, ok := service.(*AzureDevOpsService)
	assert.True(t, ok, "Expected AzureDevOpsService")
	assert.Equal(t, "testproject", azureService.project)
	assert.Equal(t, "testrepo", azureService.repo)
	assert.NotNil(t, azureService.clientFactory)

	// Verify it uses workload identity factory
	_, ok = azureService.clientFactory.(*devopsWorkloadIdentityFactory)
	assert.True(t, ok, "Expected devopsWorkloadIdentityFactory")
}

func TestNewAzureDevOpsServiceAnonymous(t *testing.T) {
	config := createTestAzureDevOpsConfig("", "testorg", "testproject", "testrepo", nil)
	service, err := NewAzureDevOpsServiceAnonymous(config)

	require.NoError(t, err)
	assert.NotNil(t, service)

	azureService, ok := service.(*AzureDevOpsService)
	assert.True(t, ok, "Expected AzureDevOpsService")
	assert.Equal(t, "testproject", azureService.project)
	assert.Equal(t, "testrepo", azureService.repo)
	assert.NotNil(t, azureService.clientFactory)

	// Verify it uses regular factory (for anonymous connection)
	_, ok = azureService.clientFactory.(*devopsFactoryImpl)
	assert.True(t, ok, "Expected devopsFactoryImpl")
}

func TestAuthProviderInterface(t *testing.T) {
	// Test that all providers implement the interface
	providers := []AuthProvider{
		NewPatAuthProvider("test"),
		NewAnonymousAuthProvider(),
		NewWorkloadIdentityAuthProvider(),
	}

	for i, provider := range providers {
		t.Run(fmt.Sprintf("Provider %d implements interface", i), func(t *testing.T) {
			factory := provider.CreateClientFactory("https://dev.azure.com/test")
			assert.NotNil(t, factory)

			// Ensure factory implements AzureDevOpsClientFactory
			assert.Implements(t, (*AzureDevOpsClientFactory)(nil), factory)
		})
	}
}
