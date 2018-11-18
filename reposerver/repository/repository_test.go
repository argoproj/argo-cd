package repository

import (
	"testing"

	"github.com/stretchr/testify/assert"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestGenerateYamlManifestInDir(t *testing.T) {
	// update this value if we add/remove manifests
	const countOfManifests = 23

	q := ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := generateManifests("../../manifests/base", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), countOfManifests)

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := generateManifests("./testdata/concatenated", &q)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res2.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	q := ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := generateManifests("./testdata/jsonnet", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 2)
}
