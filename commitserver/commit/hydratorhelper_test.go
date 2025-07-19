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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v3/commitserver/apiclient"
	"github.com/argoproj/argo-cd/v3/common"
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

func TestWriteGitAttributes(t *testing.T) {
	root := tempRoot(t)

	err := writeGitAttributes(root)
	require.NoError(t, err)

	gitAttributesPath := filepath.Join(root.Name(), ".gitattributes")
	gitAttributesBytes, err := os.ReadFile(gitAttributesPath)
	require.NoError(t, err)
	assert.Contains(t, string(gitAttributesBytes), "*/README.md linguist-generated=true")
	assert.Contains(t, string(gitAttributesBytes), "*/hydrator.metadata linguist-generated=true")
}

func TestStripTrackingMetadata(t *testing.T) {
	tests := []struct {
		name                string
		input               *unstructured.Unstructured
		expectedAnnotations map[string]string
		expectedLabels      map[string]string
		description         string
	}{
		{
			name: "Remove tracking annotations and labels but keep others",
			input: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"metadata": map[string]any{
						"name":      "test-configmap",
						"namespace": "default",
						"annotations": map[string]any{
							common.AnnotationKeyAppInstance: "test-app:/ConfigMap:default/test-configmap",
							common.AnnotationInstallationID: "test-installation-id",
							"other-annotation":              "should-remain",
						},
						"labels": map[string]any{
							common.LabelKeyAppInstance:           "test-app",
							common.LabelKeyLegacyApplicationName: "test-app",
							"other-label":                        "should-remain",
						},
					},
				},
			},
			expectedAnnotations: map[string]string{
				"other-annotation": "should-remain",
			},
			expectedLabels: map[string]string{
				"other-label": "should-remain",
			},
			description: "Should remove tracking annotations and labels but preserve others",
		},
		{
			name: "Remove all metadata when only tracking metadata exists",
			input: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Service",
					"metadata": map[string]any{
						"name":      "test-service",
						"namespace": "default",
						"annotations": map[string]any{
							common.AnnotationKeyAppInstance: "test-app:/Service:default/test-service",
						},
						"labels": map[string]any{
							common.LabelKeyAppInstance: "test-app",
						},
					},
				},
			},
			expectedAnnotations: nil,
			expectedLabels:      nil,
			description:         "Should remove metadata maps entirely when only tracking metadata exists",
		},
		{
			name: "Handle object with no metadata",
			input: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]any{
						"name":      "test-pod",
						"namespace": "default",
					},
				},
			},
			expectedAnnotations: nil,
			expectedLabels:      nil,
			description:         "Should handle objects with no metadata gracefully",
		},
		{
			name: "Handle installation ID only",
			input: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "Secret",
					"metadata": map[string]any{
						"name":      "test-secret",
						"namespace": "default",
						"annotations": map[string]any{
							common.AnnotationInstallationID: "test-installation-id",
							"keep-this":                     "annotation",
						},
					},
				},
			},
			expectedAnnotations: map[string]string{
				"keep-this": "annotation",
			},
			expectedLabels: nil,
			description:    "Should remove installation ID annotation only",
		},
		{
			name: "Handle List kind recursively",
			input: &unstructured.Unstructured{
				Object: map[string]any{
					"apiVersion": "v1",
					"kind":       "List",
					"items": []any{
						map[string]any{
							"apiVersion": "v1",
							"kind":       "ConfigMap",
							"metadata": map[string]any{
								"name":      "item1",
								"namespace": "default",
								"annotations": map[string]any{
									common.AnnotationKeyAppInstance: "test-app:/ConfigMap:default/item1",
									"keep-this":                     "annotation",
								},
								"labels": map[string]any{
									common.LabelKeyAppInstance: "test-app",
									"keep-this":                "label",
								},
							},
						},
						map[string]any{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata": map[string]any{
								"name":      "item2",
								"namespace": "default",
								"annotations": map[string]any{
									common.AnnotationKeyAppInstance: "test-app:/Service:default/item2",
								},
							},
						},
					},
				},
			},
			expectedAnnotations: nil,
			expectedLabels:      nil,
			description:         "Should handle List kind and strip tracking metadata from all items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stripTrackingMetadata(tt.input)

			annotations := tt.input.GetAnnotations()
			labels := tt.input.GetLabels()

			if tt.expectedAnnotations == nil {
				assert.Nil(t, annotations, tt.description+" (annotations)")
			} else {
				assert.Equal(t, tt.expectedAnnotations, annotations, tt.description+" (annotations)")
			}

			if tt.expectedLabels == nil {
				assert.Nil(t, labels, tt.description+" (labels)")
			} else {
				assert.Equal(t, tt.expectedLabels, labels, tt.description+" (labels)")
			}

			// Verify tracking metadata is definitely removed
			if annotations != nil {
				assert.NotContains(t, annotations, common.AnnotationKeyAppInstance, "Tracking ID annotation should be removed")
				assert.NotContains(t, annotations, common.AnnotationInstallationID, "Installation ID annotation should be removed")
			}
			if labels != nil {
				assert.NotContains(t, labels, common.LabelKeyAppInstance, "App instance label should be removed")
				assert.NotContains(t, labels, common.LabelKeyLegacyApplicationName, "Legacy app name label should be removed")
			}

			// For List kind, verify items are also cleaned
			if tt.input.GetKind() == "List" {
				items, found, err := unstructured.NestedSlice(tt.input.Object, "items")
				require.NoError(t, err)
				if found {
					for i, item := range items {
						itemMap, ok := item.(map[string]any)
						if !ok {
							continue
						}
						itemObj := &unstructured.Unstructured{Object: itemMap}
						itemAnnotations := itemObj.GetAnnotations()
						itemLabels := itemObj.GetLabels()

						if itemAnnotations != nil {
							assert.NotContains(t, itemAnnotations, common.AnnotationKeyAppInstance,
								"Item %d should not have tracking annotation", i)
							assert.NotContains(t, itemAnnotations, common.AnnotationInstallationID,
								"Item %d should not have installation ID annotation", i)
						}
						if itemLabels != nil {
							assert.NotContains(t, itemLabels, common.LabelKeyAppInstance,
								"Item %d should not have app instance label", i)
							assert.NotContains(t, itemLabels, common.LabelKeyLegacyApplicationName,
								"Item %d should not have legacy app name label", i)
						}
					}
				}
			}
		})
	}
}
