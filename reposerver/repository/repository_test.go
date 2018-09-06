package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateYamlManifestInDir(t *testing.T) {
	q := ManifestRequest{}
	res1, err := generateManifests("../../manifests/components", &q)
	assert.Nil(t, err)
	assert.True(t, len(res1.Manifests) == 16) // update this value if we add/remove manifests

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := generateManifests("../../manifests", &q)
	assert.Nil(t, err)
	assert.True(t, len(res2.Manifests) == len(res1.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	q := ManifestRequest{}
	res1, err := generateManifests("./testdata/jsonnet", &q)
	assert.Nil(t, err)
	assert.True(t, len(res1.Manifests) == 2)
}
