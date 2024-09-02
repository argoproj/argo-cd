package argo

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/gitops-engine/pkg/sync/common"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/db"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

func TestRefreshApp(t *testing.T) {
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset := appclientset.NewSimpleClientset(&testApp)
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	_, err := RefreshApp(appIf, "test-app", argoappv1.RefreshTypeNormal)
	require.NoError(t, err)
	// For some reason, the fake Application interface doesn't reflect the patch status after Patch(),
	// so can't verify it was set in unit tests.
	//_, ok := newApp.Annotations[common.AnnotationKeyRefresh]
	//assert.True(t, ok)
}

func TestGetAppProjectWithNoProjDefined(t *testing.T) {
	projName := "default"
	namespace := "default"

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: test.FakeArgoCDNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
	}

	testProj := &argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projName, Namespace: namespace},
	}

	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = namespace
	appClientset := appclientset.NewSimpleClientset(testProj)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	informer := v1alpha1.NewAppProjectInformer(appClientset, namespace, 0, indexers)
	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	kubeClient := fake.NewSimpleClientset(&cm)
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, test.FakeArgoCDNamespace)
	argoDB := db.NewDB("default", settingsMgr, kubeClient)
	proj, err := GetAppProject(&testApp, applisters.NewAppProjectLister(informer.GetIndexer()), namespace, settingsMgr, argoDB, ctx)
	require.NoError(t, err)
	assert.Equal(t, proj.Name, projName)
}

func TestIncludeResource(t *testing.T) {
	// Resource filters format - GROUP:KIND:NAMESPACE/NAME or GROUP:KIND:NAME
	var (
		blankValues = argoappv1.SyncOperationResource{Group: "", Kind: "", Name: "", Namespace: "", Exclude: false}
		// *:*:*
		includeAllResources = argoappv1.SyncOperationResource{Group: "*", Kind: "*", Name: "*", Namespace: "", Exclude: false}
		// !*:*:*
		excludeAllResources = argoappv1.SyncOperationResource{Group: "*", Kind: "*", Name: "*", Namespace: "", Exclude: true}
		// *:Service:*
		includeAllServiceResources = argoappv1.SyncOperationResource{Group: "*", Kind: "Service", Name: "*", Namespace: "", Exclude: false}
		// !*:Service:*
		excludeAllServiceResources = argoappv1.SyncOperationResource{Group: "*", Kind: "Service", Name: "*", Namespace: "", Exclude: true}
		// apps:ReplicaSet:backend
		includeAllReplicaSetResource = argoappv1.SyncOperationResource{Group: "apps", Kind: "ReplicaSet", Name: "*", Namespace: "", Exclude: false}
		// apps:ReplicaSet:backend
		includeReplicaSetResource = argoappv1.SyncOperationResource{Group: "apps", Kind: "ReplicaSet", Name: "backend", Namespace: "", Exclude: false}
		// !apps:ReplicaSet:backend
		excludeReplicaSetResource = argoappv1.SyncOperationResource{Group: "apps", Kind: "ReplicaSet", Name: "backend", Namespace: "", Exclude: true}
	)
	tests := []struct {
		testName              string
		name                  string
		namespace             string
		gvk                   schema.GroupVersionKind
		syncOperationResource []*argoappv1.SyncOperationResource
		expectedResult        bool
	}{
		//--resource apps:ReplicaSet:backend --resource *:Service:*
		{
			testName:              "Include ReplicaSet backend resource and all service resources",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "apps", Kind: "ReplicaSet"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&includeAllServiceResources, &includeReplicaSetResource},
			expectedResult:        true,
		},
		//--resource apps:ReplicaSet:backend --resource *:Service:*
		{
			testName:              "Include ReplicaSet backend resource and all service resources",
			name:                  "main-page-down",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "batch", Kind: "Job"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&includeAllServiceResources, &includeReplicaSetResource},
			expectedResult:        false,
		},
		//--resource apps:ReplicaSet:backend --resource !*:Service:*
		{
			testName:              "Include ReplicaSet backend resource and exclude all service resources",
			name:                  "main-page-down",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "batch", Kind: "Job"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&excludeAllServiceResources, &includeReplicaSetResource},
			expectedResult:        false,
		},
		// --resource !apps:ReplicaSet:backend --resource !*:Service:*
		{
			testName:              "Exclude ReplicaSet backend resource and all service resources",
			name:                  "main-page-down",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "batch", Kind: "Job"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&excludeReplicaSetResource, &excludeAllServiceResources},
			expectedResult:        true,
		},
		// --resource !apps:ReplicaSet:backend
		{
			testName:              "Exclude ReplicaSet backend resource",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "apps", Kind: "ReplicaSet"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&excludeReplicaSetResource},
			expectedResult:        false,
		},
		// --resource !apps:ReplicaSet:backend --resource !*:Service:*
		{
			testName:              "Exclude ReplicaSet backend resource and all service resources(dummy condition)",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "apps", Kind: "ReplicaSet"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&excludeReplicaSetResource, &excludeAllServiceResources},
			expectedResult:        false,
		},
		// --resource apps:ReplicaSet:backend
		{
			testName:              "Include ReplicaSet backend resource",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "apps", Kind: "ReplicaSet"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&includeReplicaSetResource},
			expectedResult:        true,
		},
		// --resource !*:Service:*
		{
			testName:              "Exclude Service resources",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "", Kind: "Service"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&excludeAllServiceResources},
			expectedResult:        false,
		},
		// --resource *:Service:*
		{
			testName:              "Include Service resources",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "", Kind: "Service"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&includeAllServiceResources},
			expectedResult:        true,
		},
		// --resource apps:ReplicaSet:* --resource !apps:ReplicaSet:backend
		{
			testName:              "Include & Exclude ReplicaSet resources",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "apps", Kind: "ReplicaSet"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&includeAllReplicaSetResource, &excludeReplicaSetResource},
			expectedResult:        false,
		},
		// --resource !*:*:*
		{
			testName:              "Exclude all resources",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "", Kind: "Service"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&excludeAllResources},
			expectedResult:        false,
		},
		// --resource *:*:*
		{
			testName:              "Include all resources",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "", Kind: "Service"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&includeAllResources},
			expectedResult:        true,
		},
		{
			testName:              "No Filters",
			name:                  "backend",
			namespace:             "default",
			gvk:                   schema.GroupVersionKind{Group: "", Kind: "Service"},
			syncOperationResource: []*argoappv1.SyncOperationResource{&blankValues},
			expectedResult:        false,
		},
		{
			testName:       "Default values",
			expectedResult: true,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			isResourceIncluded := IncludeResource(test.name, test.namespace, test.gvk, test.syncOperationResource)
			assert.Equal(t, test.expectedResult, isResourceIncluded)
		})
	}
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
		if out := ContainsSyncResource(table.u.GetName(), table.u.GetNamespace(), table.u.GroupVersionKind(), table.rr); out != table.expected {
			t.Errorf("Expected %t for slice %+v contains resource %+v; instead got %t", table.expected, table.rr, table.u, out)
		}
	}
}

