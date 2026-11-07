package commit

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/commitserver/commit/mocks"
	"github.com/argoproj/argo-cd/v3/commitserver/metrics"
	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/git"
	gitmocks "github.com/argoproj/argo-cd/v3/util/git/mocks"
	"github.com/argoproj/argo-cd/v3/util/gpgsign"
	"github.com/argoproj/argo-cd/v3/util/gpgsign/gpgsigntest"
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
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Custom Author", "custom@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("custom-author-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

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
		mockGitClient.EXPECT().Init().Return(nil).Once()
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		// When only email is provided, name defaults to "Argo CD"
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "custom@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("custom-email-only-sha", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("it-worked!", nil).Once()
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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything, mock.Anything).Return(true, nil).Twice()
		mockGitClient.EXPECT().Commit("test commit message", "").Return("", nil).Once()
		mockGitClient.EXPECT().Push("main").Return("", nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("root-and-blank-sha", nil).Twice()
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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything, mock.Anything).Return(true, nil).Once()
		mockGitClient.EXPECT().Commit("test commit message", "").Return("", nil).Once()
		mockGitClient.EXPECT().Push("main").Return("", nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("subdir-path-sha", nil).Twice()
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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything, mock.Anything).Return(true, nil).Times(3)
		mockGitClient.EXPECT().Commit("test commit message", "").Return("", nil).Once()
		mockGitClient.EXPECT().Push("main").Return("", nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("mixed-paths-sha", nil).Twice()
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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("empty-paths-sha", nil).Once()
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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return(strnote, nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("dupe-test-sha", nil).Once()
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
		mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
		mockGitClient.EXPECT().HasFileChanged(mock.Anything, mock.Anything).Return(false, nil).Once()
		mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("root-and-blank-sha", nil).Once()
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
		c.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
		c.EXPECT().SetAuthor(mock.Anything, "Argo CD", "argo-cd@example.com").Return("", nil).Once()
		c.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
		c.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
		c.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("no note %w", git.ErrNoNoteFound)).Once()
		c.EXPECT().HasFileChanged(mock.Anything, mock.Anything).Return(true, nil)
	}

	t.Run("signs commit and pushes on good signature", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID).Return("", nil).Once()
		c.EXPECT().CommitSHA(mock.Anything).Return("signed-sha", nil).Twice()
		// The signature check must be pinned to the exact SHA just created, not HEAD.
		c.EXPECT().CommitSignatureStatus(mock.Anything, "signed-sha").Return("G", signingCfg.Fingerprint, nil).Once()
		c.EXPECT().Push("main").Return("", nil).Once()
		c.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
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
		c.EXPECT().Commit("test commit message", signingCfg.KeyID).Return("", nil).Once()
		c.EXPECT().CommitSHA(mock.Anything).Return("never-pushed-sha", nil).Twice()
		c.EXPECT().CommitSignatureStatus(mock.Anything, "never-pushed-sha").Return("B", signingCfg.Fingerprint, nil).Once()
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
		c.EXPECT().Commit("test commit message", signingCfg.KeyID).Return("", nil).Once()
		c.EXPECT().CommitSHA(mock.Anything).Return("never-pushed-sha", nil).Twice()
		c.EXPECT().CommitSignatureStatus(mock.Anything, "never-pushed-sha").Return("", "", errors.New("git boom")).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		_, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.Error(t, err)
		assert.ErrorContains(t, err, "verify signature")
	})

	t.Run("does not push when commit has no signature", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID).Return("", nil).Once()
		// "N" is git's %G? code for "no signature"; it must be rejected like any
		// other non-good status so an unsigned commit is never pushed.
		c.EXPECT().CommitSHA(mock.Anything).Return("never-pushed-sha", nil).Twice()
		c.EXPECT().CommitSignatureStatus(mock.Anything, "never-pushed-sha").Return("N", "", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		_, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.Error(t, err)
		assert.ErrorContains(t, err, "signature status")
		// Push is intentionally NOT in the mock expectations — if the impl
		// reaches push, the mocks.Client strict mode in NewClient will fail.
	})

	t.Run("does not push when signed by a different key", func(t *testing.T) {
		t.Parallel()
		service, mockRepoClientFactory := newServiceWithMocksAndSigning(t, signingCfg)
		c := gitmocks.NewClient(t)
		commonMockExpectations(c)
		c.EXPECT().Commit("test commit message", signingCfg.KeyID).Return("", nil).Once()
		c.EXPECT().CommitSHA(mock.Anything).Return("never-pushed-sha", nil).Twice()
		c.EXPECT().CommitSignatureStatus(mock.Anything, "never-pushed-sha").Return("G", "DEADBEEFDEADBEEF", nil).Once()
		mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(c, nil).Once()

		_, err := service.CommitHydratedManifests(t.Context(), baseRequest)
		require.Error(t, err)
		assert.ErrorContains(t, err, "signed by key")
	})
}

