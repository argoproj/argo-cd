package repository

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/pkg/exec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/git"
	gitmocks "github.com/argoproj/argo-cd/util/git/mocks"
)

func newMockRepoServerService(root string) *Service {
	return &Service{
		repoLock:   util.NewKeyLock(),
		gitFactory: newFakeGitClientFactory(root),
		cache:      cache.NewCache(cache.NewInMemoryCache(time.Hour)),
	}
}

func newFakeGitClientFactory(root string) *fakeGitClientFactory {
	return &fakeGitClientFactory{
		root:             root,
		revision:         "aaaaaaaaaabbbbbbbbbbccccccccccdddddddddd",
		revisionMetadata: &git.RevisionMetadata{Author: "foo", Message: strings.Repeat("x", 99), Tags: []string{"bar"}},
	}
}

type fakeGitClientFactory struct {
	root             string
	revision         string
	revisionMetadata *git.RevisionMetadata
}

func (f *fakeGitClientFactory) NewClient(repoURL string, path string, creds git.Creds, insecureIgnoreHostKey bool, enableLfs bool) (git.Client, error) {
	mockClient := gitmocks.Client{}
	root := "./testdata"
	if f.root != "" {
		root = f.root
	}
	mockClient.On("Root").Return(root)
	mockClient.On("Init").Return(nil)
	mockClient.On("Fetch", mock.Anything).Return(nil)
	mockClient.On("Checkout", mock.Anything).Return(nil)
	mockClient.On("LsRemote", mock.Anything).Return(f.revision, nil)
	mockClient.On("LsFiles", mock.Anything).Return([]string{}, nil)
	mockClient.On("CommitSHA", mock.Anything).Return(f.revision, nil)
	mockClient.On("RevisionMetadata", f.revision).Return(f.revisionMetadata, nil)
	return &mockClient, nil
}

func TestGenerateYamlManifestInDir(t *testing.T) {
	// update this value if we add/remove manifests
	const countOfManifests = 25

	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("../../manifests", "base", &q)
	assert.Nil(t, err)
	assert.Equal(t, countOfManifests, len(res1.Manifests))

	// this will test concatenated manifests to verify we split YAMLs correctly
	res2, err := GenerateManifests("./testdata", "concatenated", &q)
	assert.Nil(t, err)
	assert.Equal(t, 3, len(res2.Manifests))
}