// TestNilOutZerValueAppSources verifies we will nil out app source specs when they are their zero-value
func TestNilOutZerValueAppSources(t *testing.T) {
	var spec *argoappv1.ApplicationSpec
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NamePrefix: "foo"}}})
		assert.NotNil(t, spec.GetSource().Kustomize)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NamePrefix: ""}}})
		source := spec.GetSource()
		assert.Nil(t, source.Kustomize)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NameSuffix: "foo"}}})
		assert.NotNil(t, spec.GetSource().Kustomize)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NameSuffix: ""}}})
		source := spec.GetSource()
		assert.Nil(t, source.Kustomize)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{"values.yaml"}}}})
		assert.NotNil(t, spec.GetSource().Helm)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{}}}})
		assert.Nil(t, spec.GetSource().Helm)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}})
		assert.NotNil(t, spec.GetSource().Directory)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: &argoappv1.ApplicationSource{Directory: &argoappv1.ApplicationSourceDirectory{Recurse: false}}})
		assert.Nil(t, spec.GetSource().Directory)
	}
}

func TestValidatePermissionsEmptyDestination(t *testing.T) {
	conditions, err := ValidatePermissions(context.Background(), &argoappv1.ApplicationSpec{
		Source: &argoappv1.ApplicationSource{RepoURL: "https://github.com/argoproj/argo-cd", Path: "."},
	}, &argoappv1.AppProject{
		Spec: argoappv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []argoappv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}, nil)
	require.NoError(t, err)
	assert.ElementsMatch(t, conditions, []argoappv1.ApplicationCondition{{Type: argoappv1.ApplicationConditionInvalidSpecError, Message: "Destination server missing from app spec"}})
}

func TestValidateChartWithoutRevision(t *testing.T) {
	appSpec := &argoappv1.ApplicationSpec{
		Source: &argoappv1.ApplicationSource{RepoURL: "https://charts.helm.sh/incubator/", Chart: "myChart", TargetRevision: ""},
		Destination: argoappv1.ApplicationDestination{
			Server: "https://kubernetes.default.svc", Namespace: "default",
		},
	}
	cluster := &argoappv1.Cluster{Server: "https://kubernetes.default.svc"}
	db := &dbmocks.ArgoDB{}
	ctx := context.Background()
	db.On("GetCluster", ctx, appSpec.Destination.Server).Return(cluster, nil)

	conditions, err := ValidatePermissions(ctx, appSpec, &argoappv1.AppProject{
		Spec: argoappv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []argoappv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}, db)
	require.NoError(t, err)
	assert.Len(t, conditions, 1)
	assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
	assert.Equal(t, "spec.source.targetRevision is required if the manifest source is a helm chart", conditions[0].Message)
}

func TestAPIResourcesToStrings(t *testing.T) {
	resources := []kube.APIResourceInfo{{
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1beta1"},
		GroupKind:            schema.GroupKind{Kind: "Deployment"},
	}, {
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1beta2"},
		GroupKind:            schema.GroupKind{Kind: "Deployment"},
	}, {
		GroupVersionResource: schema.GroupVersionResource{Group: "extensions", Version: "v1beta1"},
		GroupKind:            schema.GroupKind{Kind: "Deployment"},
	}}

	assert.ElementsMatch(t, []string{"apps/v1beta1", "apps/v1beta2", "extensions/v1beta1"}, APIResourcesToStrings(resources, false))
	assert.ElementsMatch(t, []string{
		"apps/v1beta1", "apps/v1beta1/Deployment", "apps/v1beta2", "apps/v1beta2/Deployment", "extensions/v1beta1", "extensions/v1beta1/Deployment",
	},
		APIResourcesToStrings(resources, true))
}

