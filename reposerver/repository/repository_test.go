package repository

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/pkg/exec"
)

func TestGenerateYamlManifestInDir(t *testing.T) {
	// update this value if we add/remove manifests
	const countOfManifests = 21

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

func TestGenerateHelmChartWithDependencies(t *testing.T) {
	helmHome, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	os.Setenv("HELM_HOME", helmHome)
	_, err = exec.RunCommand("helm", "init", "--client-only", "--skip-refresh")
	assert.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(helmHome)
		_ = os.RemoveAll("../../util/helm/testdata/wordpress/charts")
		os.Unsetenv("HELM_HOME")
	}()
	q := ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := generateManifests("../../util/helm/testdata/wordpress", &q)
	assert.Nil(t, err)
	assert.Equal(t, 12, len(res1.Manifests))
}

func TestGenerateNullList(t *testing.T) {
	q := ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := generateManifests("./testdata/null-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = generateManifests("./testdata/empty-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res2, err := generateManifests("./testdata/weird-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res2.Manifests))
}
