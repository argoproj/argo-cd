package repository

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	gitmocks "github.com/argoproj/argo-cd/v3/util/git/mocks"
	helmmocks "github.com/argoproj/argo-cd/v3/util/helm/mocks"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
	iomocks "github.com/argoproj/argo-cd/v3/util/io/mocks"
	"github.com/argoproj/argo-cd/v3/util/oci"
	ocimocks "github.com/argoproj/argo-cd/v3/util/oci/mocks"
)

func ociRefRequest(t *testing.T, repo string) refSourceResolveRequest {
	t.Helper()
	refSource := &v1alpha1.RefTarget{
		Repo:           v1alpha1.Repository{Repo: repo},
		TargetRevision: "v1.0.0",
	}
	return refSourceResolveRequest{
		q:                 &apiclient.ManifestRequest{ApplicationSource: &v1alpha1.ApplicationSource{}},
		refSourceMapping:  refSource,
		normalizedRepoURL: refSource.Repo.NormalizeRepoURL(),
		refVar:            "$ref",
		ociRefPaths:       utilio.NewRandomizedTempPaths(t.TempDir()),
	}
}

func TestResolveOCIRefSource(t *testing.T) {
	const repo = "oci://registry.example.com/chart"

	t.Run("OCI client creation fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, _ *iomocks.TempPaths) {}, ".")
		service.newOCIClient = func(_ string, _ oci.Creds, _ string, _ string, _ []string, _ ...oci.ClientOpts) (oci.Client, error) {
			return nil, errors.New("internal client failure")
		}

		_, closer, err := service.resolveOCIRefSource(t.Context(), ociRefRequest(t, repo))
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to create OCI client for repo")
		// The underlying cause must not leak into the client-facing error.
		assert.NotContains(t, err.Error(), "internal client failure")
	})

	t.Run("revision resolution fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("", errors.New("registry unreachable"))
		}, ".")

		_, closer, err := service.resolveOCIRefSource(t.Context(), ociRefRequest(t, repo))
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to resolve OCI revision v1.0.0")
		assert.NotContains(t, err.Error(), "registry unreachable")
	})

	t.Run("extraction fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("sha256:abc", nil)
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return("", nil, errors.New("layer digest mismatch"))
		}, ".")

		_, closer, err := service.resolveOCIRefSource(t.Context(), ociRefRequest(t, repo))
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to extract OCI image")
		assert.NotContains(t, err.Error(), "layer digest mismatch")
	})

	t.Run("out-of-bounds symlink is rejected", func(t *testing.T) {
		ociDir := t.TempDir()
		require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(ociDir, "link.yaml")))

		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("sha256:abc", nil)
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return(ociDir, utilio.NopCloser, nil)
		}, ".")

		req := ociRefRequest(t, repo)
		_, _, err := service.resolveOCIRefSource(t.Context(), req)
		require.ErrorContains(t, err, "oci image contains out-of-bounds symlinks")
		// The extracted path must not be registered when the symlink check fails.
		assert.Empty(t, req.ociRefPaths.GetPathIfExists(req.normalizedRepoURL))
	})

	t.Run("success registers the extracted path", func(t *testing.T) {
		ociDir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(ociDir, "values.yaml"), []byte("foo: bar"), 0o644))

		service, _, _ := newServiceWithOpt(t, func(_ *gitmocks.Client, _ *helmmocks.Client, ociClient *ocimocks.Client, _ *iomocks.TempPaths) {
			ociClient.EXPECT().ResolveRevision(mock.Anything, "v1.0.0", mock.Anything).Return("sha256:abc", nil)
			ociClient.EXPECT().Extract(mock.Anything, "sha256:abc").Return(ociDir, utilio.NopCloser, nil)
		}, ".")

		req := ociRefRequest(t, repo)
		ref, closer, err := service.resolveOCIRefSource(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, closer)
		assert.Equal(t, "v1.0.0", ref.revision)
		assert.Equal(t, "sha256:abc", ref.commitSHA)
		assert.Equal(t, "$ref", ref.key)
		assert.Equal(t, ociDir, req.ociRefPaths.GetPathIfExists(req.normalizedRepoURL))
	})
}