// gcpServiceAccountKey returns a service account key JSON with a freshly generated private key.
func gcpServiceAccountKey(t *testing.T, clientEmail string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyJSON, err := json.Marshal(map[string]string{
		"type":           "service_account",
		"project_id":     "my-google-project",
		"private_key_id": "key-id",
		"private_key":    string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})),
		"client_email":   clientEmail,
		"client_id":      "1234567890",
		"auth_uri":       "https://accounts.google.com/o/oauth2/auth",
		"token_uri":      "https://oauth2.googleapis.com/token",
	})
	require.NoError(t, err)
	return string(keyJSON)
}

// Test_CommitHydratedManifests_AuthorFromCredentials verifies the author precedence: request values first, then the
// identity of the repository credentials, then the "Argo CD" defaults.
func Test_CommitHydratedManifests_AuthorFromCredentials(t *testing.T) {
	t.Parallel()

	clientEmail := "argocd-service-account@my-google-project.iam.gserviceaccount.com"

	tests := []struct {
		name          string
		gcpKey        string
		httpsAuth     bool
		authorName    string
		authorEmail   string
		expectedName  string
		expectedEmail string
	}{
		{
			name:          "credentials identity used when request has no author",
			gcpKey:        gcpServiceAccountKey(t, clientEmail),
			expectedName:  clientEmail,
			expectedEmail: clientEmail,
		},
		{
			name:          "request values take precedence over credentials identity",
			gcpKey:        gcpServiceAccountKey(t, clientEmail),
			authorName:    "Custom Author",
			authorEmail:   "custom@example.com",
			expectedName:  "Custom Author",
			expectedEmail: "custom@example.com",
		},
		{
			name:          "credentials identity fills in only the missing email",
			gcpKey:        gcpServiceAccountKey(t, clientEmail),
			authorName:    "Custom Author",
			expectedName:  "Custom Author",
			expectedEmail: clientEmail,
		},
		{
			name:          "defaults used when credentials have a username but no email",
			httpsAuth:     true,
			expectedName:  "Argo CD",
			expectedEmail: "argo-cd@example.com",
		},
		{
			name:          "defaults used when credentials identity cannot be read",
			gcpKey:        "not-json",
			expectedName:  "Argo CD",
			expectedEmail: "argo-cd@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service, mockRepoClientFactory := newServiceWithMocks(t)

			mockGitClient := gitmocks.NewClient(t)
			mockGitClient.EXPECT().Init().Return(nil).Once()
			mockGitClient.EXPECT().Fetch(mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			mockGitClient.EXPECT().SetAuthor(mock.Anything, tt.expectedName, tt.expectedEmail).Return("", nil).Once()
			mockGitClient.EXPECT().CheckoutOrOrphan(mock.Anything, "env/test", false).Return("", nil).Once()
			mockGitClient.EXPECT().CheckoutOrNew(mock.Anything, "main", "env/test", false).Return("", nil).Once()
			mockGitClient.EXPECT().GetCommitNote(mock.Anything, mock.Anything, mock.Anything).Return("", fmt.Errorf("test %w", git.ErrNoNoteFound)).Once()
			mockGitClient.EXPECT().AddAndPushNote(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			mockGitClient.EXPECT().CommitSHA(mock.Anything).Return("sha", nil).Once()
			mockRepoClientFactory.EXPECT().NewClient(mock.Anything, mock.Anything).Return(mockGitClient, nil).Once()

			repo := &v1alpha1.Repository{
				Repo:                 "https://source.developers.google.com/p/my-google-project/r/repo",
				GCPServiceAccountKey: tt.gcpKey,
			}
			if tt.httpsAuth {
				repo = &v1alpha1.Repository{Repo: "https://github.com/argoproj/argocd-example-apps.git", Username: "someone", Password: "secret"}
			}

			resp, err := service.CommitHydratedManifests(t.Context(), &apiclient.CommitHydratedManifestsRequest{
				Repo:          repo,
				TargetBranch:  "main",
				SyncBranch:    "env/test",
				CommitMessage: "test commit message",
				AuthorName:    tt.authorName,
				AuthorEmail:   tt.authorEmail,
			})
			require.NoError(t, err)
			assert.Equal(t, "sha", resp.HydratedSha)
		})
	}
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

// Test_CommitHydratedManifests_Signing_FullFlow drives the whole signed
// hydration flow against a real git client and a local origin repo — clone,
// checkout sync/target branches, write manifests, GPG-sign the commit, locally
// verify the signature, and push — then asserts the commit that lands in origin
// is signed by the configured key. This is the closest we can get to an e2e of
// the feature without standing up a full commit-server deployment. Requires gpg.
func Test_CommitHydratedManifests_Signing_FullFlow(t *testing.T) {
	keyData, fingerprint := gpgsigntest.GenerateSigningKey(t, "")

	// One GNUPGHOME, shared by the commit server (sign + verify) and the
	// post-push verification below.
	home := gpgsigntest.ShortTempDir(t)
	t.Setenv(common.EnvGnuPGHome, home)
	require.NoError(t, gpgsign.WriteAgentConfig(home))
	signingCfg, err := gpgsign.ImportSigningKey(t.Context(), keyData, "")
	require.NoError(t, err)
	require.Equal(t, fingerprint, signingCfg.Fingerprint)

	origin := newOriginRepo(t)

	service := NewService(git.NoopCredsStore{}, metrics.NewMetricsServer(), signingCfg)

	req := &apiclient.CommitHydratedManifestsRequest{
		Repo:          &v1alpha1.Repository{Repo: "file://" + origin},
		TargetBranch:  "hydrated",
		SyncBranch:    "env/test",
		CommitMessage: "hydrate manifests",
		DrySha:        "0123456789abcdef0123456789abcdef01234567",
		Paths: []*apiclient.PathDetails{{
			Path: ".",
			Manifests: []*apiclient.HydratedManifestDetails{{
				ManifestJSON: `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"x"}}`,
			}},
		}},
	}

	resp, err := service.CommitHydratedManifests(t.Context(), req)
	require.NoError(t, err)
	require.NotEmpty(t, resp.HydratedSha)

	// The commit that landed on the target branch in origin must carry a good
	// signature from the configured key.
	cmd := exec.CommandContext(t.Context(), "git", "-C", origin, "log", "-1", "--pretty=format:%G?:%GK", "hydrated")
	cmd.Env = append(os.Environ(), "GNUPGHOME="+home, "LANG=C")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git log: %s", out)

	status, key, found := strings.Cut(strings.TrimSpace(string(out)), ":")
	require.True(t, found, "unexpected signature output: %q", out)
	assert.Contains(t, []string{git.SignatureStatusGood, git.SignatureStatusGoodUnknownTrust}, status)
	assert.True(t, signingCfg.MatchesSigningKey(key), "commit signed by unexpected key %q", key)
}

// newOriginRepo creates a non-bare git repo seeded with an empty initial commit
// to act as the push target. denyCurrentBranch=updateInstead lets the commit
// server push branches and notes into it.
func newOriginRepo(t *testing.T) string {
	t.Helper()
	dir := gpgsigntest.ShortTempDir(t)
	run := func(args ...string) {
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)
	}
	run("init", ".")
	run("config", "user.email", "origin@argo-cd.invalid")
	run("config", "user.name", "Origin")
	run("config", "receive.denyCurrentBranch", "updateInstead")
	run("commit", "--allow-empty", "-m", "init")
	return dir
}