func TestRecurseManifestsInDir(t *testing.T) {
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	q.ApplicationSource.Directory = &argoappv1.ApplicationSourceDirectory{Recurse: true}
	res1, err := GenerateManifests("./testdata", "recurse", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestPathRoot(t *testing.T) {
	_, err := appPath("./testdata", "/")
	assert.EqualError(t, err, "/: app path is absolute")
}

func TestPathAbsolute(t *testing.T) {
	_, err := appPath("./testdata", "/etc/passwd")
	assert.EqualError(t, err, "/etc/passwd: app path is absolute")
}

func TestPathDotDot(t *testing.T) {
	_, err := appPath("./testdata", "..")
	assert.EqualError(t, err, "..: app path outside repo")
}

func TestPathDotDotSlash(t *testing.T) {
	_, err := appPath("./testdata", "../")
	assert.EqualError(t, err, "../: app path outside repo")
}

func TestPathDot(t *testing.T) {
	_, err := appPath("./testdata", ".")
	assert.NoError(t, err)
}

func TestPathDotSlash(t *testing.T) {
	_, err := appPath("./testdata", "./")
	assert.NoError(t, err)
}

func TestNonExistentPath(t *testing.T) {
	_, err := appPath("./testdata", "does-not-exist")
	assert.EqualError(t, err, "does-not-exist: app path does not exist")
}

func TestPathNotDir(t *testing.T) {
	_, err := appPath("./testdata", "file.txt")
	assert.EqualError(t, err, "file.txt: app path is not a directory")
}

func TestGenerateManifests_NonExistentPath(t *testing.T) {
	_, err := GenerateManifests("./testdata", "does-not-exist", nil)
	assert.EqualError(t, err, "does-not-exist: app path does not exist")
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
	res1, err := GenerateManifests("./testdata", "jsonnet", &q)
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
	res1, err := GenerateManifests("../../util/helm/testdata", "wordpress", &q)
	assert.Nil(t, err)
	assert.Equal(t, 12, len(res1.Manifests))
}

func TestGenerateNullList(t *testing.T) {
	q := apiclient.ManifestRequest{
		ApplicationSource: &argoappv1.ApplicationSource{},
	}
	res1, err := GenerateManifests("./testdata", "null-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res1, err = GenerateManifests("./testdata", "empty-list", &q)
	assert.Nil(t, err)
	assert.Equal(t, len(res1.Manifests), 1)
	assert.Contains(t, res1.Manifests[0], "prometheus-operator-operator")

	res2, err := GenerateManifests("./testdata", "weird-list", &q)
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
	res, err := GenerateManifests(".", ".", &apiclient.ManifestRequest{
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
	res1, err := GenerateManifests("./testdata", "utf-16", &q)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(res1.Manifests))
}

func TestGetAppDetailsHelm(t *testing.T) {
	serve := newMockRepoServerService("../../util/helm/testdata")
	ctx := context.Background()

	// verify default parameters are returned when not supplying values
	t.Run("DefaultParameters", func(t *testing.T) {
		res, err := serve.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
			Repo: &argoappv1.Repository{Repo: "https://github.com/fakeorg/fakerepo.git"},
			Path: "redis",
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
		assert.Contains(t, res.Helm.Values, "registry: docker.io")
		assert.Equal(t, argoappv1.HelmParameter{Name: "image.pullPolicy", Value: "Always"}, getHelmParameter("image.pullPolicy", res.Helm.Parameters))
		assert.Equal(t, 49, len(res.Helm.Parameters))
	})

	// verify values specific parameters are returned when a values is specified
	t.Run("SpecificParameters", func(t *testing.T) {
		res, err := serve.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
			Repo: &argoappv1.Repository{Repo: "https://github.com/fakeorg/fakerepo.git"},
			Path: "redis",
			Helm: &apiclient.HelmAppDetailsQuery{
				ValueFiles: []string{"values-production.yaml"},
			},
		})
		assert.NoError(t, err)
		assert.Equal(t, []string{"values-production.yaml", "values.yaml"}, res.Helm.ValueFiles)
		assert.Contains(t, res.Helm.Values, "registry: docker.io")
		assert.Equal(t, argoappv1.HelmParameter{Name: "image.pullPolicy", Value: "IfNotPresent"}, getHelmParameter("image.pullPolicy", res.Helm.Parameters))
		assert.Equal(t, 49, len(res.Helm.Parameters))
	})
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
	serve := newMockRepoServerService("../../test/e2e/testdata")
	ctx := context.Background()

	res, err := serve.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{Repo: "https://github.com/fakeorg/fakerepo.git"},
		Path: "ksonnet",
	})
	assert.NoError(t, err)
	assert.Equal(t, "https://kubernetes.default.svc", res.Ksonnet.Environments["prod"].Destination.Server)
	assert.Equal(t, "prod", res.Ksonnet.Environments["prod"].Destination.Namespace)
	assert.Equal(t, "v1.10.0", res.Ksonnet.Environments["prod"].K8SVersion)
	assert.Equal(t, "prod", res.Ksonnet.Environments["prod"].Path)
	assert.Equal(t, argoappv1.KsonnetParameter{Component: "guestbook-ui", Name: "command", Value: "null"}, *res.Ksonnet.Parameters[0])
	assert.Equal(t, 7, len(res.Ksonnet.Parameters))
}

func TestGetAppDetailsKustomize(t *testing.T) {
	serve := newMockRepoServerService("../../util/kustomize/testdata")
	ctx := context.Background()

	res, err := serve.GetAppDetails(ctx, &apiclient.RepoServerAppDetailsQuery{
		Repo: &argoappv1.Repository{Repo: "https://github.com/fakeorg/fakerepo.git"},
		Path: "kustomization_yaml",
	})
	assert.NoError(t, err)
	assert.Equal(t, []string{"nginx:1.15.4", "k8s.gcr.io/nginx-slim:0.8"}, res.Kustomize.Images)
}

func TestService_GetRevisionMetadata(t *testing.T) {
	factory := newFakeGitClientFactory(".")
	service := &Service{
		repoLock:   util.NewKeyLock(),
		gitFactory: factory,
		cache:      cache.NewCache(cache.NewInMemoryCache(1 * time.Hour)),
	}
	type args struct {
		q *apiclient.RepoServerRevisionMetadataRequest
	}
	q := &apiclient.RepoServerRevisionMetadataRequest{Repo: &argoappv1.Repository{}, Revision: factory.revision}
	metadata := &v1alpha1.RevisionMetadata{
		Author:  factory.revisionMetadata.Author,
		Message: strings.Repeat("x", 61) + "...",
		Tags:    factory.revisionMetadata.Tags,
	}
	tests := []struct {
		name    string
		args    args
		want    *v1alpha1.RevisionMetadata
		wantErr bool
	}{
		{"CacheMiss", args{q: q}, metadata, false},
		{"CacheHit", args{q: q}, metadata, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := service.GetRevisionMetadata(context.Background(), tt.args.q)
			if (err != nil) != tt.wantErr {
				t.Errorf("Service.GetRevisionMetadata() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Service.GetRevisionMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}
