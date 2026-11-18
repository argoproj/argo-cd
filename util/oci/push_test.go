package oci

import (
	"context"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	imagev1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/content/memory"

	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

const (
	testArtifactType = "application/vnd.argoproj.test.v1"
	testCreated      = "2026-01-01T00:00:00Z"
)

// writeFiles creates the given files (slash separated relative path -> content) in dir.
func writeFiles(t *testing.T, dir string, contents map[string]string) {
	t.Helper()
	for name, data := range contents {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(data), 0o644))
	}
}

// readFiles returns all files below dir (slash separated relative path -> content).
func readFiles(t *testing.T, dir string) map[string]string {
	t.Helper()
	contents := map[string]string{}
	require.NoError(t, filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		contents[filepath.ToSlash(rel)] = string(data)
		return nil
	}))
	return contents
}

func Test_pusher_PushDirectory(t *testing.T) {
	t.Run("pushed artifact can be extracted by the read client", func(t *testing.T) {
		contents := map[string]string{
			"manifest.yaml":           "kind: ConfigMap\n",
			"guestbook/manifest.yaml": "kind: Deployment\n",
			"guestbook/README.md":     "# guestbook\n",
		}
		dir := t.TempDir()
		writeFiles(t, dir, contents)
		require.NoError(t, os.Mkdir(filepath.Join(dir, "empty"), 0o755))

		store := memory.New()
		digest, err := NewPusherForTarget(store).PushDirectory(t.Context(), dir, "main", PushOptions{ArtifactType: testArtifactType})
		require.NoError(t, err)

		// A registry resolves digests on its own, the in-memory store only resolves references that were tagged.
		desc, err := store.Resolve(t.Context(), "main")
		require.NoError(t, err)
		require.NoError(t, store.Tag(t.Context(), desc, digest))

		repoURL := "example.com/hydrated"
		c := newClientWithLock(repoURL, globalLock, store, nil, func(_ context.Context) error {
			return nil
		}, []string{imagev1.MediaTypeImageLayerGzip},
			WithImagePaths(utilio.NewRandomizedTempPaths(t.TempDir())),
			WithManifestMaxExtractedSize(10000),
			WithEventHandlers(fakeEventHandlers(t, repoURL)))

		manifest, err := c.DigestMetadata(t.Context(), digest)
		require.NoError(t, err)
		assert.Equal(t, testArtifactType, manifest.ArtifactType)
		require.Len(t, manifest.Layers, 1)
		assert.Equal(t, imagev1.MediaTypeImageLayerGzip, manifest.Layers[0].MediaType)
		// What `oras pull` needs to unpack the layer into the working directory.
		assert.Equal(t, ".", manifest.Layers[0].Annotations[imagev1.AnnotationTitle])
		assert.Equal(t, "true", manifest.Layers[0].Annotations[file.AnnotationUnpack])
		assert.NotEmpty(t, manifest.Layers[0].Annotations[file.AnnotationDigest])

		path, closer, err := c.Extract(t.Context(), digest)
		require.NoError(t, err)
		defer utilio.Close(closer)

		assert.Equal(t, contents, readFiles(t, path))
		assert.DirExists(t, filepath.Join(path, "empty"))
	})

	t.Run("digest depends only on file content and annotations", func(t *testing.T) {
		contents := map[string]string{
			"manifest.yaml":           "kind: ConfigMap\n",
			"guestbook/manifest.yaml": "kind: Deployment\n",
		}
		opts := PushOptions{
			ArtifactType: testArtifactType,
			Annotations:  map[string]string{imagev1.AnnotationCreated: testCreated, "example.com/dry-sha": "abc123"},
		}

		dir := t.TempDir()
		writeFiles(t, dir, contents)
		digest, err := NewPusherForTarget(memory.New()).PushDirectory(t.Context(), dir, "main", opts)
		require.NoError(t, err)

		// Same content, but with different timestamps and permissions.
		otherDir := t.TempDir()
		writeFiles(t, otherDir, contents)
		old := time.Date(2001, time.February, 3, 4, 5, 6, 0, time.UTC)
		require.NoError(t, filepath.WalkDir(otherDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				if err := os.Chmod(path, 0o600); err != nil {
					return err
				}
			}
			return os.Chtimes(path, old, old)
		}))
		otherDigest, err := NewPusherForTarget(memory.New()).PushDirectory(t.Context(), otherDir, "main", opts)
		require.NoError(t, err)
		assert.Equal(t, digest, otherDigest)

		changedDir := t.TempDir()
		writeFiles(t, changedDir, contents)
		writeFiles(t, changedDir, map[string]string{"guestbook/manifest.yaml": "kind: StatefulSet\n"})
		changedDigest, err := NewPusherForTarget(memory.New()).PushDirectory(t.Context(), changedDir, "main", opts)
		require.NoError(t, err)
		assert.NotEqual(t, digest, changedDigest)
	})

	t.Run("pushing to an existing tag moves the tag", func(t *testing.T) {
		opts := PushOptions{
			ArtifactType: testArtifactType,
			Annotations:  map[string]string{imagev1.AnnotationCreated: testCreated},
		}
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"manifest.yaml": "kind: ConfigMap\n"})

		p := NewPusherForTarget(memory.New())
		firstDigest, err := p.PushDirectory(t.Context(), dir, "main", opts)
		require.NoError(t, err)

		// Pushing content the repository already has is not an error and leaves the tag where it is.
		repeatedDigest, err := p.PushDirectory(t.Context(), dir, "main", opts)
		require.NoError(t, err)
		assert.Equal(t, firstDigest, repeatedDigest)

		writeFiles(t, dir, map[string]string{"manifest.yaml": "kind: Secret\n"})
		secondDigest, err := p.PushDirectory(t.Context(), dir, "main", opts)
		require.NoError(t, err)
		require.NotEqual(t, firstDigest, secondDigest)

		resolved, _, err := p.ResolveTag(t.Context(), "main")
		require.NoError(t, err)
		assert.Equal(t, secondDigest, resolved)
	})

	t.Run("invalid arguments", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"manifest.yaml": "kind: ConfigMap\n"})
		p := NewPusherForTarget(memory.New())

		_, err := p.PushDirectory(t.Context(), dir, "", PushOptions{ArtifactType: testArtifactType})
		require.ErrorContains(t, err, "tag must not be empty")

		_, err = p.PushDirectory(t.Context(), dir, "main", PushOptions{})
		require.ErrorContains(t, err, "artifact type must not be empty")

		_, err = p.PushDirectory(t.Context(), filepath.Join(dir, "manifest.yaml"), "main", PushOptions{ArtifactType: testArtifactType})
		require.ErrorContains(t, err, "not a directory")

		_, _, err = p.ResolveTag(t.Context(), "main")
		require.ErrorIs(t, err, ErrTagNotFound)
	})

	t.Run("symlinks are rejected", func(t *testing.T) {
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"manifest.yaml": "kind: ConfigMap\n"})
		require.NoError(t, os.Symlink("manifest.yaml", filepath.Join(dir, "link.yaml")))
		p := NewPusherForTarget(memory.New())

		_, err := p.PushDirectory(t.Context(), dir, "main", PushOptions{ArtifactType: testArtifactType})
		require.ErrorContains(t, err, `unsupported file type L--------- for "link.yaml"`)

		_, _, err = p.ResolveTag(t.Context(), "main")
		require.ErrorIs(t, err, ErrTagNotFound)
	})
}

