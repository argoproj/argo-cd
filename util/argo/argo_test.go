package argo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	testcore "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	mockrepo "github.com/argoproj/argo-cd/reposerver/mocks"
	"github.com/argoproj/argo-cd/reposerver/repository"
	mockreposerver "github.com/argoproj/argo-cd/reposerver/repository/mocks"
	mockdb "github.com/argoproj/argo-cd/util/db/mocks"
)

func TestRefreshApp(t *testing.T) {
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset := appclientset.NewSimpleClientset(&testApp)
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	_, err := RefreshApp(appIf, "test-app", argoappv1.RefreshTypeNormal)
	assert.Nil(t, err)
	// For some reason, the fake Application inferface doesn't reflect the patch status after Patch(),
	// so can't verify it was set in unit tests.
	//_, ok := newApp.Annotations[common.AnnotationKeyRefresh]
	//assert.True(t, ok)
}

type fakeCloser struct {
}

func (f fakeCloser) Close() error {
	return nil
}

func TestGetSpecErrors(t *testing.T) {
	// nice values
	knownGitRepoUrl := "https://github.com/argoproj/argo-cd"
	knownHelmRepoUrl := "https://kubernetes-charts.storage.googleapis.com"
	notFoundRepoUrl := "http://0.0.0.0"
	path := "xxx"
	targetRevision := ""
	server := "http://1.1.1.1"
	namespace := "default"

	var spec argoappv1.ApplicationSpec
	before := func() {
		spec = argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{RepoURL: knownGitRepoUrl, Path: path, TargetRevision: targetRevision}, Destination: argoappv1.ApplicationDestination{Namespace: namespace, Server: server}}
	}
	project := argoappv1.AppProject{Spec: argoappv1.AppProjectSpec{SourceRepos: []string{knownGitRepoUrl, knownHelmRepoUrl}, Destinations: []argoappv1.ApplicationDestination{{Server: server, Namespace: namespace}}}}

	mockRepoServiceClient := mockreposerver.RepoServerServiceClient{}
	mockRepoServiceClient.On("FindApps", mock.Anything, mock.Anything).Return(&repository.ListAppsResponse{}, nil)
	mockRepoServiceClient.On("GetApp", mock.Anything, mock.Anything).Return(&repository.GetAppResponse{}, nil)
	mockRepoServiceClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(&repository.ManifestResponse{}, nil)

	mockRepoClient := mockrepo.Clientset{}
	mockRepoClient.On("NewRepoServerClient").Return(&fakeCloser{}, &mockRepoServiceClient, nil)

	mockDb := mockdb.ArgoDB{}
	mockDb.On("GetRepository", mock.Anything, knownGitRepoUrl).Return(&argoappv1.Repository{}, nil)
	mockDb.On("GetRepository", mock.Anything, knownHelmRepoUrl).Return(&argoappv1.Repository{Type: "helm"}, nil)
	mockDb.On("GetRepository", mock.Anything, notFoundRepoUrl).Return(nil, status.Error(codes.NotFound, ""))
	mockDb.On("GetCluster", mock.Anything, server).Return(&argoappv1.Cluster{}, nil)

	t.Run("MissingRepoUrl", func(t *testing.T) {
		before()
		spec.Source.RepoURL = ""

		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Equal(t, []argoappv1.ApplicationCondition{{Type: "InvalidSpecError", Message: "spec.source.repoURL and spec.source.path are required"}}, conditions)
	})

	t.Run("MissingPath", func(t *testing.T) {
		before()
		spec.Source.Path = ""

		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Equal(t, []argoappv1.ApplicationCondition{{Type: "InvalidSpecError", Message: "spec.source.repoURL and spec.source.path are required"}}, conditions)
	})

	t.Run("MissingDestinationServer", func(t *testing.T) {
		before()
		spec.Destination.Server = ""

		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Equal(t, []argoappv1.ApplicationCondition{{Type: "InvalidSpecError", Message: "Destination server and/or namespace missing from app spec"}}, conditions)
	})

	t.Run("MissingDestinationServer", func(t *testing.T) {
		before()
		spec.Destination.Server = ""

		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Equal(t, []argoappv1.ApplicationCondition{{Type: "InvalidSpecError", Message: "Destination server and/or namespace missing from app spec"}}, conditions)
	})
	t.Run("UnknownRepo", func(t *testing.T) {
		before()
		spec.Source.RepoURL = notFoundRepoUrl

		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Contains(t, conditions[0].Message, "No credentials available for source repository and repository is not publicly accessible")
	})
	t.Run("Git", func(t *testing.T) {
		before()
		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Equal(t, []argoappv1.ApplicationCondition{}, conditions)
	})
	t.Run("Helm", func(t *testing.T) {
		before()
		spec.Source.RepoURL = knownHelmRepoUrl

		conditions, _, err := GetSpecErrors(context.TODO(), &spec, &project, &mockRepoClient, &mockDb)

		assert.NoError(t, err)
		assert.Equal(t, []argoappv1.ApplicationCondition{}, conditions)
	})
}

