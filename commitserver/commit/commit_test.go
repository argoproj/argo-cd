package commit

import (
	"fmt"
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
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(nil, assert.AnError).Once()

		_, err := service.CommitHydratedManifests(t.Context(), validRequest)
		require.Error(t, err)
		assert.ErrorIs(t, err, assert.AnError)
	})

	t.Run("happy path", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("it-worked!", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		resp, err := service.CommitHydratedManifests(t.Context(), validRequest)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.HydratedSha) // changes introduced by commit note. hydration won't happen if there are no new manifest|s to commit
	})

	t.Run("root path with dot and blank - no directory removal", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything).Return(true, nil).Twice()
		mockGitClient.EXPECT().CommitAndPush("main", "test commit message").Return("", nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("root-and-blank-sha", nil).Twice()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithRootAndBlank := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch:  "main",
			SyncBranch:    "env/test",
			CommitMessage: "test commit message",
			Paths: []*apiclient.PathDetails{
				{
					Path: ".",
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-dot"}}`,
						},
					},
				},
				{
					Path: "",
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-blank"}}`,
						},
					},
				},
			},
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithRootAndBlank)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "root-and-blank-sha", resp.HydratedSha)
	})

	t.Run("subdirectory path - triggers directory removal", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything).Return(true, nil).Once()
		mockGitClient.EXPECT().CommitAndPush("main", "test commit message").Return("", nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("subdir-path-sha", nil).Twice()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithSubdirPath := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch: "main",
			SyncBranch:   "env/test",

			CommitMessage: "test commit message",
			Paths: []*apiclient.PathDetails{
				{
					Path: "apps/staging", // subdirectory path
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"Deployment","metadata":{"name":"test-app"}}`,
						},
					},
				},
			},
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithSubdirPath)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "subdir-path-sha", resp.HydratedSha)
	})

	t.Run("mixed paths - root and subdirectory", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything).Return(true, nil).Times(3)
		mockGitClient.EXPECT().CommitAndPush("main", "test commit message").Return("", nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("mixed-paths-sha", nil).Twice()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithMixedPaths := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch:  "main",
			SyncBranch:    "env/test",
			CommitMessage: "test commit message",
			Paths: []*apiclient.PathDetails{
				{
					Path: ".", // root path - should NOT trigger removal
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"global-config"}}`,
						},
					},
				},
				{
					Path: "apps/production", // subdirectory path - SHOULD trigger removal
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"Deployment","metadata":{"name":"prod-app"}}`,
						},
					},
				},
				{
					Path: "apps/staging", // another subdirectory path - SHOULD trigger removal
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"Deployment","metadata":{"name":"staging-app"}}`,
						},
					},
				},
			},
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithMixedPaths)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "mixed-paths-sha", resp.HydratedSha)
	})

	t.Run("empty paths array", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("empty-paths-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithEmptyPaths := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch:  "main",
			SyncBranch:    "env/test",
			CommitMessage: "test commit message",
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithEmptyPaths)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.HydratedSha) // changes introduced by commit note. hydration won't happen if there are no new manifest|s to commit
	})

	t.Run("duplicate request already hydrated", func(t *testing.T) {
		t.Parallel()

		strnote := "{\"drySha\":\"abc123\"}"
		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return(strnote, nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("dupe-test-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		request := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch:  "main",
			SyncBranch:    "env/test",
			DrySha:        "abc123",
			CommitMessage: "test commit message",
			Paths: []*apiclient.PathDetails{
				{
					Path: ".",
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"Deployment","metadata":{"name":"test-app"}}`,
						},
					},
				},
			},
		}

		resp, err := service.CommitHydratedManifests(t.Context(), request)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.HydratedSha) // changes introduced by commit note. hydration won't happen if there are no new manifest|s to commit
	})

	t.Run("root path with dot - no changes to manifest - should commit note only", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)
		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything).Return(false, nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA().Return("root-and-blank-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithRootAndBlank := &apiclient.CommitHydratedManifestsRequest{
			Repo: &v1alpha1.Repository{
				Repo: "https://github.com/argoproj/argocd-example-apps.git",
			},
			TargetBranch:  "main",
			SyncBranch:    "env/test",
			CommitMessage: "test commit message",
			Paths: []*apiclient.PathDetails{
				{
					Path: ".",
					Manifests: []*apiclient.HydratedManifestDetails{
						{
							ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"test-dot"}}`,
						},
					},
				},
			},
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithRootAndBlank)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Empty(t, resp.HydratedSha)
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