func Test_pusher_ResolveTag(t *testing.T) {
	t.Run("missing tag", func(t *testing.T) {
		_, _, err := NewPusherForTarget(memory.New()).ResolveTag(t.Context(), "main")
		require.ErrorIs(t, err, ErrTagNotFound)
	})

	t.Run("existing tag", func(t *testing.T) {
		annotations := map[string]string{imagev1.AnnotationCreated: testCreated, "example.com/dry-sha": "abc123"}
		dir := t.TempDir()
		writeFiles(t, dir, map[string]string{"manifest.yaml": "kind: ConfigMap\n"})

		p := NewPusherForTarget(memory.New())
		pushed, err := p.PushDirectory(t.Context(), dir, "main", PushOptions{ArtifactType: testArtifactType, Annotations: annotations})
		require.NoError(t, err)

		resolved, resolvedAnnotations, err := p.ResolveTag(t.Context(), "main")
		require.NoError(t, err)
		assert.Equal(t, pushed, resolved)
		assert.True(t, strings.HasPrefix(resolved, "sha256:"), "expected a sha256 digest, got %q", resolved)
		assert.Equal(t, annotations, resolvedAnnotations)
	})
}

func TestNewPusher_ResolveTag(t *testing.T) {
	tests := []struct {
		name           string
		manifestStatus int
		tagNotFound    bool
	}{
		{name: "registry does not know the tag", manifestStatus: http.StatusNotFound, tagNotFound: true},
		{name: "registry fails", manifestStatus: http.StatusInternalServerError, tagNotFound: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				username, password, ok := r.BasicAuth()
				switch {
				case !ok:
					w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
					w.WriteHeader(http.StatusUnauthorized)
				case username != "user" || password != "pass":
					w.WriteHeader(http.StatusForbidden)
				case r.URL.Path == "/v2/myorg/myrepo/manifests/main":
					w.WriteHeader(tt.manifestStatus)
				default:
					w.WriteHeader(http.StatusTeapot)
				}
			}))
			t.Cleanup(server.Close)

			repoURL := "oci://" + strings.TrimPrefix(server.URL, "http://") + "/myorg/myrepo"
			p, err := NewPusher(repoURL, Creds{Username: "user", Password: "pass", InsecureHTTPOnly: true}, "", "")
			require.NoError(t, err)

			_, _, err = p.ResolveTag(t.Context(), "main")
			if tt.tagNotFound {
				require.ErrorIs(t, err, ErrTagNotFound)
				return
			}
			require.Error(t, err)
			require.NotErrorIs(t, err, ErrTagNotFound)
		})
	}
}
