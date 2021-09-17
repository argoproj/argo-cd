package argo

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/test"
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
	assert.Nil(t, err)
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
	proj, err := GetAppProject(&testApp.Spec, applisters.NewAppProjectLister(informer.GetIndexer()), namespace, settingsMgr)
	assert.Nil(t, err)
	assert.Equal(t, proj.Name, projName)
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
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NamePrefix: "foo"}}})
		assert.NotNil(t, spec.Source.Kustomize)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NamePrefix: ""}}})
		assert.Nil(t, spec.Source.Kustomize)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NameSuffix: "foo"}}})
		assert.NotNil(t, spec.Source.Kustomize)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Kustomize: &argoappv1.ApplicationSourceKustomize{NameSuffix: ""}}})
		assert.Nil(t, spec.Source.Kustomize)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{"values.yaml"}}}})
		assert.NotNil(t, spec.Source.Helm)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Helm: &argoappv1.ApplicationSourceHelm{ValueFiles: []string{}}}})
		assert.Nil(t, spec.Source.Helm)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Ksonnet: &argoappv1.ApplicationSourceKsonnet{Environment: "foo"}}})
		assert.NotNil(t, spec.Source.Ksonnet)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Ksonnet: &argoappv1.ApplicationSourceKsonnet{Environment: ""}}})
		assert.Nil(t, spec.Source.Ksonnet)
	}
	{
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Directory: &argoappv1.ApplicationSourceDirectory{Recurse: true}}})
		assert.NotNil(t, spec.Source.Directory)
		spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{Source: argoappv1.ApplicationSource{Directory: &argoappv1.ApplicationSourceDirectory{Recurse: false}}})
		assert.Nil(t, spec.Source.Directory)
	}
}

func TestValidatePermissionsEmptyDestination(t *testing.T) {
	conditions, err := ValidatePermissions(context.Background(), &argoappv1.ApplicationSpec{
		Source: argoappv1.ApplicationSource{RepoURL: "https://github.com/argoproj/argo-cd", Path: "."},
	}, &argoappv1.AppProject{
		Spec: argoappv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []argoappv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}, nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, conditions, []argoappv1.ApplicationCondition{{Type: argoappv1.ApplicationConditionInvalidSpecError, Message: "Destination server missing from app spec"}})
}

func TestValidateChartWithoutRevision(t *testing.T) {
	conditions, err := ValidatePermissions(context.Background(), &argoappv1.ApplicationSpec{
		Source: argoappv1.ApplicationSource{RepoURL: "https://charts.helm.sh/incubator/", Chart: "myChart", TargetRevision: ""},
		Destination: argoappv1.ApplicationDestination{
			Server: "https://kubernetes.default.svc", Namespace: "default",
		},
	}, &argoappv1.AppProject{
		Spec: argoappv1.AppProjectSpec{
			SourceRepos:  []string{"*"},
			Destinations: []argoappv1.ApplicationDestination{{Server: "*", Namespace: "*"}},
		},
	}, nil)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(conditions))
	assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
	assert.Equal(t, "spec.source.targetRevision is required if the manifest source is a helm chart", conditions[0].Message)
}

func Test_enrichSpec(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		spec := &argoappv1.ApplicationSpec{}
		enrichSpec(spec, &apiclient.RepoAppDetailsResponse{})
		assert.Empty(t, spec.Destination.Server)
		assert.Empty(t, spec.Destination.Namespace)
	})
	t.Run("Ksonnet", func(t *testing.T) {
		spec := &argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				Ksonnet: &argoappv1.ApplicationSourceKsonnet{
					Environment: "qa",
				},
			},
		}
		response := &apiclient.RepoAppDetailsResponse{
			Ksonnet: &apiclient.KsonnetAppSpec{
				Environments: map[string]*apiclient.KsonnetEnvironment{
					"prod": {
						Destination: &apiclient.KsonnetEnvironmentDestination{
							Server:    "my-server",
							Namespace: "my-namespace",
						},
					},
				},
			},
		}
		enrichSpec(spec, response)
		assert.Empty(t, spec.Destination.Server)
		assert.Empty(t, spec.Destination.Namespace)

		spec.Source.Ksonnet.Environment = "prod"
		enrichSpec(spec, response)
		assert.Equal(t, "my-server", spec.Destination.Server)
		assert.Equal(t, "my-namespace", spec.Destination.Namespace)
	})
}