func TestValidateRepo(t *testing.T) {
	repoPath, err := filepath.Abs("./../..")
	require.NoError(t, err)

	apiResources := []kube.APIResourceInfo{{
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1beta1"},
		GroupKind:            schema.GroupKind{Kind: "Deployment"},
	}, {
		GroupVersionResource: schema.GroupVersionResource{Group: "apps", Version: "v1beta2"},
		GroupKind:            schema.GroupKind{Kind: "Deployment"},
	}}
	kubeVersion := "v1.16"
	kustomizeOptions := &argoappv1.KustomizeOptions{BuildOptions: ""}
	repo := &argoappv1.Repository{Repo: fmt.Sprintf("file://%s", repoPath)}
	cluster := &argoappv1.Cluster{Server: "sample server"}
	app := &argoappv1.Application{
		Spec: argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL: repo.Repo,
			},
			Destination: argoappv1.ApplicationDestination{
				Server:    cluster.Server,
				Namespace: "default",
			},
		},
	}

	proj := &argoappv1.AppProject{
		Spec: argoappv1.AppProjectSpec{
			SourceRepos: []string{"*"},
		},
	}

	helmRepos := []*argoappv1.Repository{{Repo: "sample helm repo"}}

	repoClient := &mocks.RepoServerServiceClient{}
	source := app.Spec.GetSource()
	repoClient.On("GetAppDetails", context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo:             repo,
		Source:           &source,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
		HelmOptions:      &argoappv1.HelmOptions{ValuesFileSchemes: []string{"https", "http"}},
		NoRevisionCache:  true,
	}).Return(&apiclient.RepoAppDetailsResponse{}, nil)

	repo.Type = "git"
	repoClient.On("TestRepository", context.Background(), &apiclient.TestRepositoryRequest{
		Repo: repo,
	}).Return(&apiclient.TestRepositoryResponse{
		VerifiedRepository: true,
	}, nil)

	repoClientSet := &mocks.Clientset{RepoServerServiceClient: repoClient}

	db := &dbmocks.ArgoDB{}

	db.On("GetRepository", context.Background(), app.Spec.Source.RepoURL, "").Return(repo, nil)
	db.On("ListHelmRepositories", context.Background()).Return(helmRepos, nil)
	db.On("GetCluster", context.Background(), app.Spec.Destination.Server).Return(cluster, nil)
	db.On("GetAllHelmRepositoryCredentials", context.Background()).Return(nil, nil)

	var receivedRequest *apiclient.ManifestRequest

	repoClient.On("GenerateManifest", context.Background(), mock.MatchedBy(func(req *apiclient.ManifestRequest) bool {
		receivedRequest = req
		return true
	})).Return(nil, nil)

	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: test.FakeArgoCDNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{
			"globalProjects": `
 - projectName: default-x
   labelSelector:
     matchExpressions:
      - key: is-x
        operator: Exists
 - projectName: default-non-x
   labelSelector:
     matchExpressions:
      - key: is-x
        operator: DoesNotExist
`,
		},
	}

	kubeClient := fake.NewSimpleClientset(&cm)
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, test.FakeArgoCDNamespace)

	conditions, err := ValidateRepo(context.Background(), app, repoClientSet, db, &kubetest.MockKubectlCmd{Version: kubeVersion, APIResources: apiResources}, proj, settingsMgr)

	require.NoError(t, err)
	assert.Empty(t, conditions)
	assert.ElementsMatch(t, []string{"apps/v1beta1", "apps/v1beta1/Deployment", "apps/v1beta2", "apps/v1beta2/Deployment"}, receivedRequest.ApiVersions)
	assert.Equal(t, kubeVersion, receivedRequest.KubeVersion)
	assert.Equal(t, app.Spec.Destination.Namespace, receivedRequest.Namespace)
	assert.Equal(t, &source, receivedRequest.ApplicationSource)
	assert.Equal(t, kustomizeOptions, receivedRequest.KustomizeOptions)
}

func TestFormatAppConditions(t *testing.T) {
	conditions := []argoappv1.ApplicationCondition{
		{
			Type:    EventReasonOperationCompleted,
			Message: "Foo",
		},
		{
			Type:    EventReasonResourceCreated,
			Message: "Bar",
		},
	}

	t.Run("Single Condition", func(t *testing.T) {
		res := FormatAppConditions(conditions[0:1])
		assert.NotEmpty(t, res)
		assert.Equal(t, fmt.Sprintf("%s: Foo", EventReasonOperationCompleted), res)
	})

	t.Run("Multiple Conditions", func(t *testing.T) {
		res := FormatAppConditions(conditions)
		assert.NotEmpty(t, res)
		assert.Equal(t, fmt.Sprintf("%s: Foo;%s: Bar", EventReasonOperationCompleted, EventReasonResourceCreated), res)
	})

	t.Run("Empty Conditions", func(t *testing.T) {
		res := FormatAppConditions([]argoappv1.ApplicationCondition{})
		assert.Empty(t, res)
	})
}

func TestFilterByProjects(t *testing.T) {
	apps := []argoappv1.Application{
		{
			Spec: argoappv1.ApplicationSpec{
				Project: "fooproj",
			},
		},
		{
			Spec: argoappv1.ApplicationSpec{
				Project: "barproj",
			},
		},
	}

	t.Run("No apps in single project", func(t *testing.T) {
		res := FilterByProjects(apps, []string{"foobarproj"})
		assert.Empty(t, res)
	})

	t.Run("Single app in single project", func(t *testing.T) {
		res := FilterByProjects(apps, []string{"fooproj"})
		assert.Len(t, res, 1)
	})

	t.Run("Single app in multiple project", func(t *testing.T) {
		res := FilterByProjects(apps, []string{"fooproj", "foobarproj"})
		assert.Len(t, res, 1)
	})

	t.Run("Multiple apps in multiple project", func(t *testing.T) {
		res := FilterByProjects(apps, []string{"fooproj", "barproj"})
		assert.Len(t, res, 2)
	})
}

func TestFilterByProjectsP(t *testing.T) {
	apps := []*argoappv1.Application{
		{
			Spec: argoappv1.ApplicationSpec{
				Project: "fooproj",
			},
		},
		{
			Spec: argoappv1.ApplicationSpec{
				Project: "barproj",
			},
		},
	}

	t.Run("No apps in single project", func(t *testing.T) {
		res := FilterByProjectsP(apps, []string{"foobarproj"})
		assert.Empty(t, res)
	})

	t.Run("Single app in single project", func(t *testing.T) {
		res := FilterByProjectsP(apps, []string{"fooproj"})
		assert.Len(t, res, 1)
	})

	t.Run("Single app in multiple project", func(t *testing.T) {
		res := FilterByProjectsP(apps, []string{"fooproj", "foobarproj"})
		assert.Len(t, res, 1)
	})

	t.Run("Multiple apps in multiple project", func(t *testing.T) {
		res := FilterByProjectsP(apps, []string{"fooproj", "barproj"})
		assert.Len(t, res, 2)
	})
}

func TestFilterByRepo(t *testing.T) {
	apps := []argoappv1.Application{
		{
			Spec: argoappv1.ApplicationSpec{
				Source: &argoappv1.ApplicationSource{
					RepoURL: "git@github.com:owner/repo.git",
				},
			},
		},
		{
			Spec: argoappv1.ApplicationSpec{
				Source: &argoappv1.ApplicationSource{
					RepoURL: "git@github.com:owner/otherrepo.git",
				},
			},
		},
	}

	t.Run("Empty filter", func(t *testing.T) {
		res := FilterByRepo(apps, "")
		assert.Len(t, res, 2)
	})

	t.Run("Match", func(t *testing.T) {
		res := FilterByRepo(apps, "git@github.com:owner/repo.git")
		assert.Len(t, res, 1)
	})

	t.Run("No match", func(t *testing.T) {
		res := FilterByRepo(apps, "git@github.com:owner/willnotmatch.git")
		assert.Empty(t, res)
	})
}

