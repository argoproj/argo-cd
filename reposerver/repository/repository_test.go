package repository

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
)

func TestGenerateYamlManifestInDir(t *testing.T) {
	// update this value if we add/remove manifests
	const countOfManifests = 25

	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("../../manifests/base", &q)
	assert.Nil(t, err)
	assert.Equal(t, countOfManifests, len(res1.Manifests))

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := GenerateManifests("./testdata/concatenated", &q)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res2.Manifests))
}

func TestRecurseManifestsInDir(t *testing.T) {
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	q.ApplicationSource.Directory = &argoappv1.ApplicationSourceDirectory{Recurse: true}
	res1, err := GenerateManifests("./testdata/recurse", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{
			Directory: &argoappv1.ApplicationSourceDirectory{
				Jsonnet: argoappv1.ApplicationSourceJsonnet{
					ExtVars: []argoappv1.JsonnetVar{{Name: "extVarString", Value: "extVarString"}, {Name: "extVarCode", Value: "\"extVarCode\"", Code: true}},
					TLAs:    []argoappv1.JsonnetVar{{Name: "tlaString", Value: "tlaString"}, {Name: "tlaCode", Value: "\"tlaCode\"", Code: true}},
				},
			},
		},
	}
	res1, err := GenerateManifests("./testdata/jsonnet", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateHelmChartWithDependencies(t *testing.T) {
	helmHome, err := ioutil.TempDir("", "")
	assert.NoError(t, err)
	os.Setenv("HELM_HOME", helmHome)
	_, err = exec.RunCommand("helm", exec.CmdOpts{}, "init", "--client-only", "--skip-refresh")
	assert.NoError(t, err)
	defer func() {
		_ = os.RemoveAll(helmHome)
		_ = os.RemoveAll("../../util/helm/testdata/wordpress/charts")
		os.Unsetenv("HELM_HOME")
	}()
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("../../util/helm/testdata/wordpress", &q)
	assert.Nil(t, err)
	assert.Len(t, res1.Manifests, 12)
}

func TestGenerateNullList(t *testing.T) {
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("./testdata/null-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = GenerateManifests("./testdata/empty-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res2, err := GenerateManifests("./testdata/weird-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res2.Manifests))
}

func TestIdentifyAppSourceTypeByAppDirWithKustomizations(t *testing.T) {
	sourceType, err := GetAppSourceType(&argoappv1.ApplicationSource{}, "./testdata/kustomization_yaml")
	assert.Nil(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)

	sourceType, err = GetAppSourceType(&argoappv1.ApplicationSource{}, "./testdata/kustomization_yml")
	assert.Nil(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)

	sourceType, err = GetAppSourceType(&argoappv1.ApplicationSource{}, "./testdata/Kustomization")
	assert.Nil(t, err)
	assert.Equal(t, argoappv1.ApplicationSourceTypeKustomize, sourceType)
}

func TestRunCustomTool(t *testing.T) {
	res, err := GenerateManifests(".", &apiclient.ManifestRequest{
		AppLabelValue: "test-app",
		Namespace:     "test-namespace",
		ApplicationSource: &argoappv1.ApplicationSource{
			Plugin: &argoappv1.ApplicationSourcePlugin{
				Name: "test",
			},
		},
		Plugins: []*argoappv1.ConfigManagementPlugin{{
			Name: "test",
			Generate: argoappv1.Command{
				Command: []string{"sh", "-c"},
				Args:    []string{`echo "{\"kind\": \"FakeObject\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\", \"annotations\": {\"GIT_ASKPASS\": \"$GIT_ASKPASS\", \"GIT_USERNAME\": \"$GIT_USERNAME\", \"GIT_PASSWORD\": \"$GIT_PASSWORD\"}}}"`},
			},
		}},
		Repo: &argoappv1.Repository{
			Username: "foo", Password: "bar",
		},
	})

	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.Manifests))

	obj := &unstructured.Unstructured{}
	assert.Nil(t, json.Unmarshal([]byte(res.Manifests[0]), obj))

	assert.Equal(t, obj.GetName(), "test-app")
	assert.Equal(t, obj.GetNamespace(), "test-namespace")
	assert.Equal(t, "git-ask-pass.sh", obj.GetAnnotations()["GIT_ASKPASS"])
	assert.Equal(t, "foo", obj.GetAnnotations()["GIT_USERNAME"])
	assert.Equal(t, "bar", obj.GetAnnotations()["GIT_PASSWORD"])
}

func TestGenerateFromUTF16(t *testing.T) {
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("./testdata/utf-16", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}
