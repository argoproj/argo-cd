package repository

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/reposerver/cache"
	"github.com/argoproj/argo-cd/reposerver/metrics"
	"github.com/argoproj/argo-cd/util"
	cacheutil "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	gitmocks "github.com/argoproj/argo-cd/util/git/mocks"
	"github.com/argoproj/argo-cd/util/helm"
	helmmocks "github.com/argoproj/argo-cd/util/helm/mocks"
)

func newServiceWithMocks(root string) (*Service, *gitmocks.Client) {
	service := NewService(metrics.NewMetricsServer(), cache.NewCache(
		cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Minute)),
		1*time.Minute,
	), 1)
	helmClient := &helmmocks.Client{}
	gitClient := &gitmocks.Client{}
	root, err := filepath.Abs(root)
	if err != nil {
		panic(err)
	}
	gitClient.On("Init").Return(nil)
	gitClient.On("Fetch").Return(nil)
	gitClient.On("Checkout", mock.Anything).Return(nil)
	gitClient.On("LsRemote", mock.Anything).Return(mock.Anything, nil)
	gitClient.On("CommitSHA").Return(mock.Anything, nil)
	gitClient.On("Root").Return(root)

	chart := "my-chart"
	version := semver.MustParse("1.1.0")
	helmClient.On("GetIndex").Return(&helm.Index{Entries: map[string]helm.Entries{
		chart: {{Version: "1.0.0"}, {Version: version.String()}},
	}}, nil)
	helmClient.On("ExtractChart", chart, version).Return("./testdata/my-chart", util.NopCloser, nil)
	helmClient.On("CleanChartCache", chart, version).Return(nil)

	service.newGitClient = func(rawRepoURL string, creds git.Creds, insecure bool, enableLfs bool) (client git.Client, e error) {
		return gitClient, nil
	}
	service.newHelmClient = func(repoURL string, creds helm.Creds) helm.Client {
		return helmClient
	}
	return service, gitClient
}

func newService(root string) *Service {
	service, _ := newServiceWithMocks(root)
	return service
}

func TestGenerateYamlManifestInDir(t *testing.T) {
	service := newService("../..")

	src := argoappv1.ApplicationSource{Path: "manifests/base"}
	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src}

	// update this value if we add/remove manifests
	const countOfManifests = 25

	res1, err := service.GenerateManifest(context.Background(), &q)

	assert.NoError(t, err)
	assert.Equal(t, countOfManifests, len(res1.Manifests))

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := GenerateManifests("./testdata/concatenated", "", &q)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(res2.Manifests))
}

// ensure we can use a semver constraint range (>= 1.0.0) and get back the correct chart (1.0.0)
func TestHelmManifestFromChartRepo(t *testing.T) {
	service := newService(".")
	source := &argoappv1.ApplicationSource{Chart: "my-chart", TargetRevision: ">= 1.0.0"}
	request := &apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: source, NoCache: true}
	response, err := service.GenerateManifest(context.Background(), request)
	assert.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, &apiclient.ManifestResponse{
		Manifests:  []string{"{\"apiVersion\":\"v1\",\"kind\":\"ConfigMap\",\"metadata\":{\"name\":\"my-map\"}}"},
		Namespace:  "",
		Server:     "",
		Revision:   "1.1.0",
		SourceType: "Helm",
	}, response)
}

func TestGenerateManifestsUseExactRevision(t *testing.T) {
	service, gitClient := newServiceWithMocks(".")

	src := argoappv1.ApplicationSource{Path: "./testdata/recurse", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src, Revision: "abc"}

	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
	assert.Equal(t, gitClient.Calls[0].Arguments[0], "abc")
}