func TestResolveGitRefSource(t *testing.T) {
	const repo = "https://github.com/test/repo.git"

	gitRefRequest := func() refSourceResolveRequest {
		refSource := &v1alpha1.RefTarget{
			Repo:           v1alpha1.Repository{Repo: repo},
			TargetRevision: "main",
		}
		return refSourceResolveRequest{
			q: &apiclient.ManifestRequest{
				ApplicationSource: &v1alpha1.ApplicationSource{RepoURL: repo},
				Revision:          "primary-revision",
			},
			refSourceMapping:  refSource,
			normalizedRepoURL: refSource.Repo.NormalizeRepoURL(),
			refVar:            "$ref",
			commitSHA:         "primary-sha",
		}
	}

	t.Run("git client resolution fails", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(t.TempDir(), nil)
			gitClient.EXPECT().Root().Return("/tmp/repo")
			gitClient.EXPECT().LsRemote(mock.Anything).Return("", errors.New("auth required"))
		}, ".")

		_, closer, err := service.resolveGitRefSource(t.Context(), gitRefRequest())
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "failed to get git client for repo")
		assert.NotContains(t, err.Error(), "auth required")
	})

	t.Run("referencing a different revision of the same repository is rejected", func(t *testing.T) {
		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(t.TempDir(), nil)
			gitClient.EXPECT().Root().Return("/tmp/repo")
			// Resolves to a commit that differs from the primary source's commitSHA.
			gitClient.EXPECT().LsRemote(mock.Anything).Return("referenced-sha", nil)
		}, ".")

		_, closer, err := service.resolveGitRefSource(t.Context(), gitRefRequest())
		assert.Nil(t, closer)
		require.ErrorContains(t, err, "cannot reference a different revision of the same repository")
	})

	t.Run("out-of-bounds symlink is rejected after checkout", func(t *testing.T) {
		root := t.TempDir()
		require.NoError(t, os.Symlink("/etc/passwd", filepath.Join(root, "link.yaml")))

		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(root, nil)
			gitClient.EXPECT().Root().Return(root)
			gitClient.EXPECT().LsRemote(mock.Anything).Return("primary-sha", nil)
			gitClient.EXPECT().Init().Return(nil)
			gitClient.EXPECT().IsRevisionPresent(mock.Anything, mock.Anything).Return(true)
			gitClient.EXPECT().Checkout(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
		}, ".")

		req := gitRefRequest()
		req.q.NoCache = true // bypass the symlink-check cache so this test is self-contained
		_, _, err := service.resolveGitRefSource(t.Context(), req)
		require.ErrorContains(t, err, "repository contains out-of-bounds symlinks")
	})

	t.Run("success returns a repoRef and a closer", func(t *testing.T) {
		root := t.TempDir()

		service, _, _ := newServiceWithOpt(t, func(gitClient *gitmocks.Client, _ *helmmocks.Client, _ *ocimocks.Client, paths *iomocks.TempPaths) {
			paths.EXPECT().GetPath(mock.Anything).Return(root, nil)
			gitClient.EXPECT().Root().Return(root)
			gitClient.EXPECT().LsRemote(mock.Anything).Return("primary-sha", nil)
			gitClient.EXPECT().Init().Return(nil)
			gitClient.EXPECT().IsRevisionPresent(mock.Anything, mock.Anything).Return(true)
			gitClient.EXPECT().Checkout(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return("", nil)
		}, ".")

		req := gitRefRequest()
		req.q.NoCache = true
		ref, closer, err := service.resolveGitRefSource(t.Context(), req)
		require.NoError(t, err)
		require.NotNil(t, closer)
		defer closeAndLog(closer, "test lock")
		assert.Equal(t, "main", ref.revision)
		assert.Equal(t, "primary-sha", ref.commitSHA)
		assert.Equal(t, "$ref", ref.key)
	})
}
