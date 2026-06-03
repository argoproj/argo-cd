package commit

import (
	"errors"
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
	"github.com/argoproj/argo-cd/v3/util/gpgsign"
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

	t.Run("custom author name and email configured", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)

		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.On("Init").Return(nil).Once()
		mockGitClient.On("Fetch", mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.On("SetAuthor", "Custom Author", "custom@example.com").Return("", nil).Once()
		mockGitClient.On("CheckoutOrOrphan", "env/test", false).Return("", nil).Once()
		mockGitClient.On("CheckoutOrNew", "main", "env/test", false).Return("", nil).Once()
		mockGitClient.On("GetCommitNote", mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.On("AddAndPushNote", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.On("CommitSHA").Return("custom-author-sha", nil).Once()
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithCustomAuthor := &apiclient.CommitHydratedManifestsRequest{
			Repo:          validRequest.Repo,
			SyncBranch:    validRequest.SyncBranch,
			TargetBranch:  validRequest.TargetBranch,
			DrySha:        validRequest.DrySha,
			CommitMessage: validRequest.CommitMessage,
			Paths:         validRequest.Paths,
			AuthorName:    "Custom Author",
			AuthorEmail:   "custom@example.com",
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithCustomAuthor)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "custom-author-sha", resp.HydratedSha)
	})

	t.Run("custom author email only configured", func(t *testing.T) {
		t.Parallel()

		service, mockRepoClientFactory := newServiceWithMocks(t)

		mockGitClient := gitmocks.NewClient(t)
		mockGitClient.On("Init").Return(nil).Once()
		mockGitClient.On("Fetch", mock.Anything, mock.Anything).Return(nil).Once()
		// When only email is provided, name defaults to "Argo CD"
		mockGitClient.On("SetAuthor", "Argo CD", "custom@example.com").Return("", nil).Once()
		mockGitClient.On("CheckoutOrOrphan", "env/test", false).Return("", nil).Once()
		mockGitClient.On("CheckoutOrNew", "main", "env/test", false).Return("", nil).Once()
		mockGitClient.On("GetCommitNote", mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.On("AddAndPushNote", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.On("CommitSHA").Return("custom-email-only-sha", nil).Once()
		mockRepoClientFactory.On("NewClient", mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

		requestWithEmailOnly := &apiclient.CommitHydratedManifestsRequest{
			Repo:          validRequest.Repo,
			SyncBranch:    validRequest.SyncBranch,
			TargetBranch:  validRequest.TargetBranch,
			DrySha:        validRequest.DrySha,
			CommitMessage: validRequest.CommitMessage,
			Paths:         validRequest.Paths,
			AuthorName:    "",
			AuthorEmail:   "custom@example.com",
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithEmailOnly)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "custom-email-only-sha", resp.HydratedSha)
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
		assert.Equal(t, "it-worked!", resp.HydratedSha, "Should return existing hydrated SHA for no-op")
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
		mockGitClient.EXPECT().Commit("test commit message", "", "").Return("", nil).Once()
		mockGitClient.EXPECT().Push("main").Return("", nil).Once()
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
		mockGitClient.EXPECT().Commit("test commit message", "", "").Return("", nil).Once()
		mockGitClient.EXPECT().Push("main").Return("", nil).Once()
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
		mockGitClient.EXPECT().Commit("test commit message", "", "").Return("", nil).Once()
		mockGitClient.EXPECT().Push("main").Return("", nil).Once()
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
			DrySha:        "dry-sha-456",
		}

		resp, err := service.CommitHydratedManifests(t.Context(), requestWithEmptyPaths)
		require.NoError(t, err)
		require.NotNil(t, resp)
		assert.Equal(t, "empty-paths-sha", resp.HydratedSha, "Should return existing hydrated SHA for no-op")
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
		assert.Equal(t, "dupe-test-sha", resp.HydratedSha, "Should return existing hydrated SHA when already hydrated")
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
			DrySha:        "dry-sha-123",
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
		// BUG FIX: When manifests don't change (no-op), the existing hydrated SHA should be returned.
		assert.Equal(t, "root-and-blank-sha", resp.HydratedSha, "Should return existing hydrated SHA for no-op")
	})
}

func Test_CommitHydratedManifests_Signing(t *testing.T) {
	t.Parallel()

	// Long key ID is the trailing 16 chars of the fingerprint.
	const fp = "001122334455667788990011ABCDEF1234567890"
	signingCfg := &gpgsign.Config{
		KeyID:         fp[len(fp)-16:],
		Fingerprint:   fp,
		GPGProgram:    "/tmp/wrap.sh",
		SigningKeyIDs: []string{fp[len(fp)-16:]},
	}

	baseRequest := &apiclient.CommitHydratedManifestsRequest{
		Repo:          &v1alpha1.Repository{Repo: "https://github.com/argoproj/argocd-example-apps.git"},
		TargetBranch:  "main",
		SyncBranch:    "env/test",
		CommitMessage: "test commit message",
		Paths: []*apiclient.PathDetails{{
			Path: ".",
			Manifests: []*apiclient.HydratedManifestDetails{{
				ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"x"}}`,
			}},
		}},
	}

	commonMockExpectations := func(c *gitmocks.Client) {
		c.EXPECT().Init().Return(nil).Once()
		c.EXPECT().Fetch(mock.Anything, mock.Anything).Return(nil).Once()
		c.EXPECT().SetAuthor("Argo CD", "argo-cd@example.com").Return("", nil).Once()
		c.EXPECT().CheckoutOrOrphan("env/test", false).Return("", nil).Once()
		c.EXPECT().CheckoutOrNew("main", "env/test", false).Return("", nil).Once()
		c.EXPECT().GetCommitNote(mock.Anything, mock.Anything).Return("", fmt.Errorf("no note %w", git.ErrNoNoteFound)).Once()
		c.EXPECT().HasFileChanged(mock.Anything).Return(true, nil)
	}

	t.Run("signs commit and pushes on good signature", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID, signingCfg.GPGProgram).Return("", nil).Once()
		c.EXPECT().HeadSignatureStatus().Return("G", signingCfg.Fingerprint, nil).Once()
		c.EXPECT().Push("main").Return("", nil).Once()
		c.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		c.EXPECT().CommitSHA().Return("signed-sha", nil).Twice()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		resp, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.NoError(t, err)
		assert.Equal(t, "signed-sha", resp.HydratedSha)
	})

	t.Run("does not push when signature status is bad", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID, signingCfg.GPGProgram).Return("", nil).Once()
		c.EXPECT().HeadSignatureStatus().Return("B", signingCfg.Fingerprint, nil).Once()
		c.EXPECT().CommitSHA().Return("never-pushed-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		_, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.Error(t, err)
		assert.ErrorContains(t, err, "signature status")
		// Push is intentionally NOT in the mock expectations — if the impl
		// reaches push, the mocks.Client strict mode in NewClient will fail.
	})

	t.Run("does not push when signature lookup itself fails", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID, signingCfg.GPGProgram).Return("", nil).Once()
		c.EXPECT().HeadSignatureStatus().Return("", "", errors.New("git boom")).Once()
		c.EXPECT().CommitSHA().Return("never-pushed-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		_, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.Error(t, err)
		assert.ErrorContains(t, err, "verify signature")
	})

	t.Run("does not push when signed by a different key", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID, signingCfg.GPGProgram).Return("", nil).Once()
		c.EXPECT().HeadSignatureStatus().Return("G", "DEADBEEFDEADBEEF", nil).Once()
		c.EXPECT().CommitSHA().Return("never-pushed-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		_, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.Error(t, err)
		assert.ErrorContains(t, err, "signed by key")
	})
}

func newServiceWithMocks(t *testing.T) (*Service, *mocks.RepoClientFactory) {
	t.Helper()
	return newServiceWithMocksAndSigning(t, nil)
}

func newServiceWithMocksAndSigning(t *testing.T, signingConfig *gpgsign.Config) (*Service, *mocks.RepoClientFactory) {
	t.Helper()

	metricsServer := metrics.NewMetricsServer()
	mockCredsStore := git.NoopCredsStore{}
	service := NewService(mockCredsStore, metricsServer, signingConfig)
	mockRepoClientFactory := mocks.NewRepoClientFactory(t)
	service.repoClientFactory = mockRepoClientFactory

	return service, mockRepoClientFactory
}
