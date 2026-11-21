package commit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"oras.land/oras-go/v2/content"
	"oras.land/oras-go/v2/content/memory"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/util/gpgsign"
	"github.com/argoproj/argo-cd/v3/util/hydrator"
	"github.com/argoproj/argo-cd/v3/util/io/files"
	"github.com/argoproj/argo-cd/v3/util/oci"
)

const (
	testOCIRepo    = "oci://example.com/hydrated"
	testDryRepo    = "https://example.com/dry"
	testConfigMapA = `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"a"}}`
	testConfigMapB = `{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"b"}}`
)

var testDryCommitDate = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// newOCIServiceWithStore returns a service which pushes OCI artifacts to an in-memory store. The mocked git client
// factory has no expectations, so the test fails if an OCI request ever falls through to the git code path.
func newOCIServiceWithStore(t *testing.T) (*Service, *memory.Store) {
	t.Helper()
	service, _ := newServiceWithMocks(t)
	store := memory.New()
	service.newOCIPusher = func(repo *v1alpha1.Repository) (oci.Pusher, error) {
		assert.Equal(t, testOCIRepo, repo.Repo)
		return oci.NewPusherForTarget(store), nil
	}
	return service, store
}

func newOCIRequest(drySha string, paths ...*apiclient.PathDetails) *apiclient.CommitHydratedManifestsRequest {
	return &apiclient.CommitHydratedManifestsRequest{
		Repo:         &v1alpha1.Repository{Repo: testOCIRepo},
		TargetBranch: "dev",
		DrySha:       drySha,
		DryRepoURL:   testDryRepo,
		DryCommitMetadata: &v1alpha1.RevisionMetadata{
			Author:  "Jane Doe <jane@example.com>",
			Date:    &metav1.Time{Time: testDryCommitDate},
			Message: "bump image\n\nsome body",
		},
		ReadmeMessage: "hydrated from {{ .RepoURL }} at {{ .DrySHA }}",
		Paths:         paths,
	}
}

func newOCIPath(path string, manifests ...string) *apiclient.PathDetails {
	details := &apiclient.PathDetails{Path: path, Commands: []string{"kustomize build " + path}}
	for _, manifest := range manifests {
		details.Manifests = append(details.Manifests, &apiclient.HydratedManifestDetails{ManifestJSON: manifest})
	}
	return details
}

// fetchOCIArtifact returns the OCI manifest the tag points to and a directory holding the extracted content of its
// only layer.
func fetchOCIArtifact(t *testing.T, store *memory.Store, tag string) (imagev1.Manifest, string) {
	t.Helper()
	desc, err := store.Resolve(t.Context(), tag)
	require.NoError(t, err)
	manifestBytes, err := content.FetchAll(t.Context(), store, desc)
	require.NoError(t, err)
	var manifest imagev1.Manifest
	require.NoError(t, json.Unmarshal(manifestBytes, &manifest))
	require.Len(t, manifest.Layers, 1)

	layer, err := content.FetchAll(t.Context(), store, manifest.Layers[0])
	require.NoError(t, err)
	dir := t.TempDir()
	require.NoError(t, files.Untgz(dir, bytes.NewReader(layer), 1024*1024, false))
	return manifest, dir
}

func readHydratorMetadata(t *testing.T, path string) hydrator.HydratorCommitMetadata {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var metadata hydrator.HydratorCommitMetadata
	require.NoError(t, json.Unmarshal(data, &metadata))
	return metadata
}

