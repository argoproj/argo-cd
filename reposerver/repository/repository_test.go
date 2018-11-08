package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateYamlManifestInDir(t *testing.T) {
	// update this value if we add/remove manifests
	const countOfManifests = 23

	q := ManifestRequest{}
	res1, err := generateManifests("../../manifests/base", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), countOfManifests)

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := generateManifests("./testdata/concatenated", &q)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res2.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	q := ManifestRequest{}
	res1, err := generateManifests("./testdata/jsonnet", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 2)
}
