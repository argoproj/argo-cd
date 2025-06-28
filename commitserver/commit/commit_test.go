package commit

import (
	"os/exec"
	"testing"

	log "github.com/sirupsen/logrus"
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
		mockGitClient.On("RemoveContents").Return("", nil).Once()
		mockGitClient.On("CommitAndPush", "main", "test commit message").Return("", nil).Once()
		mockGitClient.On("CommitSHA").Return("it-worked!", nil).Once()
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		resp, err := service.CommitHydratedManifests(t.Context(), validRequest)
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

func Test_getCoAuthorTrailers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		metadata *v1alpha1.RevisionMetadata
		expected []string
	}{
		{
			name:     "nil metadata",
			metadata: nil,
			expected: nil,
		},
		{
			name: "empty metadata",
			metadata: &v1alpha1.RevisionMetadata{
				Author:     "",
				References: nil,
			},
			expected: nil,
		},
		{
			name: "single author",
			metadata: &v1alpha1.RevisionMetadata{
				Author: "John Doe <john.doe@example.com>",
			},
			expected: []string{"Co-Authored-By: John Doe <john.doe@example.com>"},
		},
		{
			name: "multiple references with authors",
			metadata: &v1alpha1.RevisionMetadata{
				References: []v1alpha1.RevisionReference{
					{
						Commit: &v1alpha1.CommitMetadata{
							Author: "Alice <alice@example.com>",
						},
					},
					{
						Commit: &v1alpha1.CommitMetadata{
							Author: "Bob <bob@example.com>",
						},
					},
				},
			},
			expected: []string{
				"Co-Authored-By: Alice <alice@example.com>",
				"Co-Authored-By: Bob <bob@example.com>",
			},
		},
		{
			name: "author and references combined",
			metadata: &v1alpha1.RevisionMetadata{
				Author: "John Doe <john.doe@example.com>",
				References: []v1alpha1.RevisionReference{
					{
						Commit: &v1alpha1.CommitMetadata{
							Author: "Alice <alice@example.com>",
						},
					},
				},
			},
			expected: []string{
				"Co-Authored-By: Alice <alice@example.com>",
				"Co-Authored-By: John Doe <john.doe@example.com>",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := getCoAuthorTrailers(tt.metadata)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_handleCommitRequest_HappyPath(t *testing.T) {
	t.Parallel()

	// Create a temporary directory for the local git repository
	tempDir := t.TempDir()

	// Initialize a local git repository
	err := exec.Command("git", "-C", tempDir, "init").Run()
	require.NoError(t, err)
	err = exec.Command("git", "-C", tempDir, "config", "receive.denyCurrentBranch", "ignore").Run()
	require.NoError(t, err)

	// Create a test service
	metricsServer := metrics.NewMetricsServer()
	service := NewService(git.NoopCredsStore{}, metricsServer)

	// Prepare the request
	request := &apiclient.CommitHydratedManifestsRequest{
		Repo: &v1alpha1.Repository{
			Repo: "file://" + tempDir,
		},
		TargetBranch:  "main",
		SyncBranch:    "test-sync",
		CommitMessage: "Test commit message",
		DrySha:        "123456",
		DryCommitMetadata: &v1alpha1.RevisionMetadata{
			Author: "John Doe <john.doe@example.com>",
			References: []v1alpha1.RevisionReference{
				{
					Commit: &v1alpha1.CommitMetadata{
						Author: "Alice <alice@example.com>",
					},
				},
			},
		},
		Paths: []*apiclient.PathDetails{
			{
				Path: "test-path",
				Manifests: []*apiclient.HydratedManifestDetails{
					{ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"empty-configmap"}}`},
				},
			},
		},
	}

	// Call handleCommitRequest
	logCtx := log.WithField("test", "handleCommitRequest")
	_, sha, err := service.handleCommitRequest(logCtx, request)
	require.NoError(t, err)

	// Verify the commit SHA
	assert.NotEmpty(t, sha)

	// Verify the commit trailers
	out, err := exec.Command("git", "-C", tempDir, "show", sha).Output()
	if err != nil {
		require.NoError(t, err)
	}
	commitMessage := string(out)
	assert.Contains(t, commitMessage, "Co-Authored-By: Alice <alice@example.com>")
	assert.Contains(t, commitMessage, "Co-Authored-By: John Doe <john.doe@example.com>")
}