func TestFilterByRepoP(t *testing.T) {
	apps := []*argoappv1.Application{
		{
			Spec: argoappv1.ApplicationSpec{
				Source: &argoappv1.ApplicationSource{
					RepoURL: "git@github.com:owner/repo.git",
				},
			},
		},
		{
			Spec: argoappv1.ApplicationSpec{
				Source: &argoappv1.ApplicationSource{
					RepoURL: "git@github.com:owner/otherrepo.git",
				},
			},
		},
	}

	t.Run("Empty filter", func(t *testing.T) {
		res := FilterByRepoP(apps, "")
		assert.Len(t, res, 2)
	})

	t.Run("Match", func(t *testing.T) {
		res := FilterByRepoP(apps, "git@github.com:owner/repo.git")
		assert.Len(t, res, 1)
	})

	t.Run("No match", func(t *testing.T) {
		res := FilterByRepoP(apps, "git@github.com:owner/willnotmatch.git")
		assert.Empty(t, res)
	})
}

func TestValidatePermissions(t *testing.T) {
	t.Run("Empty Repo URL result in condition", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL: "",
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "are required")
	})

	t.Run("Incomplete Path/Chart combo result in condition", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL: "http://some/where",
				Path:    "",
				Chart:   "",
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "are required")
	})

	t.Run("Helm chart requires targetRevision", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL: "http://some/where",
				Path:    "",
				Chart:   "somechart",
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "is required if the manifest source is a helm chart")
	})

	t.Run("Application source is not permitted in project", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL:        "http://some/where",
				Path:           "",
				Chart:          "somechart",
				TargetRevision: "1.4.1",
			},
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://127.0.0.1:6443",
				Namespace: "testns",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "*",
					},
				},
				SourceRepos: []string{"http://some/where/else"},
			},
		}
		cluster := &argoappv1.Cluster{Server: "https://127.0.0.1:6443"}
		db := &dbmocks.ArgoDB{}
		db.On("GetCluster", context.Background(), spec.Destination.Server).Return(cluster, nil)
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "application repo http://some/where is not permitted")
	})

	t.Run("Application destination is not permitted in project", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL:        "http://some/where",
				Path:           "",
				Chart:          "somechart",
				TargetRevision: "1.4.1",
			},
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://127.0.0.1:6443",
				Namespace: "testns",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "default",
					},
				},
				SourceRepos: []string{"http://some/where"},
			},
		}
		cluster := &argoappv1.Cluster{Server: "https://127.0.0.1:6443"}
		db := &dbmocks.ArgoDB{}
		db.On("GetCluster", context.Background(), spec.Destination.Server).Return(cluster, nil)
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "application destination")
	})

	t.Run("Destination cluster does not exist", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL:        "http://some/where",
				Path:           "",
				Chart:          "somechart",
				TargetRevision: "1.4.1",
			},
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://127.0.0.1:6443",
				Namespace: "default",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "default",
					},
				},
				SourceRepos: []string{"http://some/where"},
			},
		}
		db := &dbmocks.ArgoDB{}
		db.On("GetCluster", context.Background(), spec.Destination.Server).Return(nil, status.Errorf(codes.NotFound, "Cluster does not exist"))
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "has not been configured")
	})

	t.Run("Destination cluster name does not exist", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL:        "http://some/where",
				Path:           "",
				Chart:          "somechart",
				TargetRevision: "1.4.1",
			},
			Destination: argoappv1.ApplicationDestination{
				Name:      "does-not-exist",
				Namespace: "default",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "default",
					},
				},
				SourceRepos: []string{"http://some/where"},
			},
		}
		db := &dbmocks.ArgoDB{}
		db.On("GetClusterServersByName", context.Background(), "does-not-exist").Return(nil, nil)
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "unable to find destination server: there are no clusters with this name: does-not-exist")
	})

	t.Run("Cannot get cluster info from DB", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL:        "http://some/where",
				Path:           "",
				Chart:          "somechart",
				TargetRevision: "1.4.1",
			},
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://127.0.0.1:6443",
				Namespace: "default",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "default",
					},
				},
				SourceRepos: []string{"http://some/where"},
			},
		}
		db := &dbmocks.ArgoDB{}
		db.On("GetCluster", context.Background(), spec.Destination.Server).Return(nil, fmt.Errorf("Unknown error occurred"))
		_, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.Error(t, err)
	})

	t.Run("Destination cluster name resolves to valid server", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: &argoappv1.ApplicationSource{
				RepoURL:        "http://some/where",
				Path:           "",
				Chart:          "somechart",
				TargetRevision: "1.4.1",
			},
			Destination: argoappv1.ApplicationDestination{
				Name:      "does-exist",
				Namespace: "default",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "default",
					},
				},
				SourceRepos: []string{"http://some/where"},
			},
		}
		db := &dbmocks.ArgoDB{}
		cluster := argoappv1.Cluster{
			Name:   "does-exist",
			Server: "https://127.0.0.1:6443",
		}
		db.On("GetClusterServersByName", context.Background(), "does-exist").Return([]string{"https://127.0.0.1:6443"}, nil)
		db.On("GetCluster", context.Background(), "https://127.0.0.1:6443").Return(&cluster, nil)
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Empty(t, conditions)
	})
}

