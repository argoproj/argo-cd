package commit

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	appsv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// tempRoot creates a temporary directory and returns an os.Root object for it.
// We use this instead of t.TempDir() because OSX does weird things with temp directories, and it triggers
// the os.Root protections.
func tempRoot(t *testing.T) *os.Root {
	t.Helper()

	dir, err := os.MkdirTemp(".", "")
	require.NoError(t, err)
	t.Cleanup(func() {
		err := os.RemoveAll(dir)
		require.NoError(t, err)
	})
	root, err := os.OpenRoot(dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		err := root.Close()
		require.NoError(t, err)
	})
	return root
}

func TestWriteForPaths(t *testing.T) {
	root := tempRoot(t)

	repoURL := "https://github.com/example/repo"
	drySha := "abc123"
	paths := []*apiclient.PathDetails{
		{
			Path: "path1",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Pod","apiVersion":"v1"}`},
			},
			Commands: []string{"command1", "command2"},
		},
		{
			Path: "path2",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Service","apiVersion":"v1"}`},
			},
			Commands: []string{"command3"},
		},
		{
			Path: "path3/nested",
			Manifests: []*apiclient.HydratedManifestDetails{
				{ManifestJSON: `{"kind":"Deployment","apiVersion":"apps/v1"}`},
			},
			Commands: []string{"command4"},
		},
	}

	now := metav1.NewTime(time.Now())
	metadata := &appsv1.RevisionMetadata{
		Author: "test-author",
		Date:   &now,
		Message: `test-message

Signed-off-by: Test User <test@example.com>
Argocd-reference-commit-sha: abc123
`,
		References: []appsv1.RevisionReference{
			{
				Commit: &appsv1.CommitMetadata{
					Author:  "test-code-author <test-email-author@example.com>",
					Date:    now.Format(time.RFC3339),
					Subject: "test-code-subject",
					SHA:     "test-code-sha",
					RepoURL: "https://example.com/test/repo.git",
				},
			},
		},
	}

	err := WriteForPaths(root, repoURL, drySha, metadata, paths)
	require.NoError(t, err)

	// Check if the top-level hydrator.metadata exists and contains the repo URL and dry SHA
	topMetadataPath := filepath.Join(root.Name(), "hydrator.metadata")
	topMetadataBytes, err := os.ReadFile(topMetadataPath)
	require.NoError(t, err)

	var topMetadata hydratorMetadataFile
	err = json.Unmarshal(topMetadataBytes, &topMetadata)
	require.NoError(t, err)
	assert.Equal(t, repoURL, topMetadata.RepoURL)
	assert.Equal(t, drySha, topMetadata.DrySHA)
	assert.Equal(t, metadata.Author, topMetadata.Author)
	assert.Equal(t, "test-message", topMetadata.Subject)
	// The body should exclude the Argocd- trailers.
	assert.Equal(t, "Signed-off-by: Test User <test@example.com>\n", topMetadata.Body)
	assert.Equal(t, metadata.Date.Format(time.RFC3339), topMetadata.Date)
	assert.Equal(t, metadata.References, topMetadata.References)

	for _, p := range paths {
		fullHydratePath := filepath.Join(root.Name(), p.Path)

		// Check if each path directory exists
		assert.DirExists(t, fullHydratePath)

		// Check if each path contains a hydrator.metadata file and contains the repo URL
		metadataPath := path.Join(fullHydratePath, "hydrator.metadata")
		metadataBytes, err := os.ReadFile(metadataPath)
		require.NoError(t, err)

		var readMetadata hydratorMetadataFile
		err = json.Unmarshal(metadataBytes, &readMetadata)
		require.NoError(t, err)
		assert.Equal(t, repoURL, readMetadata.RepoURL)
		// Check if each path contains a README.md file and contains the repo URL
		readmePath := path.Join(fullHydratePath, "README.md")
		readmeBytes, err := os.ReadFile(readmePath)
		require.NoError(t, err)
		assert.Contains(t, string(readmeBytes), repoURL)

		// Check if each path contains a manifest.yaml file and contains the word kind
		manifestPath := path.Join(fullHydratePath, "manifest.yaml")
		manifestBytes, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		assert.Contains(t, string(manifestBytes), "kind")
	}
}

func TestWriteMetadata(t *testing.T) {
	root := tempRoot(t)

	metadata := hydratorMetadataFile{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
	}

	err := writeMetadata(root, "", metadata)
	require.NoError(t, err)

	metadataPath := filepath.Join(root.Name(), "hydrator.metadata")
	metadataBytes, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var readMetadata hydratorMetadataFile
	err = json.Unmarshal(metadataBytes, &readMetadata)
	require.NoError(t, err)
	assert.Equal(t, metadata, readMetadata)
}

func TestWriteReadme(t *testing.T) {
	root := tempRoot(t)

	randomData := make([]byte, 32)
	_, err := rand.Read(randomData)
	require.NoError(t, err)
	hash := sha256.Sum256(randomData)
	sha := hex.EncodeToString(hash[:])

	metadata := hydratorMetadataFile{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
		References: []appsv1.RevisionReference{
			{
				Commit: &appsv1.CommitMetadata{
					Author:  "test-code-author <test@example.com>",
					Date:    time.Now().Format(time.RFC3339),
					Subject: "test-code-subject",
					SHA:     sha,
					RepoURL: "https://example.com/test/repo.git",
				},
			},
		},
	}

	err = writeReadme(root, "", metadata)
	require.NoError(t, err)

	readmePath := filepath.Join(root.Name(), "README.md")
	readmeBytes, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	assert.Equal(t, `# Manifest Hydration

To hydrate the manifests in this repository, run the following commands:

`+"```shell"+`
git clone https://github.com/example/repo
# cd into the cloned directory
git checkout abc123
`+"```"+fmt.Sprintf(`
## References

* [%s](https://example.com/test/repo.git): test-code-subject (test-code-author <test@example.com>)
`, sha[:7]), string(readmeBytes))
}

func TestWriteManifests(t *testing.T) {
	root := tempRoot(t)

	manifests := []*apiclient.HydratedManifestDetails{
		{ManifestJSON: `{"kind":"Pod","apiVersion":"v1"}`},
	}

	err := writeManifests(root, "", manifests)
	require.NoError(t, err)

	manifestPath := path.Join(root.Name(), "manifest.yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(manifestBytes), "kind")
}
