package commit

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v2/commitserver/commit/mocks"
	"github.com/argoproj/argo-cd/v2/commitserver/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/util/git"
	gitmocks "github.com/argoproj/argo-cd/v2/util/git/mocks"
)

func Test_CommitHydratedManifests(t *testing.T) {
	t.Parallel()

	validRequest := &apiclient.CommitHydratedManifestsRequest{
		Repo: &v1alpha1.Repository{
			Repo: "https://github.com/argoproj/argocd-example-apps.git",
		},
		TargetBranch:  "main",
		SyncBranch:    "env/test",
		CommitMessage: "test commit message",
	}

	t.Run("missing repo", func(t *testing.T) {
		t.Parallel()

		service, _ := newServiceWithMocks(t)
		request := &apiclient.CommitHydratedManifestsRequest{}
		_, err := service.CommitHydratedManifests(context.Background(), request)
		require.Error(t, err)
		assert.ErrorContains(t, err, "repo is required")
	})

	t.Run("missing repo URL", func(t *testing.T) {
		t.Parallel()

		service, _ := newServiceWithMocks(t)
		request := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{},
		}
		_, err := service.CommitHydratedManifests(context.Background(), request)
		require.Error(t, err)
		assert.ErrorContains(t, err, "repo URL is required")
	})

	t.Run("missing target branch", func(t *testing.T) {
		t.Parallel()

		service, _ := newServiceWithMocks(t)
		request := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
		}
		_, err := service.CommitHydratedManifests(context.Background(), request)
		require.Error(t, err)
		assert.ErrorContains(t, err, "target branch is required")
	})

	t.Run("missing sync branch", func(t *testing.T) {
		t.Parallel()

		service, _ := newServiceWithMocks(t)
		request := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch: "main",
		}
		_, err := service.CommitHydratedManifests(context.Background(), request)
		require.Error(t, err)
		assert.ErrorContains(t, err, "sync branch is required")
	})

	t.Run("failed to create git client", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		_, err := service.CommitHydratedManifests(context.Background(), validRequest)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.On("Init").Return(nil).Once()
		mockGitClient.On("Fetch", mock.Anything).Return(nil).Once()
		mockGitClient.On("SetAuthor", "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.On("CheckoutOrOrphan", "env/test", false).Return("", nil).Once()
		mockGitClient.On("CheckoutOrNew", "main", "env/test", false).Return("", nil).Once()
		mockGitClient.On("RemoveContents").Return("", nil).Once()
		mockGitClient.On("CommitAndPush", "main", "test commit message").Return("", nil).Once()
		mockGitClient.On("CommitSHA").Return("it-worked!", nil).Once()
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		resp, err := service.CommitHydratedManifests(context.Background(), validRequest)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "it-worked!", resp.HydratedSha)
	})
}

func newServiceWithMocks(t *testing.T) (*Service, *mocks.RepoClientFactory) {
	t.Helper()

	metricsServer := metrics.NewMetricsServer()
	mockCredsStore := git.NoopCredsStore{}
	service := NewService(mockCredsStore, metricsServer)
	mockRepoClientFactory := mocks.NewRepoClientFactory(t)
	service.repoClientFactory = mockRepoClientFactory

	return service, mockRepoClientFactory
}