func TestSetAppOperations(t *testing.T) {
	t.Run("Application not existing", func(t *testing.T) {
		appIf := appclientset.NewSimpleClientset().ArgoprojV1alpha1().Applications("default")
		app, err := SetAppOperation(appIf, "someapp", &argoappv1.Operation{Sync: &argoappv1.SyncOperation{Revision: "aaa"}})
		require.Error(t, err)
		assert.Nil(t, app)
	})

	t.Run("Operation already in progress", func(t *testing.T) {
		a := argoappv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "someapp",
				Namespace: "default",
			},
			Operation: &argoappv1.Operation{Sync: &argoappv1.SyncOperation{Revision: "aaa"}},
		}
		appIf := appclientset.NewSimpleClientset(&a).ArgoprojV1alpha1().Applications("default")
		app, err := SetAppOperation(appIf, "someapp", &argoappv1.Operation{Sync: &argoappv1.SyncOperation{Revision: "aaa"}})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "operation is already in progress")
		assert.Nil(t, app)
	})

	t.Run("Operation unspecified", func(t *testing.T) {
		a := argoappv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "someapp",
				Namespace: "default",
			},
		}
		appIf := appclientset.NewSimpleClientset(&a).ArgoprojV1alpha1().Applications("default")
		app, err := SetAppOperation(appIf, "someapp", &argoappv1.Operation{Sync: nil})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Operation unspecified")
		assert.Nil(t, app)
	})

	t.Run("Success", func(t *testing.T) {
		a := argoappv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "someapp",
				Namespace: "default",
			},
		}
		appIf := appclientset.NewSimpleClientset(&a).ArgoprojV1alpha1().Applications("default")
		app, err := SetAppOperation(appIf, "someapp", &argoappv1.Operation{Sync: &argoappv1.SyncOperation{Revision: "aaa"}})
		require.NoError(t, err)
		assert.NotNil(t, app)
	})
}

func TestValidateDestination(t *testing.T) {
	t.Run("Validate destination with server url", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Server:    "https://127.0.0.1:6443",
			Namespace: "default",
		}

		appCond := ValidateDestination(context.Background(), &dest, nil)
		require.NoError(t, appCond)
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Validate destination with server name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		db := &dbmocks.ArgoDB{}
		db.On("GetClusterServersByName", context.Background(), "minikube").Return([]string{"https://127.0.0.1:6443"}, nil)

		appCond := ValidateDestination(context.Background(), &dest, db)
		require.NoError(t, appCond)
		assert.Equal(t, "https://127.0.0.1:6443", dest.Server)
		assert.True(t, dest.IsServerInferred())
	})

	t.Run("Error when having both server url and name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Server:    "https://127.0.0.1:6443",
			Name:      "minikube",
			Namespace: "default",
		}

		err := ValidateDestination(context.Background(), &dest, nil)
		assert.Equal(t, "application destination can't have both name and server defined: minikube https://127.0.0.1:6443", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("GetClusterServersByName fails", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		db := &dbmocks.ArgoDB{}
		db.On("GetClusterServersByName", context.Background(), mock.Anything).Return(nil, fmt.Errorf("an error occurred"))

		err := ValidateDestination(context.Background(), &dest, db)
		assert.Contains(t, err.Error(), "an error occurred")
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Destination cluster does not exist", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		db := &dbmocks.ArgoDB{}
		db.On("GetClusterServersByName", context.Background(), "minikube").Return(nil, nil)

		err := ValidateDestination(context.Background(), &dest, db)
		assert.Equal(t, "unable to find destination server: there are no clusters with this name: minikube", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Validate too many clusters with the same name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "dind",
		}

		db := &dbmocks.ArgoDB{}
		db.On("GetClusterServersByName", context.Background(), "dind").Return([]string{"https://127.0.0.1:2443", "https://127.0.0.1:8443"}, nil)

		err := ValidateDestination(context.Background(), &dest, db)
		assert.Equal(t, "unable to find destination server: there are 2 clusters with the same name: [https://127.0.0.1:2443 https://127.0.0.1:8443]", err.Error())
		assert.False(t, dest.IsServerInferred())
	})
}

func TestFilterByName(t *testing.T) {
	apps := []argoappv1.Application{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: argoappv1.ApplicationSpec{
				Project: "fooproj",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar",
			},
			Spec: argoappv1.ApplicationSpec{
				Project: "barproj",
			},
		},
	}

	t.Run("Name is empty string", func(t *testing.T) {
		res, err := FilterByName(apps, "")
		require.NoError(t, err)
		assert.Len(t, res, 2)
	})

	t.Run("Single app by name", func(t *testing.T) {
		res, err := FilterByName(apps, "foo")
		require.NoError(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("No such app", func(t *testing.T) {
		res, err := FilterByName(apps, "foobar")
		require.Error(t, err)
		assert.Empty(t, res)
	})
}

func TestFilterByNameP(t *testing.T) {
	apps := []*argoappv1.Application{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "foo",
			},
			Spec: argoappv1.ApplicationSpec{
				Project: "fooproj",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "bar",
			},
			Spec: argoappv1.ApplicationSpec{
				Project: "barproj",
			},
		},
	}

	t.Run("Name is empty string", func(t *testing.T) {
		res := FilterByNameP(apps, "")
		assert.Len(t, res, 2)
	})

	t.Run("Single app by name", func(t *testing.T) {
		res := FilterByNameP(apps, "foo")
		assert.Len(t, res, 1)
	})

	t.Run("No such app", func(t *testing.T) {
		res := FilterByNameP(apps, "foobar")
		assert.Empty(t, res)
	})
}

func TestGetGlobalProjects(t *testing.T) {
	t.Run("Multiple global projects", func(t *testing.T) {
		namespace := "default"

		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "argocd-cm",
				Namespace: test.FakeArgoCDNamespace,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "argocd",
				},
			},
			Data: map[string]string{
				"globalProjects": `
 - projectName: default-x
   labelSelector:
     matchExpressions:
      - key: is-x
        operator: Exists
 - projectName: default-non-x
   labelSelector:
     matchExpressions:
      - key: is-x
        operator: DoesNotExist
`,
			},
		}

		defaultX := &argoappv1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "default-x", Namespace: namespace},
			Spec: argoappv1.AppProjectSpec{
				ClusterResourceWhitelist: []metav1.GroupKind{
					{Group: "*", Kind: "*"},
				},
				ClusterResourceBlacklist: []metav1.GroupKind{
					{Kind: "Volume"},
				},
			},
		}

		defaultNonX := &argoappv1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "default-non-x", Namespace: namespace},
			Spec: argoappv1.AppProjectSpec{
				ClusterResourceBlacklist: []metav1.GroupKind{
					{Group: "*", Kind: "*"},
				},
			},
		}

		isX := &argoappv1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "is-x",
				Namespace: namespace,
				Labels: map[string]string{
					"is-x": "yep",
				},
			},
		}

		isNoX := &argoappv1.AppProject{
			ObjectMeta: metav1.ObjectMeta{Name: "is-no-x", Namespace: namespace},
		}

		projClientset := appclientset.NewSimpleClientset(defaultX, defaultNonX, isX, isNoX)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
		informer := v1alpha1.NewAppProjectInformer(projClientset, namespace, 0, indexers)
		go informer.Run(ctx.Done())
		cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

		kubeClient := fake.NewSimpleClientset(&cm)
		settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, test.FakeArgoCDNamespace)

		projLister := applisters.NewAppProjectLister(informer.GetIndexer())

		xGlobalProjects := GetGlobalProjects(isX, projLister, settingsMgr)
		assert.Len(t, xGlobalProjects, 1)
		assert.Equal(t, "default-x", xGlobalProjects[0].Name)

		nonXGlobalProjects := GetGlobalProjects(isNoX, projLister, settingsMgr)
		assert.Len(t, nonXGlobalProjects, 1)
		assert.Equal(t, "default-non-x", nonXGlobalProjects[0].Name)
	})
}

