package pull_request

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/core"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/git"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/webapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	azureMock "github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/azure_devops/git/mocks"
	"github.com/argoproj/argo-cd/v3/applicationset/services/scm_provider/mocks"
	"github.com/argoproj/argo-cd/v3/util/workloadidentity"
	workloadidentitymocks "github.com/argoproj/argo-cd/v3/util/workloadidentity/mocks"
)

func TestListPullRequest(t *testing.T) {
	t.Parallel()
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
		clientFactory:  clientFactoryMock,
		project:        teamProject,
		repo:           repoName,
		labels:         nil,
		excludedLabels: nil,
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

func TestListPullRequestExcludedLabels(t *testing.T) {
	t.Parallel()
	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	prID := 123
	prTitle := "feat(123)"
	prHeadSha := "cd4973d9d14a08ffe6b641a89a68891d6aac8056"
	ctx := t.Context()
	uniqueName := "testName"

	// The PR matches the repo and has no required-label restriction, so only the
	// excluded "stale" label should cause it to be filtered out.
	pullRequestMock := []git.GitPullRequest{
		{
			PullRequestId: new(prID),
			Title:         new(prTitle),
			SourceRefName: new("refs/heads/feature-branch"),
			TargetRefName: new("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: new(prHeadSha),
			},
			Labels: &[]core.WebApiTagDefinition{
				{Name: new("stale"), Active: new(true)},
			},
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
		clientFactory:  clientFactoryMock,
		project:        teamProject,
		repo:           repoName,
		labels:         nil,
		excludedLabels: []string{"stale"},
	}

	list, err := provider.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestNewAzureDevOpsService(t *testing.T) {
	t.Parallel()
	svc, err := NewAzureDevOpsService("", "https://dev.azure.com/", "myorg", "myproject", "myrepo", nil, []string{"stale"})
	require.NoError(t, err)
	azureSvc, ok := svc.(*AzureDevOpsService)
	require.True(t, ok)
	assert.Equal(t, []string{"stale"}, azureSvc.excludedLabels)
}

func TestConvertLabes(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		name           string
		gotLabels      *[]core.WebApiTagDefinition
		expectedLabels []string
	}{
		{
			name:           "empty labels",
			gotLabels:      &[]core.WebApiTagDefinition{},
			expectedLabels: []string{},
		},
		{
			name:           "nil labels",
			gotLabels:      nil,
			expectedLabels: []string{},
		},
		{
			name: "one label",
			gotLabels: &[]core.WebApiTagDefinition{
				{Name: new("label1"), Active: new(true)},
			},
			expectedLabels: []string{"label1"},
		},
		{
			name: "two label",
			gotLabels: &[]core.WebApiTagDefinition{
				{Name: new("label1"), Active: new(true)},
				{Name: new("label2"), Active: new(true)},
			},
			expectedLabels: []string{"label1", "label2"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := convertLabels(tc.gotLabels)
			assert.Equal(t, tc.expectedLabels, got)
		})
	}
}

func TestContainAzureDevOpsLabels(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			got := containAzureDevOpsLabels(tc.expectedLabels, tc.gotLabels)
			assert.Equal(t, tc.expectedResult, got)
		})
	}
}

func TestBuildURL(t *testing.T) {
	t.Parallel()
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
			t.Parallel()
			result := buildURL(tc.url, tc.organization)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestAzureDevOpsListReturnsRepositoryNotFoundError(t *testing.T) {
	t.Parallel()
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
		clientFactory:  clientFactoryMock,
		project:        "nonexistent",
		repo:           "nonexistent",
		labels:         nil,
		excludedLabels: nil,
	}

	prs, err := provider.List(t.Context())

	// Should return empty pull requests list
	assert.Empty(t, prs)

	// Should return RepositoryNotFoundError
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}

func TestNewAzureDevOpsServiceWithWorkloadIdentity(t *testing.T) {
	t.Parallel()
	tokenProvider := workloadidentitymocks.NewTokenProvider(t)
	tokenProvider.EXPECT().GetToken(azureDevOpsTokenScope).
		Return(&workloadidentity.Token{AccessToken: "entra-token", ExpiresOn: time.Now().Add(time.Hour)}, nil).Once()

	svc, err := NewAzureDevOpsServiceWithWorkloadIdentity(tokenProvider, "", "myorg", "myproject", "myrepo", []string{"preview"}, []string{"stale"})
	require.NoError(t, err)
	azureSvc, ok := svc.(*AzureDevOpsService)
	require.True(t, ok)
	assert.Equal(t, "myproject", azureSvc.project)
	assert.Equal(t, "myrepo", azureSvc.repo)
	assert.Equal(t, []string{"preview"}, azureSvc.labels)
	assert.Equal(t, []string{"stale"}, azureSvc.excludedLabels)

	factory, ok := azureSvc.clientFactory.(*workloadIdentityClientFactory)
	require.True(t, ok, "workload identity must use the workload identity client factory")
	assert.Equal(t, "https://dev.azure.com/myorg", factory.organizationURL)

	// The token must come from the injected provider, for the Azure DevOps resource scope.
	token, err := factory.getToken()
	require.NoError(t, err)
	assert.Equal(t, "entra-token", token)
}