func TestRecurseManifestsInDir(t *testing.T) {
	service := newService(".")

	src := argoappv1.ApplicationSource{Path: "./testdata/recurse", Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}

	q := apiclient.ManifestRequest{Repo: &argoappv1.Repository{}, ApplicationSource: &src}

	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateJsonnetManifestInDir(t *testing.T) {
	service := newService(".")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./testdata/jsonnet",
			Directory: &argoappv1.ApplicationSourceDirectory{
				Jsonnet: argoappv1.ApplicationSourceJsonnet{
					ExtVars: []argoappv1.JsonnetVar{{Name: "extVarString", Value: "extVarString"}, {Name: "extVarCode", Value: "\"extVarCode\"", Code: true}},
					TLAs:    []argoappv1.JsonnetVar{{Name: "tlaString", Value: "tlaString"}, {Name: "tlaCode", Value: "\"tlaCode\"", Code: true}},
				},
			},
		},
	}
	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGenerateKsonnetManifest(t *testing.T) {
	service := newService("../..")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./test/e2e/testdata/ksonnet",
			Ksonnet: &argoappv1.ApplicationSourceKsonnet{
				Environment: "dev",
			},
		},
	}
	res, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res.Manifests))
	assert.Equal(t, "dev", res.Namespace)
	assert.Equal(t, "https://kubernetes.default.svc", res.Server)
}

func TestGenerateHelmChartWithDependencies(t *testing.T) {
	service := newService("../..")

	q := apiclient.ManifestRequest{
		Repo: &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/wordpress",
		},
	}
	res1, err := service.GenerateManifest(context.Background(), &q)
	assert.Nil(t, err)
	assert.Len(t, res1.Manifests, 12)
}

func TestGenerateHelmWithValues(t *testing.T) {
	service := newService("../..")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-production.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})

	assert.NoError(t, err)

	replicasVerified := false
	for _, src := range res.Manifests {
		obj := unstructured.Unstructured{}
		err = json.Unmarshal([]byte(src), &obj)
		assert.NoError(t, err)

		if obj.GetKind() == "Deployment" && obj.GetName() == "test-redis-slave" {
			var dep v1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &dep)
			assert.NoError(t, err)
			assert.Equal(t, int32(2), *dep.Spec.Replicas)
			replicasVerified = true
		}
	}
	assert.True(t, replicasVerified)

}

// The requested value file (`../minio/values.yaml`) is outside the app path (`./util/helm/testdata/redis`), however
// since the requested value is sill under the repo directory (`~/go/src/github.com/argoproj/argo-cd`), it is allowed
func TestGenerateHelmWithValuesDirectoryTraversal(t *testing.T) {
	service := newService("../..")

	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"../minio/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.NoError(t, err)
}

func TestGenerateHelmWithURL(t *testing.T) {
	service := newService("../..")

	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"https://raw.githubusercontent.com/argoproj/argocd-example-apps/master/helm-guestbook/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.NoError(t, err)
}

// The requested value file (`../../../../../minio/values.yaml`) is outside the repo directory
// (`~/go/src/github.com/argoproj/argo-cd`), so it is blocked
func TestGenerateHelmWithValuesDirectoryTraversalOutsideRepo(t *testing.T) {
	service := newService("../..")

	_, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:          &argoappv1.Repository{},
		AppLabelValue: "test",
		ApplicationSource: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/redis",
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"../../../../../minio/values.yaml"},
				Values:     `cluster: {slaveCount: 2}`,
			},
		},
	})
	assert.Error(t, err, "should be on or under current directory")
}

func TestGenerateNullList(t *testing.T) {
	service := newService(".")

	res1, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{Path: "./testdata/null-list"},
	})
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{Path: "./testdata/empty-list"},
	})
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{Path: "./testdata/weird-list"},
	})
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
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
	service := newService(".")

	res, err := service.GenerateManifest(context.Background(), &apiclient.ManifestRequest{
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
		Repo:              &argoappv1.Repository{},
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("./testdata/utf-16", "", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestListApps(t *testing.T) {
	service := newService("./testdata")

	res, err := service.ListApps(context.Background(), &apiclient.ListAppsRequest{Repo: &argoappv1.Repository{}})
	assert.NoError(t, err)

	expectedApps := map[string]string{
		"Kustomization":      "Kustomize",
		"invalid-helm":       "Helm",
		"invalid-kustomize":  "Kustomize",
		"kustomization_yaml": "Kustomize",
		"kustomization_yml":  "Kustomize",
		"my-chart":           "Helm",
	}
	assert.Equal(t, expectedApps, res.Apps)
}

func TestGetAppDetailsHelm(t *testing.T) {
	service := newService("../..")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./util/helm/testdata/wordpress",
		},
	})

	assert.NoError(t, err)
	assert.NotNil(t, res.Helm)

	assert.Equal(t, "Helm", res.Type)
	assert.EqualValues(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
}

func TestGetAppDetailsKustomize(t *testing.T) {
	service := newService("../..")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./util/kustomize/testdata/kustomization_yaml",
		},
	})

	assert.NoError(t, err)

	assert.Equal(t, "Kustomize", res.Type)
	assert.NotNil(t, res.Kustomize)
	assert.EqualValues(t, []string{"nginx:1.15.4", "k8s.gcr.io/nginx-slim:0.8"}, res.Kustomize.Images)
}