func Test_GetDifferentPathsBetweenStructs(t *testing.T) {
	r1 := argoappv1.Repository{}
	r2 := argoappv1.Repository{
		Name: "SomeName",
	}

	difference, _ := GetDifferentPathsBetweenStructs(r1, r2)
	assert.Equal(t, []string{"Name"}, difference)
}

func Test_GenerateSpecIsDifferentErrorMessageWithNoDiff(t *testing.T) {
	r1 := argoappv1.Repository{}
	r2 := argoappv1.Repository{}

	msg := GenerateSpecIsDifferentErrorMessage("application", r1, r2)
	assert.Equal(t, "existing application spec is different; use upsert flag to force update", msg)
}

func Test_GenerateSpecIsDifferentErrorMessageWithDiff(t *testing.T) {
	r1 := argoappv1.Repository{}
	r2 := argoappv1.Repository{
		Name: "test",
	}

	msg := GenerateSpecIsDifferentErrorMessage("repo", r1, r2)
	assert.Equal(t, "existing repo spec is different; use upsert flag to force update; difference in keys \"Name\"", msg)
}

func Test_ParseAppQualifiedName(t *testing.T) {
	testcases := []struct {
		name       string
		input      string
		implicitNs string
		appName    string
		appNs      string
	}{
		{"Full qualified without implicit NS", "namespace/name", "", "name", "namespace"},
		{"Non qualified without implicit NS", "name", "", "name", ""},
		{"Full qualified with implicit NS", "namespace/name", "namespace2", "name", "namespace"},
		{"Non qualified with implicit NS", "name", "namespace2", "name", "namespace2"},
		{"Invalid without implicit NS", "namespace_name", "", "namespace_name", ""},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			appName, appNs := ParseFromQualifiedName(tt.input, tt.implicitNs)
			assert.Equal(t, tt.appName, appName)
			assert.Equal(t, tt.appNs, appNs)
		})
	}
}

func Test_ParseAppInstanceName(t *testing.T) {
	testcases := []struct {
		name       string
		input      string
		implicitNs string
		appName    string
		appNs      string
	}{
		{"Full qualified without implicit NS", "namespace_name", "", "name", "namespace"},
		{"Non qualified without implicit NS", "name", "", "name", ""},
		{"Full qualified with implicit NS", "namespace_name", "namespace2", "name", "namespace"},
		{"Non qualified with implicit NS", "name", "namespace2", "name", "namespace2"},
		{"Invalid without implicit NS", "namespace/name", "", "namespace/name", ""},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			appName, appNs := ParseInstanceName(tt.input, tt.implicitNs)
			assert.Equal(t, tt.appName, appName)
			assert.Equal(t, tt.appNs, appNs)
		})
	}
}

func Test_AppInstanceName(t *testing.T) {
	testcases := []struct {
		name         string
		appName      string
		appNamespace string
		defaultNs    string
		result       string
	}{
		{"defaultns different as appns", "appname", "appns", "defaultns", "appns_appname"},
		{"defaultns same as appns", "appname", "appns", "appns", "appname"},
		{"defaultns set and appns not given", "appname", "", "appns", "appname"},
		{"neither defaultns nor appns set", "appname", "", "appns", "appname"},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			result := AppInstanceName(tt.appName, tt.appNamespace, tt.defaultNs)
			assert.Equal(t, tt.result, result)
		})
	}
}

func Test_AppInstanceNameFromQualified(t *testing.T) {
	testcases := []struct {
		name      string
		appName   string
		defaultNs string
		result    string
	}{
		{"Qualified name with namespace not being defaultns", "appns/appname", "defaultns", "appns_appname"},
		{"Qualified name with namespace being defaultns", "defaultns/appname", "defaultns", "appname"},
		{"Qualified name without namespace", "appname", "defaultns", "appname"},
		{"Qualified name without namespace and defaultns", "appname", "", "appname"},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			result := InstanceNameFromQualified(tt.appName, tt.defaultNs)
			assert.Equal(t, tt.result, result)
		})
	}
}

