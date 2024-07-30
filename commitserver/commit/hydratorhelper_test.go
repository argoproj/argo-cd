package commit

import (
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/argoproj/argo-cd/v2/commitserver/apiclient"
	"github.com/stretchr/testify/assert"
)

func TestWriteMetadata(t *testing.T) {
	dir := t.TempDir()

	metadata := hydratorMetadataFile{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
	}

	err := writeMetadata(dir, metadata)
	assert.NoError(t, err)

	metadataPath := path.Join(dir, "hydrator.metadata")
	metadataBytes, err := os.ReadFile(metadataPath)
	assert.NoError(t, err)

	var readMetadata hydratorMetadataFile
	err = json.Unmarshal(metadataBytes, &readMetadata)
	assert.NoError(t, err)
	assert.Equal(t, metadata, readMetadata)
}

func TestWriteReadme(t *testing.T) {
	dir := t.TempDir()

	metadata := hydratorMetadataFile{
		RepoURL: "https://github.com/example/repo",
		DrySHA:  "abc123",
	}

	err := writeReadme(dir, metadata)
	assert.NoError(t, err)

	readmePath := path.Join(dir, "README.md")
	readmeBytes, err := os.ReadFile(readmePath)
	assert.NoError(t, err)
	assert.Contains(t, string(readmeBytes), metadata.RepoURL)
}

func TestWriteManifests(t *testing.T) {
	dir := t.TempDir()

	manifests := []*apiclient.ManifestDetails{
		{Manifest: `{"kind":"Pod","apiVersion":"v1"}`},
	}

	err := writeManifests(dir, manifests)
	assert.NoError(t, err)

	manifestPath := path.Join(dir, "manifest.yaml")
	manifestBytes, err := os.ReadFile(manifestPath)
	assert.NoError(t, err)
	assert.Contains(t, string(manifestBytes), "kind: Pod")
}
