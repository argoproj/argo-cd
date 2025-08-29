package commit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/commit/mocks"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	gitmocks "github.com/argoproj/argo-cd/v3/util/git/mocks"
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
		_, err := service.CommitHydratedManifests(t.Context(), request)
		require.Error(t, err)
		assert.ErrorContains(t, err, "repo is required")
	})

	t.Run("missing repo URL", func(t *testing.T) {
		t.Parallel()

		service, _ := newServiceWithMocks(t)
		request := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{},
		}
		_, err := service.CommitHydratedManifests(t.Context(), request)
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
		_, err := service.CommitHydratedManifests(t.Context(), request)
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
		_, err := service.CommitHydratedManifests(t.Context(), request)
		require.Error(t, err)
		assert.ErrorContains(t, err, "sync branch is required")
	})

	t.Run("failed to create git client", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		_, err := service.CommitHydratedManifests(t.Context(), validRequest)
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
		// Since the test doesn't specify paths, we expect no specific path removal
		mockGitClient.On("CommitAndPush", "main", "test commit message").Return("", nil).Once()
		mockGitClient.On("CommitSHA").Return("it-worked!", nil).Once()
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		resp, err := service.CommitHydratedManifests(t.Context(), validRequest)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "it-worked!", resp.HydratedSha)
	})
}

func TestService_RemoveSpecificPaths(t *testing.T) {
	// Test that removeSpecificPaths only removes the specific paths being written to
	// and doesn't wipe out the entire repository like the old RemoveContents() did
	
	service := &Service{
		metricsServer:     metrics.NewMetricsServer(),
		repoClientFactory: &MockRepoClientFactory{},
	}
	
	mockGitClient := gitmocks.NewClient(t)
	
	// Test case: Multiple paths, including root and subdirectories
	paths := []*apiclient.PathDetails{
		{
			Path: "root/",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "root-config"}}`},
			},
		},
		{
			Path: "apps/app1/",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "app1-config"}}`},
			},
		},
	}
	
	// Expect root path to be removed completely (it's not the actual root, just a path named "root")
	mockGitClient.On("RunCmd", []string{"rm", "-rf", "root/"}).Return("", nil).Once()
	
	// Expect app1 path to be removed completely
	mockGitClient.On("RunCmd", []string{"rm", "-rf", "apps/app1/"}).Return("", nil).Once()
	
	err := service.removeSpecificPaths(mockGitClient, paths)
	
	assert.NoError(t, err)
	mockGitClient.AssertExpectations(t)
}

func TestService_RemoveSpecificPaths_NonExistentPath(t *testing.T) {
	// Test that removeSpecificPaths handles non-existent paths gracefully
	
	service := &Service{
		metricsServer:     metrics.NewMetricsServer(),
		repoClientFactory: &MockRepoClientFactory{},
	}
	
	mockGitClient := gitmocks.NewClient(t)
	
	paths := []*apiclient.PathDetails{
		{
			Path: "apps/new-app/",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "new-app-config"}}`},
			},
		},
	}
	
	// Simulate path doesn't exist
	mockGitClient.On("RunCmd", []string{"rm", "-rf", "apps/new-app/"}).Return("No such file or directory", assert.AnError).Once()
	
	err := service.removeSpecificPaths(mockGitClient, paths)
	
	assert.NoError(t, err) // Should not error for non-existent paths
	mockGitClient.AssertExpectations(t)
}

func TestService_RemoveSpecificPaths_RootOnly(t *testing.T) {
	// Test that removeSpecificPaths handles root path correctly
	
	service := &Service{
		metricsServer:     metrics.NewMetricsServer(),
		repoClientFactory: &MockRepoClientFactory{},
	}
	
	mockGitClient := gitmocks.NewClient(t)
	
	paths := []*apiclient.PathDetails{
		{
			Path: ".",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"apiVersion": "v1", "kind": "ConfigMap", "metadata": {"name": "root-config"}}`},
			},
		},
	}
	
	// Expect root path to remove only files, not directories
	mockGitClient.On("RunCmd", []string{"find", ".", "-maxdepth", "1", "-type", "f", "-delete"}).Return("", nil).Once()
	
	err := service.removeSpecificPaths(mockGitClient, paths)
	
	assert.NoError(t, err)
	mockGitClient.AssertExpectations(t)
}

// MockRepoClientFactory is a mock implementation of RepoClientFactory for testing
type MockRepoClientFactory struct {
	mock.Mock
}

func (m *MockRepoClientFactory) NewClient(repo *v1alpha1.Repository, path string) (git.Client, error) {
	args := m.Called(repo, path)
	return args.Get(0).(git.Client), args.Error(1)
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