func Test_GetRefSources(t *testing.T) {
	repoPath, err := filepath.Abs("./../..")
	require.NoError(t, err)

	getMultiSourceAppSpec := func(sources argoappv1.ApplicationSources) *argoappv1.ApplicationSpec {
		return &argoappv1.ApplicationSpec{
			Sources: sources,
		}
	}

	repo := &argoappv1.Repository{Repo: fmt.Sprintf("file://%s", repoPath)}

	t.Run("target ref exists", func(t *testing.T) {
		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: fmt.Sprintf("file://%s", repoPath), Ref: "source-1_2"},
			{RepoURL: fmt.Sprintf("file://%s", repoPath)},
		})

		refSources, err := GetRefSources(context.Background(), argoSpec.Sources, argoSpec.Project, func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
			return repo, nil
		}, []string{}, false)

		expectedRefSource := argoappv1.RefTargetRevisionMapping{
			"$source-1_2": &argoappv1.RefTarget{
				Repo: *repo,
			},
		}
		require.NoError(t, err)
		assert.Len(t, refSources, 1)
		assert.Equal(t, expectedRefSource, refSources)
	})

	t.Run("target ref does not exist", func(t *testing.T) {
		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: "file://does-not-exist", Ref: "source1"},
			{RepoURL: fmt.Sprintf("file://%s", repoPath)},
		})

		refSources, err := GetRefSources(context.Background(), argoSpec.Sources, argoSpec.Project, func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
			return nil, errors.New("repo does not exist")
		}, []string{}, false)

		require.Error(t, err)
		assert.Empty(t, refSources)
	})

	t.Run("invalid ref", func(t *testing.T) {
		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: "file://does-not-exist", Ref: "%invalid-name%"},
			{RepoURL: fmt.Sprintf("file://%s", repoPath)},
		})

		refSources, err := GetRefSources(context.TODO(), argoSpec.Sources, argoSpec.Project, func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
			return nil, err
		}, []string{}, false)

		require.Error(t, err)
		assert.Empty(t, refSources)
	})

	t.Run("duplicate ref keys", func(t *testing.T) {
		argoSpec := getMultiSourceAppSpec(argoappv1.ApplicationSources{
			{RepoURL: "file://does-not-exist", Ref: "source1"},
			{RepoURL: "file://does-not-exist", Ref: "source1"},
		})

		refSources, err := GetRefSources(context.TODO(), argoSpec.Sources, argoSpec.Project, func(ctx context.Context, url string, project string) (*argoappv1.Repository, error) {
			return nil, err
		}, []string{}, false)

		require.Error(t, err)
		assert.Empty(t, refSources)
	})
}

func TestValidatePermissionsMultipleSources(t *testing.T) {
	t.Run("Empty Repo URL result in condition", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Sources: argoappv1.ApplicationSources{
				{RepoURL: ""},
			},
		}

		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "are required")
	})

	t.Run("Incomplete Path/Chart/Ref combo result in condition", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Sources: argoappv1.ApplicationSources{
				{
					RepoURL: "http://some/where",
					Path:    "",
					Chart:   "",
					Ref:     "",
				},
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "are required")
	})

	t.Run("One of the Application sources is not permitted in project", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Sources: argoappv1.ApplicationSources{
				{
					RepoURL:        "http://some/where",
					Path:           "",
					Chart:          "somechart",
					TargetRevision: "1.4.1",
				},
			},
			Destination: argoappv1.ApplicationDestination{
				Server:    "https://127.0.0.1:6443",
				Namespace: "testns",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "*",
					},
				},
				SourceRepos: []string{"http://some/where/else"},
			},
		}
		cluster := &argoappv1.Cluster{Server: "https://127.0.0.1:6443"}
		db := &dbmocks.ArgoDB{}
		db.On("GetCluster", context.Background(), spec.Destination.Server).Return(cluster, nil)
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "application repo http://some/where is not permitted")
	})

	t.Run("Source with a Ref field and missing Path/Chart field", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Sources: argoappv1.ApplicationSources{
				{
					RepoURL: "http://some/where",
					Path:    "",
					Chart:   "",
					Ref:     "somechart",
				},
			},
			Destination: argoappv1.ApplicationDestination{
				Name:      "does-exist",
				Namespace: "default",
			},
		}
		proj := argoappv1.AppProject{
			Spec: argoappv1.AppProjectSpec{
				Destinations: []argoappv1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "default",
					},
				},
				SourceRepos: []string{"http://some/where"},
			},
		}
		db := &dbmocks.ArgoDB{}
		cluster := argoappv1.Cluster{
			Name:   "does-exist",
			Server: "https://127.0.0.1:6443",
		}
		db.On("GetClusterServersByName", context.Background(), "does-exist").Return([]string{"https://127.0.0.1:6443"}, nil)
		db.On("GetCluster", context.Background(), "https://127.0.0.1:6443").Return(&cluster, nil)
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		require.NoError(t, err)
		assert.Empty(t, conditions)
	})
}

func TestAugmentSyncMsg(t *testing.T) {
	mockAPIResourcesFn := func() ([]kube.APIResourceInfo, error) {
		return []kube.APIResourceInfo{
			{
				GroupKind: schema.GroupKind{
					Group: "apps",
					Kind:  "Deployment",
				},
				GroupVersionResource: schema.GroupVersionResource{
					Group:   "apps",
					Version: "v1",
				},
			},
			{
				GroupKind: schema.GroupKind{
					Group: "networking.k8s.io",
					Kind:  "Ingress",
				},
				GroupVersionResource: schema.GroupVersionResource{
					Group:   "networking.k8s.io",
					Version: "v1",
				},
			},
		}, nil
	}

	testcases := []struct {
		name            string
		msg             string
		expectedMessage string
		res             common.ResourceSyncResult
		mockFn          func() ([]kube.APIResourceInfo, error)
		errMsg          string
	}{
		{
			name: "match specific k8s error",
			msg:  "the server could not find the requested resource",
			res: common.ResourceSyncResult{
				ResourceKey: kube.ResourceKey{
					Name:      "deployment-resource",
					Namespace: "test-namespace",
					Kind:      "Deployment",
					Group:     "apps",
				},
				Version: "v1beta1",
			},
			expectedMessage: "The Kubernetes API could not find version \"v1beta1\" of apps/Deployment for requested resource test-namespace/deployment-resource. Version \"v1\" of apps/Deployment is installed on the destination cluster.",
			mockFn:          mockAPIResourcesFn,
		},
		{
			name: "any random k8s msg",
			msg:  "random message from k8s",
			res: common.ResourceSyncResult{
				ResourceKey: kube.ResourceKey{
					Name:      "deployment-resource",
					Namespace: "test-namespace",
					Kind:      "Deployment",
					Group:     "apps",
				},
				Version: "v1beta1",
			},
			expectedMessage: "random message from k8s",
			mockFn:          mockAPIResourcesFn,
		},
		{
			name: "resource doesn't exist in the target cluster",
			res: common.ResourceSyncResult{
				ResourceKey: kube.ResourceKey{
					Name:      "persistent-volume-resource",
					Namespace: "test-namespace",
					Kind:      "PersistentVolume",
					Group:     "",
				},
				Version: "v1",
			},
			msg:             "the server could not find the requested resource",
			expectedMessage: "The Kubernetes API could not find /PersistentVolume for requested resource test-namespace/persistent-volume-resource. Make sure the \"PersistentVolume\" CRD is installed on the destination cluster.",
			mockFn:          mockAPIResourcesFn,
		},
		{
			name: "API Resource returns error",
			res: common.ResourceSyncResult{
				ResourceKey: kube.ResourceKey{
					Name:      "persistent-volume-resource",
					Namespace: "test-namespace",
					Kind:      "PersistentVolume",
					Group:     "",
				},
				Version: "v1",
			},
			msg:             "the server could not find the requested resource",
			expectedMessage: "the server could not find the requested resource",
			mockFn: func() ([]kube.APIResourceInfo, error) {
				return nil, errors.New("failed to fetch resource of given kind %s from the target cluster")
			},
			errMsg: "failed to get API resource info for group \"\" and kind \"PersistentVolume\": failed to get API resource info: failed to fetch resource of given kind %s from the target cluster",
		},
		{
			name: "old Ingress type returns error suggesting new Ingress type",
			res: common.ResourceSyncResult{
				ResourceKey: kube.ResourceKey{
					Name:      "ingress-resource",
					Namespace: "test-namespace",
					Kind:      "Ingress",
					Group:     "extensions",
				},
				Version: "v1beta1",
			},
			msg:             "the server could not find the requested resource",
			expectedMessage: "The Kubernetes API could not find version \"v1beta1\" of extensions/Ingress for requested resource test-namespace/ingress-resource. Version \"v1\" of networking.k8s.io/Ingress is installed on the destination cluster.",
			mockFn:          mockAPIResourcesFn,
		},
	}

	for _, tt := range testcases {
		t.Run(tt.name, func(t *testing.T) {
			tt.res.Message = tt.msg
			msg, err := AugmentSyncMsg(tt.res, tt.mockFn)
			if tt.errMsg != "" {
				require.Error(t, err)
				assert.Equal(t, tt.errMsg, err.Error())
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedMessage, msg)
			}
		})
	}
}

