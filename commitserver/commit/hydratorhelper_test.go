package commit

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
)

func TestWriteForPaths(t *testing.T) {
	dir := t.TempDir()

	repoUrl := "https://github.com/example/repo"
	drySha := "abc123"
	paths := []*apiclient.PathDetails{
		{
			Path: "path1",
			Manifests: []*apiclient.ManifestDetails{
				{Manifest: `{"kind":"Pod","apiVersion":"v1"}`},
			},
			Commands: []string{"command1", "command2"},
		},
		{
			Path: "path2",
			Manifests: []*apiclient.ManifestDetails{
				{Manifest: `{"kind":"Service","apiVersion":"v1"}`},
			},
			Commands: []string{"command3"},
		},
	}

	err := WriteForPaths(dir, repoUrl, drySha, paths)
	require.NoError(t, err)

	topMetadataPath := path.Join(dir, "hydrator.metadata")
	topMetadataBytes, err := os.ReadFile(topMetadataPath)
	require.NoError(t, err)

	var topMetadata hydratorMetadataFile
	err = json.Unmarshal(topMetadataBytes, &topMetadata)
	require.NoError(t, err)
	assert.Equal(t, hydratorMetadataFile{RepoURL: repoUrl, DrySHA: drySha}, topMetadata)

	for _, p := range paths {
		fullHydratePath, err := securejoin.SecureJoin(dir, p.Path)
		require.NoError(t, err)

		metadataPath := path.Join(fullHydratePath, "hydrator.metadata")
		metadataBytes, err := os.ReadFile(metadataPath)
		require.NoError(t, err)

		var readMetadata hydratorMetadataFile
		err = json.Unmarshal(metadataBytes, &readMetadata)
		require.NoError(t, err)
		assert.Equal(t, hydratorMetadataFile{
			Commands: p.Commands,
			DrySHA:   drySha,
			RepoURL:  repoUrl,
		}, readMetadata)

		readmePath := path.Join(fullHydratePath, "README.md")
		readmeBytes, err := os.ReadFile(readmePath)
		require.NoError(t, err)
		assert.Contains(t, string(readmeBytes), repoUrl)

		manifestPath := path.Join(fullHydratePath, "manifest.yaml")
		manifestBytes, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		for _, m := range p.Manifests {
			obj := &unstructured.Unstructured{}
			err := json.Unmarshal([]byte(m.Manifest), obj)
			require.NoError(t, err)
			assert.Contains(t, string(manifestBytes), obj.GetKind())
		}
	}
}

func TestWriteMetadata(t *testing.T) {
	dir := t.TempDir()

	metadata := hydratorMetadataFile{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
	}

	err := writeMetadata(dir, metadata)
	require.NoError(t, err)

	metadataPath := path.Join(dir, "hydrator.metadata")
	metadataBytes, err := os.ReadFile(metadataPath)
	require.NoError(t, err)

	var readMetadata hydratorMetadataFile
	err = json.Unmarshal(metadataBytes, &readMetadata)
	require.NoError(t, err)
	assert.Equal(t, metadata, readMetadata)
}

func TestWriteReadme(t *testing.T) {
	dir := t.TempDir()

	metadata := hydratorMetadataFile{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
	}

	err := writeReadme(dir, metadata)
	require.NoError(t, err)

	readmePath := path.Join(dir, "README.md")
	readmeBytes, err := os.ReadFile(readmePath)
	require.NoError(t, err)
	assert.Contains(t, string(readmeBytes), metadata.RepoURL)
}

func TestWriteManifests(t *testing.T) {
	dir := t.TempDir()

	manifests := []*apiclient.ManifestDetails{
		{Manifest: `{"kind":"Pod","apiVersion":"v1"}`},
	}

	err := writeManifests(dir, manifests)
	require.NoError(t, err)

	manifestPath := path.Join(dir, "manifest.yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(manifestBytes), "kind: Pod")
}