func TestAPIGroupsToVersions(t *testing.T) {
	versions := APIGroupsToVersions([]metav1.APIGroup{{
		Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "apps/v1beta1"}, {GroupVersion: "apps/v1beta2"}},
	}, {
		Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "extensions/v1beta1"}},
	}})

	assert.EqualValues(t, []string{"apps/v1beta1", "apps/v1beta2", "extensions/v1beta1"}, versions)
}

func TestValidateRepo(t *testing.T) {
	repoPath, err := filepath.Abs("./../..")
	assert.NoError(t, err)

	apiGroups := []metav1.APIGroup{{Versions: []metav1.GroupVersionForDiscovery{{GroupVersion: "apps/v1beta1"}, {GroupVersion: "apps/v1beta2"}}}}
	kubeVersion := "v1.16"
	kustomizeOptions := &argoappv1.KustomizeOptions{BuildOptions: "sample options"}
	repo := &argoappv1.Repository{Repo: fmt.Sprintf("file://%s", repoPath)}
	cluster := &argoappv1.Cluster{Server: "sample server"}
	app := &argoappv1.Application{
		Spec: argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
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
	repoClient.On("GetAppDetails", context.Background(), &apiclient.RepoServerAppDetailsQuery{
		Repo:             repo,
		Source:           &app.Spec.Source,
		Repos:            helmRepos,
		KustomizeOptions: kustomizeOptions,
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

	db.On("GetRepository", context.Background(), app.Spec.Source.RepoURL).Return(repo, nil)
	db.On("ListHelmRepositories", context.Background()).Return(helmRepos, nil)
	db.On("GetCluster", context.Background(), app.Spec.Destination.Server).Return(cluster, nil)
	db.On("GetAllHelmRepositoryCredentials", context.Background()).Return(nil, nil)

	var receivedRequest *apiclient.ManifestRequest

	repoClient.On("GenerateManifest", context.Background(), mock.MatchedBy(func(req *apiclient.ManifestRequest) bool {
		receivedRequest = req
		return true
	})).Return(nil, nil)

	conditions, err := ValidateRepo(context.Background(), app, repoClientSet, db, kustomizeOptions, nil, &kubetest.MockKubectlCmd{Version: kubeVersion, APIGroups: apiGroups}, proj)

	assert.NoError(t, err)
	assert.Empty(t, conditions)
	assert.ElementsMatch(t, []string{"apps/v1beta1", "apps/v1beta2"}, receivedRequest.ApiVersions)
	assert.Equal(t, kubeVersion, receivedRequest.KubeVersion)
	assert.Equal(t, app.Spec.Destination.Namespace, receivedRequest.Namespace)
	assert.Equal(t, &app.Spec.Source, receivedRequest.ApplicationSource)
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

func TestFilterByRepo(t *testing.T) {
	apps := []argoappv1.Application{
		{
			Spec: argoappv1.ApplicationSpec{
				Source: argoappv1.ApplicationSource{
					RepoURL: "git@github.com:owner/repo.git",
				},
			},
		},
		{
			Spec: argoappv1.ApplicationSpec{
				Source: argoappv1.ApplicationSource{
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
		assert.Len(t, res, 0)
	})
}

func TestValidatePermissions(t *testing.T) {
	t.Run("Empty Repo URL result in condition", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				RepoURL: "",
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		assert.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "are required")
	})

	t.Run("Incomplete Path/Chart combo result in condition", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				RepoURL: "http://some/where",
				Path:    "",
				Chart:   "",
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		assert.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "are required")
	})

	t.Run("Helm chart requires targetRevision", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
				RepoURL: "http://some/where",
				Path:    "",
				Chart:   "somechart",
			},
		}
		proj := argoappv1.AppProject{}
		db := &dbmocks.ArgoDB{}
		conditions, err := ValidatePermissions(context.Background(), &spec, &proj, db)
		assert.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Equal(t, argoappv1.ApplicationConditionInvalidSpecError, conditions[0].Type)
		assert.Contains(t, conditions[0].Message, "is required if the manifest source is a helm chart")
	})

	t.Run("Application source is not permitted in project", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
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
		assert.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "application repo http://some/where is not permitted")
	})

	t.Run("Application destination is not permitted in project", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
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
		assert.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "application destination")
	})

	t.Run("Destination cluster does not exist", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
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
		assert.NoError(t, err)
		assert.Len(t, conditions, 1)
		assert.Contains(t, conditions[0].Message, "has not been configured")
	})

	t.Run("Cannot get cluster info from DB", func(t *testing.T) {
		spec := argoappv1.ApplicationSpec{
			Source: argoappv1.ApplicationSource{
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
		assert.Error(t, err)
	})
}

func TestSetAppOperations(t *testing.T) {
	t.Run("Application not existing", func(t *testing.T) {
		appIf := appclientset.NewSimpleClientset().ArgoprojV1alpha1().Applications("default")
		app, err := SetAppOperation(appIf, "someapp", &argoappv1.Operation{Sync: &argoappv1.SyncOperation{Revision: "aaa"}})
		assert.Error(t, err)
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
		assert.Error(t, err)
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
		assert.Error(t, err)
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
		assert.NoError(t, err)
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
		assert.Nil(t, appCond)
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Validate destination with server name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		db := &dbmocks.ArgoDB{}
		db.On("ListClusters", context.Background()).Return(&argoappv1.ClusterList{
			Items: []argoappv1.Cluster{
				{
					Name:   "minikube",
					Server: "https://127.0.0.1:6443",
				},
			},
		}, nil)

		appCond := ValidateDestination(context.Background(), &dest, db)
		assert.Nil(t, appCond)
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

	t.Run("List clusters fails", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		db := &dbmocks.ArgoDB{}
		db.On("ListClusters", context.Background()).Return(nil, fmt.Errorf("an error occurred"))

		err := ValidateDestination(context.Background(), &dest, db)
		assert.Equal(t, "unable to find destination server: an error occurred", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Destination cluster does not exist", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		db := &dbmocks.ArgoDB{}
		db.On("ListClusters", context.Background()).Return(&argoappv1.ClusterList{
			Items: []argoappv1.Cluster{
				{
					Name:   "dind",
					Server: "https://127.0.0.1:6443",
				},
			},
		}, nil)

		err := ValidateDestination(context.Background(), &dest, db)
		assert.Equal(t, "unable to find destination server: there are no clusters with this name: minikube", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Validate too many clusters with the same name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "dind",
		}

		db := &dbmocks.ArgoDB{}
		db.On("ListClusters", context.Background()).Return(&argoappv1.ClusterList{
			Items: []argoappv1.Cluster{
				{
					Name:   "dind",
					Server: "https://127.0.0.1:2443",
				},
				{
					Name:   "dind",
					Server: "https://127.0.0.1:8443",
				},
			},
		}, nil)

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
		assert.NoError(t, err)
		assert.Len(t, res, 2)
	})

	t.Run("Single app by name", func(t *testing.T) {
		res, err := FilterByName(apps, "foo")
		assert.NoError(t, err)
		assert.Len(t, res, 1)
	})

	t.Run("No such app", func(t *testing.T) {
		res, err := FilterByName(apps, "foobar")
		assert.Error(t, err)
		assert.Len(t, res, 0)
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
		assert.Equal(t, xGlobalProjects[0].Name, "default-x")

		nonXGlobalProjects := GetGlobalProjects(isNoX, projLister, settingsMgr)
		assert.Len(t, nonXGlobalProjects, 1)
		assert.Equal(t, nonXGlobalProjects[0].Name, "default-non-x")
	})
}