func Test_CommitHydratedManifests_OCI(t *testing.T) {
	t.Parallel()

	t.Run("creates the tag when it does not exist", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)

		resp, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA)))
		require.NoError(t, err)

		desc, err := store.Resolve(t.Context(), "dev")
		require.NoError(t, err)
		assert.Equal(t, desc.Digest.String(), resp.HydratedSha)

		manifest, dir := fetchOCIArtifact(t, store, "dev")
		assert.Equal(t, OCIArtifactType, manifest.ArtifactType)
		assert.Equal(t, imagev1.MediaTypeImageLayerGzip, manifest.Layers[0].MediaType)
		assert.Equal(t, "dry1", manifest.Annotations[imagev1.AnnotationRevision])
		assert.Equal(t, testDryRepo, manifest.Annotations[imagev1.AnnotationSource])
		assert.Equal(t, "Jane Doe <jane@example.com>", manifest.Annotations[imagev1.AnnotationAuthors])
		assert.Equal(t, "2026-01-02T03:04:05Z", manifest.Annotations[imagev1.AnnotationCreated])
		assert.NotEmpty(t, manifest.Annotations[OCIManifestsDigestAnnotation])

		manifestYaml, err := os.ReadFile(filepath.Join(dir, "guestbook", ManifestYaml))
		require.NoError(t, err)
		assert.Contains(t, string(manifestYaml), "name: a")
		readme, err := os.ReadFile(filepath.Join(dir, "guestbook", "README.md"))
		require.NoError(t, err)
		assert.Equal(t, "hydrated from "+testDryRepo+" at dry1", string(readme))
		assert.Equal(t, hydrator.HydratorCommitMetadata{
			RepoURL:  testDryRepo,
			DrySHA:   "dry1",
			Commands: []string{"kustomize build guestbook"},
		}, readHydratorMetadata(t, filepath.Join(dir, "guestbook", "hydrator.metadata")))
		topLevel := readHydratorMetadata(t, filepath.Join(dir, "hydrator.metadata"))
		assert.Equal(t, testDryRepo, topLevel.RepoURL)
		assert.Equal(t, "bump image", topLevel.Subject)
		assert.Empty(t, topLevel.Commands)
		assert.NoFileExists(t, filepath.Join(dir, ".gitattributes"))
	})

	t.Run("does not push when the manifests are unchanged", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)

		first, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA)))
		require.NoError(t, err)
		second, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry2", newOCIPath("guestbook", testConfigMapA)))
		require.NoError(t, err)

		assert.Equal(t, first.HydratedSha, second.HydratedSha)
		manifest, _ := fetchOCIArtifact(t, store, "dev")
		assert.Equal(t, "dry1", manifest.Annotations[imagev1.AnnotationRevision], "the artifact of the first dry commit must be left in place")
	})

	t.Run("replaces the artifact when the manifests change", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)

		first, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA), newOCIPath("other", testConfigMapB)))
		require.NoError(t, err)
		// The second dry commit changes one application and no longer contains the other.
		second, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry2", newOCIPath("guestbook", testConfigMapB)))
		require.NoError(t, err)

		assert.NotEqual(t, first.HydratedSha, second.HydratedSha)
		desc, err := store.Resolve(t.Context(), "dev")
		require.NoError(t, err)
		assert.Equal(t, desc.Digest.String(), second.HydratedSha)
		manifest, dir := fetchOCIArtifact(t, store, "dev")
		assert.Equal(t, "dry2", manifest.Annotations[imagev1.AnnotationRevision])
		manifestYaml, err := os.ReadFile(filepath.Join(dir, "guestbook", ManifestYaml))
		require.NoError(t, err)
		assert.Contains(t, string(manifestYaml), "name: b")
		assert.NoDirExists(t, filepath.Join(dir, "other"), "paths which are no longer hydrated must not be carried over")
	})

	t.Run("pushes when only the set of paths changes", func(t *testing.T) {
		t.Parallel()
		service, _ := newOCIServiceWithStore(t)

		first, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA)))
		require.NoError(t, err)
		second, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("renamed", testConfigMapA)))
		require.NoError(t, err)

		assert.NotEqual(t, first.HydratedSha, second.HydratedSha)
	})

	t.Run("hydrates to the root of the artifact", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)

		_, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath(".", testConfigMapA)))
		require.NoError(t, err)

		_, dir := fetchOCIArtifact(t, store, "dev")
		assert.FileExists(t, filepath.Join(dir, ManifestYaml))
		assert.FileExists(t, filepath.Join(dir, "README.md"))
		// At the root, the application metadata and the dry commit metadata are the same file.
		metadata := readHydratorMetadata(t, filepath.Join(dir, "hydrator.metadata"))
		assert.Equal(t, []string{"kustomize build ."}, metadata.Commands)
		assert.Equal(t, "bump image", metadata.Subject)
		assert.Equal(t, "Jane Doe <jane@example.com>", metadata.Author)
	})

	t.Run("pushes unsigned when commit signing is enabled", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)
		service.signingConfig = &gpgsign.Config{KeyID: "ABCDEF0123456789"}

		resp, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA)))
		require.NoError(t, err)

		desc, err := store.Resolve(t.Context(), "dev")
		require.NoError(t, err)
		assert.Equal(t, desc.Digest.String(), resp.HydratedSha)
	})

	t.Run("missing tag", func(t *testing.T) {
		t.Parallel()
		service, _ := newOCIServiceWithStore(t)
		request := newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA))
		request.TargetBranch = ""

		_, err := service.CommitHydratedManifests(t.Context(), request)
		require.EqualError(t, err, "target tag is required")
	})

	t.Run("two applications hydrated to the same path", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)

		_, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA), newOCIPath("./guestbook/", testConfigMapB)))
		require.ErrorContains(t, err, "more than one application is hydrated to path")

		_, err = store.Resolve(t.Context(), "dev")
		require.Error(t, err, "nothing must be pushed")
	})

	t.Run("path outside of the artifact", func(t *testing.T) {
		t.Parallel()
		service, store := newOCIServiceWithStore(t)

		_, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("../escape", testConfigMapA)))
		require.Error(t, err)

		_, err = store.Resolve(t.Context(), "dev")
		require.Error(t, err, "nothing must be pushed")
	})

	t.Run("does not push when the current state of the tag is unknown", func(t *testing.T) {
		t.Parallel()
		service, _ := newServiceWithMocks(t)
		pusher := &failingResolvePusher{}
		service.newOCIPusher = func(*v1alpha1.Repository) (oci.Pusher, error) {
			return pusher, nil
		}

		_, err := service.CommitHydratedManifests(t.Context(), newOCIRequest("dry1", newOCIPath("guestbook", testConfigMapA)))
		require.ErrorContains(t, err, "failed to resolve tag: registry unavailable")
		assert.False(t, pusher.pushed)
	})
}

// failingResolvePusher is a pusher whose registry cannot be asked what a tag points to.
type failingResolvePusher struct {
	pushed bool
}

func (p *failingResolvePusher) ResolveTag(context.Context, string) (string, map[string]string, error) {
	return "", nil, errors.New("registry unavailable")
}

func (p *failingResolvePusher) PushDirectory(context.Context, string, string, oci.PushOptions) (string, error) {
	p.pushed = true
	return "", nil
}