func TestGetAppDetailsKsonnet(t *testing.T) {
	service := newService("../..")

	res, err := service.GetAppDetails(context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{},
		Source: &argoappv1.ApplicationSource{
			Path: "./test/e2e/testdata/ksonnet",
		},
	})

	assert.NoError(t, err)

	assert.Equal(t, "Ksonnet", res.Type)
	assert.NotNil(t, res.Ksonnet)
	assert.Equal(t, "guestbook", res.Ksonnet.Name)
	assert.Len(t, res.Ksonnet.Environments, 3)
}

func TestGetHelmCharts(t *testing.T) {
	service := newService("../..")
	res, err := service.GetHelmCharts(context.Background(), &apiclient.HelmChartsRequest{Repo: &argoappv1.Repository{}})
	assert.NoError(t, err)
	assert.Len(t, res.Items, 1)

	item := res.Items[0]
	assert.Equal(t, "my-chart", item.Name)
	assert.EqualValues(t, []string{"1.0.0", "1.1.0"}, item.Versions)
}

func TestGetRevisionMetadata(t *testing.T) {
	service, gitClient := newServiceWithMocks("../..")
	now := time.Now()

	gitClient.On("RevisionMetadata", mock.Anything).Return(&git.RevisionMetadata{
		Message: strings.Repeat("a", 100) + "\n" + "second line",
		Author:  "author",
		Date:    now,
		Tags:    []string{"tag1", "tag2"},
	}, nil)

	res, err := service.GetRevisionMetadata(context.Background(), &apiclient.RepoServerRevisionMetadataRequest{
		Repo:     &argoappv1.Repository{},
		Revision: "c0b400fc458875d925171398f9ba9eabd5529923",
	})

	assert.NoError(t, err)
	assert.Equal(t, strings.Repeat("a", 61)+"...", res.Message)
	assert.Equal(t, now, res.Date.Time)
	assert.Equal(t, "author", res.Author)
	assert.EqualValues(t, []string{"tag1", "tag2"}, res.Tags)

}

func Test_newEnv(t *testing.T) {
	assert.Equal(t, &argoappv1.Env{
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_NAME", Value: "my-app-name"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_NAMESPACE", Value: "my-namespace"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_REVISION", Value: "my-revision"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_REPO_URL", Value: "https://github.com/my-org/my-repo"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_PATH", Value: "my-path"},
		&argoappv1.EnvEntry{Name: "ARGOCD_APP_SOURCE_TARGET_REVISION", Value: "my-target-revision"},
	}, newEnv(&apiclient.ManifestRequest{
		AppLabelValue: "my-app-name",
		Namespace:     "my-namespace",
		Repo:          &argoappv1.Repository{Repo: "https://github.com/my-org/my-repo"},
		ApplicationSource: &argoappv1.ApplicationSource{
			Path:           "my-path",
			TargetRevision: "my-target-revision",
		},
	}, "my-revision"))
}

func TestService_newHelmClientResolveRevision(t *testing.T) {
	service := newService(".")

	t.Run("EmptyRevision", func(t *testing.T) {
		_, _, err := service.newHelmClientResolveRevision(&argoappv1.Repository{}, "", "")
		assert.EqualError(t, err, "invalid revision '': improper constraint: ")
	})
	t.Run("InvalidRevision", func(t *testing.T) {
		_, _, err := service.newHelmClientResolveRevision(&argoappv1.Repository{}, "???", "")
		assert.EqualError(t, err, "invalid revision '???': improper constraint: ???")
	})
}
