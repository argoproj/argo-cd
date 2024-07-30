package commit

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
)

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