func TestGetAppProjectWithNoProjDefined(t *testing.T) {
	projName := "default"
	namespace := "default"

	testProj := &argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projName, Namespace: namespace},
	}

	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = namespace
	appClientset := appclientset.NewSimpleClientset(testProj)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	informer := v1alpha1.NewAppProjectInformer(appClientset, namespace, 0, cache.Indexers{})
	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	proj, err := GetAppProject(&testApp.Spec, applisters.NewAppProjectLister(informer.GetIndexer()), namespace)
	assert.Nil(t, err)
	assert.Equal(t, proj.Name, projName)
}

func TestWaitForRefresh(t *testing.T) {
	appClientset := appclientset.NewSimpleClientset()

	// Verify timeout
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	oneHundredMs := 100 * time.Millisecond
	app, err := WaitForRefresh(context.Background(), appIf, "test-app", &oneHundredMs)
	assert.NotNil(t, err)
	assert.Nil(t, app)
	assert.Contains(t, strings.ToLower(err.Error()), "deadline exceeded")

	// Verify success
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset = appclientset.NewSimpleClientset()

	appIf = appClientset.ArgoprojV1alpha1().Applications("default")
	watcher := watch.NewFake()
	appClientset.PrependWatchReactor("applications", testcore.DefaultWatchReactor(watcher, nil))
	// simulate add/update/delete watch events
	go watcher.Add(&testApp)
	app, err = WaitForRefresh(context.Background(), appIf, "test-app", &oneHundredMs)
	assert.Nil(t, err)
	assert.NotNil(t, app)
}

func TestContainsSyncResource(t *testing.T) {
	var (
		blankUnstructured unstructured.Unstructured
		blankResource     argoappv1.SyncOperationResource
		helloResource     = argoappv1.SyncOperationResource{Name: "hello"}
	)
	tables := []struct {
		u        *unstructured.Unstructured
		rr       []argoappv1.SyncOperationResource
		expected bool
	}{
		{&blankUnstructured, []argoappv1.SyncOperationResource{}, false},
		{&blankUnstructured, []argoappv1.SyncOperationResource{blankResource}, true},
		{&blankUnstructured, []argoappv1.SyncOperationResource{helloResource}, false},
	}

	for _, table := range tables {
		if out := ContainsSyncResource(table.u.GetName(), table.u.GroupVersionKind(), table.rr); out != table.expected {
			t.Errorf("Expected %t for slice %+v conains resource %+v; instead got %t", table.expected, table.rr, table.u, out)
		}
	}
}

