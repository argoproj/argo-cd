package repository

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
)

func newMockRepoServerService() *Service {
	return &Service{
		repoLock: util.NewKeyLock(),
		cache:    cache.NewCache(cache.NewInMemoryCache(time.Hour)),
	}
}

func repoUrl() string {
	wd, _ := os.Getwd()

	return "file://" + filepath.Join(wd, "../..")
}

func TestGenerateYamlManifestInDir(t *testing.T) {
	// update this value if we add/remove manifests
	const countOfManifests = 23

	q := ManifestRequest{
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
	q := ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	q.ApplicationSource.Directory = &argoappv1.ApplicationSourceDirectory{Recurse: true}
	res1, err := GenerateManifests("./testdata/recurse", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	q := ManifestRequest{
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
	res1, err := GenerateManifests("../../util/helm/testdata/wordpress", &q)
	assert.Nil(t, err)
	assert.Equal(t, 11, len(res1.Manifests))
}

func TestGenerateNullList(t *testing.T) {
	q := ManifestRequest{
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
	res, err := GenerateManifests(".", &ManifestRequest{
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
				Args:    []string{`echo "{\"kind\": \"FakeObject\", \"metadata\": { \"name\": \"$ARGOCD_APP_NAME\", \"namespace\": \"$ARGOCD_APP_NAMESPACE\"}}"`},
			},
		}},
	})

	assert.Nil(t, err)
	assert.Equal(t, 1, len(res.Manifests))

	obj := &unstructured.Unstructured{}
	assert.Nil(t, json.Unmarshal([]byte(res.Manifests[0]), obj))

	assert.Equal(t, obj.GetName(), "test-app")
	assert.Equal(t, obj.GetNamespace(), "test-namespace")
}

func TestGenerateFromUTF16(t *testing.T) {
	q := ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("./testdata/utf-16", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGetAppDetailsHelm(t *testing.T) {
	serve := newMockRepoServerService()
	ctx := context.Background()

	// verify default parameters are returned when not supplying values
	{
		res, err := serve.GetAppDetails(ctx, &RepoServerAppDetailsQuery{
			Repo: &argoappv1.Repository{Repo: repoUrl()},
			Path: "util/helm/testdata/redis",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
		assert.Equal(t, argoappv1.HelmParameter{Name: "image.pullPolicy", Value: "Always"}, getHelmParameter("image.pullPolicy", res.Helm.Parameters))
		assert.Equal(t, 49, len(res.Helm.Parameters))
	}

	// verify values specific parameters are returned when a values is specified
	{
		res, err := serve.GetAppDetails(ctx, &RepoServerAppDetailsQuery{
			Repo: &argoappv1.Repository{Repo: repoUrl()},
			Path: "util/helm/testdata/redis",
			Helm: &HelmAppDetailsQuery{
				ValueFiles: []string{"values-production.yaml"},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
		assert.Equal(t, argoappv1.HelmParameter{Name: "image.pullPolicy", Value: "IfNotPresent"}, getHelmParameter("image.pullPolicy", res.Helm.Parameters))
		assert.Equal(t, 49, len(res.Helm.Parameters))
	}
}

func getHelmParameter(name string, params []*argoappv1.HelmParameter) argoappv1.HelmParameter {
	for _, p := range params {
		if name == p.Name {
			return *p
		}
	}
	panic(name + " not in params")
}

func TestGetAppDetailsKsonnet(t *testing.T) {
	serve := newMockRepoServerService()
	ctx := context.Background()

	res, err := serve.GetAppDetails(ctx, &RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{Repo: repoUrl()},
		Path: "util/ksonnet/testdata/test-app",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://1.2.3.4", res.Ksonnet.Environments["test-env"].Destination.Server)
	assert.Equal(t, "test-namespace", res.Ksonnet.Environments["test-env"].Destination.Namespace)
	assert.Equal(t, "v1.8.0", res.Ksonnet.Environments["test-env"].K8SVersion)
	assert.Equal(t, "test-env", res.Ksonnet.Environments["test-env"].Path)
	assert.Equal(t, argoappv1.KsonnetParameter{Component: "demo", Name: "containerPort", Value: "80"}, *res.Ksonnet.Parameters[0])
	assert.Equal(t, 6, len(res.Ksonnet.Parameters))
}

func TestGetAppDetailsKustomize(t *testing.T) {
	serve := newMockRepoServerService()
	ctx := context.Background()

	res, err := serve.GetAppDetails(ctx, &RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{Repo: repoUrl()},
		Path: "util/kustomize/testdata/kustomization_yaml",
	})
	assert.NoError(t, err)
	assert.Nil(t, res.Kustomize.Images)
	assert.Equal(t, []*argoappv1.KustomizeImageTag{{"nginx", "1.15.4"}, {"k8s.gcr.io/nginx-slim", "0.8"}}, res.Kustomize.ImageTags)
}
