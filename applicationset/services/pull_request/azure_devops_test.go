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
	"github.com/google/uuid"
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

	repoID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	repositories := []git.GitRepository{
		{
			Id:   &repoID,
			Name: &repoName,
		},
	}

	gitClientMock := &azureMock.Client{}
	clientFactoryMock := &mocks.AzureDevOpsClientFactory{}
	clientFactoryMock.EXPECT().GetClient(mock.Anything).Return(gitClientMock, nil)
	gitClientMock.EXPECT().GetRepositories(mock.Anything, git.GetRepositoriesArgs{
		Project: &teamProject,
	}).
		Return(&repositories, nil)
	gitClientMock.EXPECT().
		GetPullRequestsByProject(mock.Anything, git.GetPullRequestsByProjectArgs{
			Project: &teamProject,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				RepositoryId: &repoID,
			},
			Skip: new(0),
			Top:  new(100),
		}).
		Return(&pullRequestMock, nil)

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

func TestAzureDevOpsListPullRequestPagination(t *testing.T) {
	t.Parallel()

	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	repoID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	repositories := []git.GitRepository{
		{
			Id:   &repoID,
			Name: &repoName,
		},
	}

	firstPage := make([]git.GitPullRequest, 100)
	for i := range firstPage {
		prID := i + 1
		firstPage[i] = git.GitPullRequest{
			PullRequestId: &prID,
			Title:         new("first-page"),
			SourceRefName: new("refs/heads/feature"),
			TargetRefName: new("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: new("first-page-sha"),
			},
			Repository: &git.GitRepository{
				Name: &repoName,
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: new("user@example.com"),
			},
		}
	}

	secondPage := []git.GitPullRequest{
		{
			PullRequestId: new(101),
			Title:         new("second-page"),
			SourceRefName: new("refs/heads/feature-101"),
			TargetRefName: new("refs/heads/main"),
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: new("second-page-sha"),
			},
			Repository: &git.GitRepository{
				Name: &repoName,
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: new("user@example.com"),
			},
		},
	}

	gitClientMock := &azureMock.Client{}
	clientFactoryMock := &mocks.AzureDevOpsClientFactory{}

	clientFactoryMock.EXPECT().
		GetClient(mock.Anything).
		Return(gitClientMock, nil)

	gitClientMock.EXPECT().
		GetRepositories(mock.Anything, git.GetRepositoriesArgs{
			Project: &teamProject,
		}).
		Return(&repositories, nil)

	firstSkip := 0
	secondSkip := 100
	top := 100
	gitClientMock.EXPECT().
		GetPullRequestsByProject(mock.Anything, git.GetPullRequestsByProjectArgs{
			Project: &teamProject,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				RepositoryId: &repoID,
			},
			Skip: &firstSkip,
			Top:  &top,
		}).
		Return(&firstPage, nil)

	gitClientMock.EXPECT().
		GetPullRequestsByProject(mock.Anything, git.GetPullRequestsByProjectArgs{
			Project: &teamProject,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				RepositoryId: &repoID,
			},
			Skip: &secondSkip,
			Top:  &top,
		}).
		Return(&secondPage, nil)

	provider := AzureDevOpsService{
		clientFactory: clientFactoryMock,
		project:       teamProject,
		repo:          repoName,
	}

	pullRequests, err := provider.List(t.Context())

	require.NoError(t, err)
	assert.Len(t, pullRequests, 101)
	assert.Equal(t, int64(1), pullRequests[0].Number)
	assert.Equal(t, int64(101), pullRequests[100].Number)
}

func TestListPullRequestFiltersByRepository(t *testing.T) {
	t.Parallel()

	teamProject := "myorg_project"
	repoName := "myorg_project_repo"
	repoID := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	repositories := []git.GitRepository{
		{
			Id:   &repoID,
			Name: &repoName,
		},
	}

	targetPRID := 1
	targetTitle := "target repository PR"
	targetSourceRef := "refs/heads/feature-target"
	targetTargetRef := "refs/heads/main"
	targetCommitID := "target-commit"
	targetRepoName := repoName
	targetAuthor := "user@example.com"

	otherPRID := 2
	otherTitle := "other repository PR"
	otherSourceRef := "refs/heads/feature-other"
	otherTargetRef := "refs/heads/main"
	otherCommitID := "other-commit"
	otherRepoName := "another_repo"
	otherAuthor := "other@example.com"

	pullRequests := []git.GitPullRequest{
		{
			PullRequestId: &targetPRID,
			Title:         &targetTitle,
			SourceRefName: &targetSourceRef,
			TargetRefName: &targetTargetRef,
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: &targetCommitID,
			},
			Repository: &git.GitRepository{
				Name: &targetRepoName,
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: &targetAuthor,
			},
		},
		{
			PullRequestId: &otherPRID,
			Title:         &otherTitle,
			SourceRefName: &otherSourceRef,
			TargetRefName: &otherTargetRef,
			LastMergeSourceCommit: &git.GitCommitRef{
				CommitId: &otherCommitID,
			},
			Repository: &git.GitRepository{
				Name: &otherRepoName,
			},
			CreatedBy: &webapi.IdentityRef{
				UniqueName: &otherAuthor,
			},
		},
	}

	gitClientMock := &azureMock.Client{}
	clientFactoryMock := &mocks.AzureDevOpsClientFactory{}

	clientFactoryMock.EXPECT().GetClient(mock.Anything).Return(gitClientMock, nil)

	gitClientMock.EXPECT().GetRepositories(mock.Anything, git.GetRepositoriesArgs{
		Project: &teamProject,
	}).
		Return(&repositories, nil)

	skip := 0
	top := 100
	gitClientMock.EXPECT().
		GetPullRequestsByProject(mock.Anything, git.GetPullRequestsByProjectArgs{
			Project: &teamProject,
			SearchCriteria: &git.GitPullRequestSearchCriteria{
				RepositoryId: &repoID,
			},
			Skip: &skip,
			Top:  &top,
		}).
		Return(&pullRequests, nil)

	provider := AzureDevOpsService{
		clientFactory: clientFactoryMock,
		project:       teamProject,
		repo:          repoName,
	}

	prs, err := provider.List(t.Context())

	require.NoError(t, err)
	require.Len(t, prs, 1)
	assert.Equal(t, int64(targetPRID), prs[0].Number)
	assert.Equal(t, targetTitle, prs[0].Title)
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

	project := "nonexistent"

	gitClientMock := &azureMock.Client{}
	clientFactoryMock := &mocks.AzureDevOpsClientFactory{}

	clientFactoryMock.EXPECT().
		GetClient(mock.Anything).
		Return(gitClientMock, nil)

	gitClientMock.EXPECT().
		GetRepositories(mock.Anything, git.GetRepositoriesArgs{
			Project: &project,
		}).
		Return(nil, errors.New("The following project does not exist:"))

	provider := AzureDevOpsService{
		clientFactory: clientFactoryMock,
		project:       project,
		repo:          "nonexistent",
		labels:        nil,
	}

	prs, err := provider.List(t.Context())

	assert.Empty(t, prs)
	require.Error(t, err)
	assert.True(t, IsRepositoryNotFoundError(err), "Expected RepositoryNotFoundError but got: %v", err)
}