func TestGetAppEventLabels(t *testing.T) {
	tests := []struct {
		name                string
		cmInEventLabelKeys  string
		cmExEventLabelKeys  string
		appLabels           map[string]string
		projLabels          map[string]string
		expectedEventLabels map[string]string
	}{
		{
			name:                "no label keys in cm - no event labels",
			cmInEventLabelKeys:  "",
			appLabels:           map[string]string{"team": "A", "tier": "frontend"},
			projLabels:          map[string]string{"environment": "dev"},
			expectedEventLabels: nil,
		},
		{
			name:                "label keys in cm, no labels on app & proj - no event labels",
			cmInEventLabelKeys:  "team, environment",
			appLabels:           nil,
			projLabels:          nil,
			expectedEventLabels: nil,
		},
		{
			name:                "labels on app, no labels on proj - event labels matched on app only",
			cmInEventLabelKeys:  "team, environment",
			appLabels:           map[string]string{"team": "A", "tier": "frontend"},
			projLabels:          nil,
			expectedEventLabels: map[string]string{"team": "A"},
		},
		{
			name:                "no labels on app, labels on proj - event labels matched on proj only",
			cmInEventLabelKeys:  "team, environment",
			appLabels:           nil,
			projLabels:          map[string]string{"environment": "dev"},
			expectedEventLabels: map[string]string{"environment": "dev"},
		},
		{
			name:                "labels on app & proj with conflicts - event labels matched on both app & proj and app labels prioritized on conflict",
			cmInEventLabelKeys:  "team, environment",
			appLabels:           map[string]string{"team": "A", "environment": "stage", "tier": "frontend"},
			projLabels:          map[string]string{"environment": "dev"},
			expectedEventLabels: map[string]string{"team": "A", "environment": "stage"},
		},
		{
			name:                "wildcard support - matched all labels",
			cmInEventLabelKeys:  "*",
			appLabels:           map[string]string{"team": "A", "tier": "frontend"},
			projLabels:          map[string]string{"environment": "dev"},
			expectedEventLabels: map[string]string{"team": "A", "tier": "frontend", "environment": "dev"},
		},
		{
			name:                "exclude event labels",
			cmInEventLabelKeys:  "example.com/team,tier,env*",
			cmExEventLabelKeys:  "tie*",
			appLabels:           map[string]string{"example.com/team": "A", "tier": "frontend"},
			projLabels:          map[string]string{"environment": "dev"},
			expectedEventLabels: map[string]string{"example.com/team": "A", "environment": "dev"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cm := corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "argocd-cm",
					Namespace: test.FakeArgoCDNamespace,
					Labels: map[string]string{
						"app.kubernetes.io/part-of": "argocd",
					},
				},
				Data: map[string]string{
					"resource.includeEventLabelKeys": tt.cmInEventLabelKeys,
					"resource.excludeEventLabelKeys": tt.cmExEventLabelKeys,
				},
			}

			proj := &argoappv1.AppProject{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "default",
					Namespace: test.FakeArgoCDNamespace,
					Labels:    tt.projLabels,
				},
			}

			var app argoappv1.Application
			app.Name = "test-app"
			app.Namespace = test.FakeArgoCDNamespace
			app.Labels = tt.appLabels
			appClientset := appclientset.NewSimpleClientset(proj)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
			informer := v1alpha1.NewAppProjectInformer(appClientset, test.FakeArgoCDNamespace, 0, indexers)
			go informer.Run(ctx.Done())
			cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

			kubeClient := fake.NewSimpleClientset(&cm)
			settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, test.FakeArgoCDNamespace)
			argoDB := db.NewDB("default", settingsMgr, kubeClient)

			eventLabels := GetAppEventLabels(&app, applisters.NewAppProjectLister(informer.GetIndexer()), test.FakeArgoCDNamespace, settingsMgr, argoDB, ctx)
			assert.Equal(t, len(tt.expectedEventLabels), len(eventLabels))
			for ek, ev := range tt.expectedEventLabels {
				v, found := eventLabels[ek]
				assert.True(t, found)
				assert.Equal(t, ev, v)
			}
		})
	}
}