func TestNormalizeApplicationSpec(t *testing.T) {
	{
		// Verify we normalize project name
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{}, argoappv1.ApplicationSourceTypeKustomize)
		assert.Equal(t, spec.Project, "default")
	}

	{
		// Verify we copy over legacy componentParameterOverride to ksonnet field
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				Ksonnet: &argoappv1.ApplicationSourceKsonnet{
					Environment: "foo",
				},
				ComponentParameterOverrides: []argoappv1.ComponentParameter{{Component: "foo", Name: "bar", Value: "baz"}},
			},
		}, argoappv1.ApplicationSourceTypeKsonnet)
		assert.Equal(t, spec.Source.Ksonnet.Parameters[0].Component, "foo")
		assert.Equal(t, spec.Source.Ksonnet.Parameters[0].Name, "bar")
		assert.Equal(t, spec.Source.Ksonnet.Parameters[0].Value, "baz")
		_, err := spec.Source.ExplicitType()
		assert.NoError(t, err)
	}

	{
		// Verify we sync ksonnet.parameters field to legacy field
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				Ksonnet: &argoappv1.ApplicationSourceKsonnet{
					Environment: "foo",
					Parameters:  []argoappv1.KsonnetParameter{{Component: "foo", Name: "bar", Value: "baz"}},
				},
			},
		}, argoappv1.ApplicationSourceTypeKsonnet)
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Component, "foo")
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Name, "bar")
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Value, "baz")
		_, err := spec.Source.ExplicitType()
		assert.NoError(t, err)
	}

	{
		// Verify we copy over legacy componentParameterOverride to helm.parameters
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				ComponentParameterOverrides: []argoappv1.ComponentParameter{{Name: "bar", Value: "baz"}},
			},
		}, argoappv1.ApplicationSourceTypeHelm)
		assert.Equal(t, spec.Source.Helm.Parameters[0].Name, "bar")
		assert.Equal(t, spec.Source.Helm.Parameters[0].Value, "baz")
		_, err := spec.Source.ExplicitType()
		assert.NoError(t, err)
	}

	{
		// Verify we sync helm.parameters field to legacy componentParameterOverride field
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				Helm: &argoappv1.ApplicationSourceHelm{
					Parameters: []argoappv1.HelmParameter{{Name: "bar", Value: "baz"}},
				},
			},
		}, argoappv1.ApplicationSourceTypeHelm)
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Component, "")
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Name, "bar")
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Value, "baz")
		_, err := spec.Source.ExplicitType()
		assert.NoError(t, err)
	}

	{
		// Verify we copy over legacy componentParameterOverride to kustomize.imageTags
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				ComponentParameterOverrides: []argoappv1.ComponentParameter{{Component: "imagetag", Name: "bar", Value: "baz"}},
			},
		}, argoappv1.ApplicationSourceTypeKustomize)
		assert.Equal(t, spec.Source.Kustomize.ImageTags[0].Name, "bar")
		assert.Equal(t, spec.Source.Kustomize.ImageTags[0].Value, "baz")
		_, err := spec.Source.ExplicitType()
		assert.NoError(t, err)
	}

	{
		// Verify we sync kustomize.imageTags field to legacy componentParameterOverride field
		spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				Kustomize: &argoappv1.ApplicationSourceKustomize{
					ImageTags: []argoappv1.KustomizeImageTag{{Name: "bar", Value: "baz"}},
				},
			},
		}, argoappv1.ApplicationSourceTypeHelm)
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Component, "imagetag")
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Name, "bar")
		assert.Equal(t, spec.Source.ComponentParameterOverrides[0].Value, "baz")
		_, err := spec.Source.ExplicitType()
		assert.NoError(t, err)
	}
}

// TestNilOutZerValueAppSources verifies we will nil out app source specs when they are their zero-value
func TestNilOutZerValueAppSources(t *testing.T) {
	var spec *argoappv1.ApplicationSpec
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NamePrefix: "foo"}}}, argoappv1.ApplicationSourceTypeKustomize)
		assert.NotNil(t, spec.Source.Kustomize)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NamePrefix: ""}}}, argoappv1.ApplicationSourceTypeKustomize)
		assert.Nil(t, spec.Source.Kustomize)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{"values.yaml"}}}}, argoappv1.ApplicationSourceTypeHelm)
		assert.NotNil(t, spec.Source.Helm)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{}}}}, argoappv1.ApplicationSourceTypeHelm)
		assert.Nil(t, spec.Source.Helm)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Ksonnet: &argoappv1.ApplicationSourceKsonnet{Environment: "foo"}}}, argoappv1.ApplicationSourceTypeKsonnet)
		assert.NotNil(t, spec.Source.Ksonnet)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Ksonnet: &argoappv1.ApplicationSourceKsonnet{Environment: ""}}}, argoappv1.ApplicationSourceTypeKsonnet)
		assert.Nil(t, spec.Source.Ksonnet)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}}, argoappv1.ApplicationSourceTypeDirectory)
		assert.NotNil(t, spec.Source.Directory)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Directory: &argoappv1.ApplicationSourceDirectory{Recurse: false}}}, argoappv1.ApplicationSourceTypeDirectory)
		assert.Nil(t, spec.Source.Directory)
	}
}
