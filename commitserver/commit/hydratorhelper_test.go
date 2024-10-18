package commit

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
)

func TestWriteForPaths(t *testing.T) {
	dir := t.TempDir()

	repoUrl := "https://github.com/example/repo"
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
	}

	err := WriteForPaths(dir, repoUrl, drySha, paths)
	require.NoError(t, err)

	// Check if the top-level hydrator.metadata exists and contains the repo URL and dry SHA
	topMetadataPath := path.Join(dir, "hydrator.metadata")
	topMetadataBytes, err := os.ReadFile(topMetadataPath)
	require.NoError(t, err)

	var topMetadata hydratorMetadataFile
	err = json.Unmarshal(topMetadataBytes, &topMetadata)
	require.NoError(t, err)
	assert.Equal(t, repoUrl, topMetadata.RepoURL)
	assert.Equal(t, drySha, topMetadata.DrySHA)

	for _, p := range paths {
		fullHydratePath, err := securejoin.SecureJoin(dir, p.Path)
		require.NoError(t, err)

		// Check if each path directory exists
		assert.DirExists(t, fullHydratePath)

		// Check if each path contains a hydrator.metadata file and contains the repo URL
		metadataPath := path.Join(fullHydratePath, "hydrator.metadata")
		metadataBytes, err := os.ReadFile(metadataPath)
		require.NoError(t, err)

		var readMetadata hydratorMetadataFile
		err = json.Unmarshal(metadataBytes, &readMetadata)
		require.NoError(t, err)
		assert.Equal(t, repoUrl, readMetadata.RepoURL)

		// Check if each path contains a README.md file and contains the repo URL
		readmePath := path.Join(fullHydratePath, "README.md")
		readmeBytes, err := os.ReadFile(readmePath)
		require.NoError(t, err)
		assert.Contains(t, string(readmeBytes), repoUrl)

		// Check if each path contains a manifest.yaml file and contains the word Pod
		manifestPath := path.Join(fullHydratePath, "manifest.yaml")
		manifestBytes, err := os.ReadFile(manifestPath)
		require.NoError(t, err)
		assert.Contains(t, string(manifestBytes), "kind")
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

	manifests := []*apiclient.HydratedManifestDetails{
		{ManifestJSON: `{"kind":"Pod","apiVersion":"v1"}`},
	}

	err := writeManifests(dir, manifests)
	require.NoError(t, err)

	manifestPath := path.Join(dir, "manifest.yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	require.NoError(t, err)
	assert.Contains(t, string(manifestBytes), "kind")
}