func TestNewAzureDevOpsServiceWithWorkloadIdentityTokenProviderError(t *testing.T) {
	t.Parallel()
	tokenProvider := workloadidentitymocks.NewTokenProvider(t)
	tokenProvider.EXPECT().GetToken(azureDevOpsTokenScope).Return(nil, errors.New("no federated token file")).Once()

	svc, err := NewAzureDevOpsServiceWithWorkloadIdentity(tokenProvider, "", "myorg", "myproject", "myrepo", nil, nil)
	require.NoError(t, err)

	_, err = svc.(*AzureDevOpsService).clientFactory.(*workloadIdentityClientFactory).getToken()
	require.ErrorContains(t, err, "no federated token file")
}

func TestNewAzureDevOpsServiceWithWorkloadIdentityAPIValidation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		api         string
		expectedURL string
		expectedErr string
	}{
		{name: "default api", api: "", expectedURL: "https://dev.azure.com/myorg"},
		{name: "explicit default api", api: "https://dev.azure.com/", expectedURL: "https://dev.azure.com/myorg"},
		{name: "custom https api", api: "https://devops.example.com/tfs", expectedURL: "https://devops.example.com/tfs/myorg"},
		{name: "uppercase https scheme", api: "HTTPS://dev.azure.com", expectedURL: "HTTPS://dev.azure.com/myorg"},
		{name: "plain http api", api: "http://dev.azure.com", expectedErr: "requires an https:// Azure DevOps api URL"},
		{name: "in-cluster http api", api: "http://my-service.my-namespace.svc.cluster.local", expectedErr: "requires an https:// Azure DevOps api URL"},
		{name: "api without scheme", api: "dev.azure.com", expectedErr: "requires an https:// Azure DevOps api URL"},
		{name: "api without host", api: "https://", expectedErr: "requires an https:// Azure DevOps api URL"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// No token may be requested while the service is created, whether or not the api URL is valid.
			tokenProvider := workloadidentitymocks.NewTokenProvider(t)

			svc, err := NewAzureDevOpsServiceWithWorkloadIdentity(tokenProvider, tc.api, "myorg", "myproject", "myrepo", nil, nil)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				assert.Nil(t, svc)
				return
			}
			require.NoError(t, err)
			factory := svc.(*AzureDevOpsService).clientFactory.(*workloadIdentityClientFactory)
			assert.Equal(t, tc.expectedURL, factory.organizationURL)
		})
	}
}

func TestNewAzureDevOpsServiceWithWorkloadIdentityNilProvider(t *testing.T) {
	t.Parallel()
	svc, err := NewAzureDevOpsServiceWithWorkloadIdentity(nil, "", "myorg", "myproject", "myrepo", nil, nil)
	require.Error(t, err)
	assert.Nil(t, svc)
}

func TestNewBearerConnection(t *testing.T) {
	t.Parallel()
	connection := newBearerConnection("https://dev.azure.com/myorg/", "entra-token")
	assert.Equal(t, "Bearer entra-token", connection.AuthorizationString)
	assert.Equal(t, "https://dev.azure.com/myorg", connection.BaseUrl)
}

func TestWorkloadIdentityFactoryGetClientTokenError(t *testing.T) {
	t.Parallel()
	factory := &workloadIdentityClientFactory{
		organizationURL: "https://dev.azure.com/myorg",
		getToken: func() (string, error) {
			return "", errors.New("no federated token file")
		},
	}

	client, err := factory.GetClient(t.Context())
	require.ErrorContains(t, err, "failed to get Azure DevOps access token using Azure Workload Identity")
	require.ErrorContains(t, err, "no federated token file")
	assert.Nil(t, client)
}

func TestWorkloadIdentityFactoryGetClientSendsFreshBearerToken(t *testing.T) {
	t.Parallel()
	var mu sync.Mutex
	var authHeaders []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authHeaders = append(authHeaders, r.Header.Get("Authorization"))
		mu.Unlock()
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	tokens := []string{"token-1", "token-2"}
	calls := 0
	factory := &workloadIdentityClientFactory{
		organizationURL: server.URL + "/myorg",
		getToken: func() (string, error) {
			token := tokens[calls]
			calls++
			return token, nil
		},
	}

	// The fake server does not implement the Azure DevOps API, so client creation fails after the first request,
	// which is enough to check which credentials were sent.
	_, err := factory.GetClient(t.Context())
	require.Error(t, err)
	_, err = factory.GetClient(t.Context())
	require.Error(t, err)

	assert.Equal(t, 2, calls, "a token must be requested for every client")
	mu.Lock()
	defer mu.Unlock()
	require.NotEmpty(t, authHeaders)
	assert.Contains(t, authHeaders, "Bearer token-1")
	assert.Contains(t, authHeaders, "Bearer token-2")
	for _, h := range authHeaders {
		assert.NotContains(t, h, "Basic", "workload identity must not fall back to PAT basic auth")
	}
}
