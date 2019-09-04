package controller

import (
	"context"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	mockstatecache "github.com/argoproj/argo-cd/controller/cache/mocks"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/reposerver/apiclient/mocks"
	mockreposerver "github.com/argoproj/argo-cd/reposerver/mocks"
	"github.com/argoproj/argo-cd/test"
	utilcache "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

type namespacedResource struct {
	argoappv1.ResourceNode
	AppName string
}

type fakeData struct {
	apps                []runtime.Object
	manifestResponse    *apiclient.ManifestResponse
	managedLiveObjs     map[kube.ResourceKey]*unstructured.Unstructured
	namespacedResources map[kube.ResourceKey]namespacedResource
	configMapData       map[string]string
}

func newFakeController(data *fakeData) *ApplicationController {
	var clust corev1.Secret
	err := yaml.Unmarshal([]byte(fakeCluster), &clust)
	if err != nil {
		panic(err)
	}

	// Mock out call to GenerateManifest
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}
	mockRepoClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(data.manifestResponse, nil)
	mockRepoClientset := mockreposerver.Clientset{}
	mockRepoClientset.On("NewRepoServerClient").Return(&fakeCloser{}, &mockRepoClient, nil)

	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: test.FakeArgoCDNamespace,
		},
		Data: map[string][]byte{
			"admin.password":   []byte("test"),
			"server.secretkey": []byte("test"),
		},
	}
	cm := corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: test.FakeArgoCDNamespace,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: data.configMapData,
	}
	kubeClient := fake.NewSimpleClientset(&clust, &cm, &secret)
	settingsMgr := settings.NewSettingsManager(context.Background(), kubeClient, test.FakeArgoCDNamespace)
	ctrl, err := NewApplicationController(
		test.FakeArgoCDNamespace,
		settingsMgr,
		kubeClient,
		appclientset.NewSimpleClientset(data.apps...),
		&mockRepoClientset,
		utilcache.NewCache(utilcache.NewInMemoryCache(1*time.Hour)),
		time.Minute,
		time.Minute,
		common.DefaultPortArgoCDMetrics,
	)
	if err != nil {
		panic(err)
	}
	cancelProj := test.StartInformer(ctrl.projInformer)
	defer cancelProj()
	cancelApp := test.StartInformer(ctrl.appInformer)
	defer cancelApp()
	mockStateCache := mockstatecache.LiveStateCache{}
	ctrl.appStateManager.(*appStateManager).liveStateCache = &mockStateCache
	ctrl.stateCache = &mockStateCache
	mockStateCache.On("IsNamespaced", mock.Anything, mock.Anything).Return(true, nil)
	mockStateCache.On("GetManagedLiveObjs", mock.Anything, mock.Anything).Return(data.managedLiveObjs, nil)
	response := make(map[kube.ResourceKey]argoappv1.ResourceNode)
	for k, v := range data.namespacedResources {
		response[k] = v.ResourceNode
	}
	mockStateCache.On("GetNamespaceTopLevelResources", mock.Anything, mock.Anything).Return(response, nil)
	mockStateCache.On("IterateHierarchy", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		key := args[1].(kube.ResourceKey)
		action := args[2].(func(child argoappv1.ResourceNode, appName string))
		appName := ""
		if res, ok := data.namespacedResources[key]; ok {
			appName = res.AppName
		}
		action(argoappv1.ResourceNode{ResourceRef: argoappv1.ResourceRef{Group: key.Group, Namespace: key.Namespace, Name: key.Name}}, appName)
	}).Return(nil)
	return ctrl
}

type fakeCloser struct{}

func (f *fakeCloser) Close() error { return nil }

var fakeCluster = `
apiVersion: v1
data:
  # {"bearerToken":"fake","tlsClientConfig":{"insecure":true},"awsAuthConfig":null}
  config: eyJiZWFyZXJUb2tlbiI6ImZha2UiLCJ0bHNDbGllbnRDb25maWciOnsiaW5zZWN1cmUiOnRydWV9LCJhd3NBdXRoQ29uZmlnIjpudWxsfQ==
  # minikube
  name: aHR0cHM6Ly9sb2NhbGhvc3Q6NjQ0Mw==
  # https://localhost:6443
  server: aHR0cHM6Ly9sb2NhbGhvc3Q6NjQ0Mw==
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
  name: some-secret
  namespace: ` + test.FakeArgoCDNamespace + `
type: Opaque
`

var fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  uid: "123"
  name: my-app
  namespace: ` + test.FakeArgoCDNamespace + `
spec:
  destination:
    namespace: ` + test.FakeDestNamespace + `
    server: https://localhost:6443
  project: default
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
  syncPolicy:
    automated: {}
status:
  operationState:
    finishedAt: 2018-09-21T23:50:29Z
    message: successfully synced
    operation:
      sync:
        revision: HEAD
    phase: Succeeded
    startedAt: 2018-09-21T23:50:25Z
    syncResult:
      resources:
      - kind: RoleBinding
        message: |-
          rolebinding.rbac.authorization.k8s.io/always-outofsync reconciled
          rolebinding.rbac.authorization.k8s.io/always-outofsync configured
        name: always-outofsync
        namespace: default
        status: Synced
      revision: aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      source:
        path: some/path
        repoURL: https://github.com/argoproj/argocd-example-apps.git
`

func newFakeApp() *argoappv1.Application {
	var app argoappv1.Application
	err := yaml.Unmarshal([]byte(fakeApp), &app)
	if err != nil {
		panic(err)
	}
	return &app
}

func TestAutoSync(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := argoappv1.SyncStatus{
		Status:   argoappv1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, app.Operation)
	assert.NotNil(t, app.Operation.Sync)
	assert.False(t, app.Operation.Sync.Prune)
}

func TestSkipAutoSync(t *testing.T) {
	// Verify we skip when we previously synced to it in our most recent history
	// Set current to 'aaaaa', desired to 'aaaa' and mark system OutOfSync
	{
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := argoappv1.SyncStatus{
			Status:   argoappv1.SyncStatusCodeOutOfSync,
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}
		cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	}

	// Verify we skip when we are already Synced (even if revision is different)
	{
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := argoappv1.SyncStatus{
			Status:   argoappv1.SyncStatusCodeSynced,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	}

	// Verify we skip when auto-sync is disabled
	{
		app := newFakeApp()
		app.Spec.SyncPolicy = nil
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := argoappv1.SyncStatus{
			Status:   argoappv1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	}

	// Verify we skip when application is marked for deletion
	{
		app := newFakeApp()
		now := metav1.Now()
		app.DeletionTimestamp = &now
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := argoappv1.SyncStatus{
			Status:   argoappv1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	}

	// Verify we skip when previous sync attempt failed and return error condition
	// Set current to 'aaaaa', desired to 'bbbbb' and add 'bbbbb' to failure history
	{
		app := newFakeApp()
		app.Status.OperationState = &argoappv1.OperationState{
			Operation: argoappv1.Operation{
				Sync: &argoappv1.SyncOperation{},
			},
			Phase: argoappv1.OperationFailed,
			SyncResult: &argoappv1.SyncOperationResult{
				Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Source:   *app.Spec.Source.DeepCopy(),
			},
		}
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := argoappv1.SyncStatus{
			Status:   argoappv1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
		assert.NotNil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	}
}

// TestAutoSyncIndicateError verifies we skip auto-sync and return error condition if previous sync failed
func TestAutoSyncIndicateError(t *testing.T) {
	app := newFakeApp()
	app.Spec.Source.Helm = &argoappv1.ApplicationSourceHelm{
		Parameters: []argoappv1.HelmParameter{
			{
				Name:  "a",
				Value: "1",
			},
		},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := argoappv1.SyncStatus{
		Status:   argoappv1.SyncStatusCodeOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	app.Status.OperationState = &argoappv1.OperationState{
		Operation: argoappv1.Operation{
			Sync: &argoappv1.SyncOperation{
				Source: app.Spec.Source.DeepCopy(),
			},
		},
		Phase: argoappv1.OperationFailed,
		SyncResult: &argoappv1.SyncOperationResult{
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Source:   *app.Spec.Source.DeepCopy(),
		},
	}
	cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
	assert.NotNil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)
}

// TestAutoSyncParameterOverrides verifies we auto-sync if revision is same but parameter overrides are different
func TestAutoSyncParameterOverrides(t *testing.T) {
	app := newFakeApp()
	app.Spec.Source.Helm = &argoappv1.ApplicationSourceHelm{
		Parameters: []argoappv1.HelmParameter{
			{
				Name:  "a",
				Value: "1",
			},
		},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := argoappv1.SyncStatus{
		Status:   argoappv1.SyncStatusCodeOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	app.Status.OperationState = &argoappv1.OperationState{
		Operation: argoappv1.Operation{
			Sync: &argoappv1.SyncOperation{
				Source: &argoappv1.ApplicationSource{
					Helm: &argoappv1.ApplicationSourceHelm{
						Parameters: []argoappv1.HelmParameter{
							{
								Name:  "a",
								Value: "2", // this value changed
							},
						},
					},
				},
			},
		},
		Phase: argoappv1.OperationFailed,
		SyncResult: &argoappv1.SyncOperationResult{
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	cond := ctrl.autoSync(app, &syncStatus, []argoappv1.ResourceStatus{})
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, app.Operation)
}

// TestFinalizeAppDeletion verifies application deletion
func TestFinalizeAppDeletion(t *testing.T) {
	app := newFakeApp()
	app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
	appObj := kube.MustToUnstructured(&app)
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
		kube.GetResourceKey(appObj): appObj,
	}})

	patched := false
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	defaultReactor := fakeAppCs.ReactionChain[0]
	fakeAppCs.ReactionChain = nil
	fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		return defaultReactor.React(action)
	})
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, nil, nil
	})
	err := ctrl.finalizeApplicationDeletion(app)
	assert.NoError(t, err)
	assert.True(t, patched)
}

// TestNormalizeApplication verifies we normalize an application during reconciliation
func TestNormalizeApplication(t *testing.T) {
	defaultProj := argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: argoappv1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []argoappv1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	}
	app := newFakeApp()
	app.Spec.Project = ""
	app.Spec.Source.Kustomize = &argoappv1.ApplicationSourceKustomize{NamePrefix: "foo-"}
	data := fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}

	{
		// Verify we normalize the app because project is missing
		ctrl := newFakeController(&data)
		key, _ := cache.MetaNamespaceKeyFunc(app)
		ctrl.appRefreshQueue.Add(key)
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		fakeAppCs.ReactionChain = nil
		normalized := false
		fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if patchAction, ok := action.(kubetesting.PatchAction); ok {
				if string(patchAction.GetPatch()) == `{"spec":{"project":"default"}}` {
					normalized = true
				}
			}
			return true, nil, nil
		})
		ctrl.processAppRefreshQueueItem()
		assert.True(t, normalized)
	}

	{
		// Verify we don't unnecessarily normalize app when project is set
		app.Spec.Project = "default"
		data.apps[0] = app
		ctrl := newFakeController(&data)
		key, _ := cache.MetaNamespaceKeyFunc(app)
		ctrl.appRefreshQueue.Add(key)
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		fakeAppCs.ReactionChain = nil
		normalized := false
		fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if patchAction, ok := action.(kubetesting.PatchAction); ok {
				if string(patchAction.GetPatch()) == `{"spec":{"project":"default"}}` {
					normalized = true
				}
			}
			return true, nil, nil
		})
		ctrl.processAppRefreshQueueItem()
		assert.False(t, normalized)
	}
}

func TestHandleAppUpdated(t *testing.T) {
	app := newFakeApp()
	app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
	app.Spec.Destination.Server = common.KubernetesInternalAPIServerAddr
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})

	ctrl.handleObjectUpdated(map[string]bool{app.Name: true}, kube.GetObjectRef(kube.MustToUnstructured(app)))
	isRequested, level := ctrl.isRefreshRequested(app.Name)
	assert.False(t, isRequested)
	assert.Equal(t, ComparisonWithNothing, level)

	ctrl.handleObjectUpdated(map[string]bool{app.Name: true}, corev1.ObjectReference{UID: "test", Kind: kube.DeploymentKind, Name: "test", Namespace: "default"})
	isRequested, level = ctrl.isRefreshRequested(app.Name)
	assert.True(t, isRequested)
	assert.Equal(t, CompareWithRecent, level)
}

func TestHandleOrphanedResourceUpdated(t *testing.T) {
	app1 := newFakeApp()
	app1.Name = "app1"
	app1.Spec.Destination.Namespace = test.FakeArgoCDNamespace
	app1.Spec.Destination.Server = common.KubernetesInternalAPIServerAddr

	app2 := newFakeApp()
	app2.Name = "app2"
	app2.Spec.Destination.Namespace = test.FakeArgoCDNamespace
	app2.Spec.Destination.Server = common.KubernetesInternalAPIServerAddr

	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &argoappv1.OrphanedResourcesMonitorSettings{}

	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app1, app2, proj}})

	ctrl.handleObjectUpdated(map[string]bool{}, corev1.ObjectReference{UID: "test", Kind: kube.DeploymentKind, Name: "test", Namespace: test.FakeArgoCDNamespace})

	isRequested, level := ctrl.isRefreshRequested(app1.Name)
	assert.True(t, isRequested)
	assert.Equal(t, ComparisonWithNothing, level)

	isRequested, level = ctrl.isRefreshRequested(app2.Name)
	assert.True(t, isRequested)
	assert.Equal(t, ComparisonWithNothing, level)
}

func TestSetOperationStateOnDeletedApp(t *testing.T) {
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	patched := false
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, nil, apierr.NewNotFound(schema.GroupResource{}, "my-app")
	})
	ctrl.setOperationState(newFakeApp(), &argoappv1.OperationState{Phase: argoappv1.OperationSucceeded})
	assert.True(t, patched)
}

func TestNeedRefreshAppStatus(t *testing.T) {
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})

	app := newFakeApp()
	now := metav1.Now()
	app.Status.ReconciledAt = &now
	app.Status.Sync = argoappv1.SyncStatus{
		Status: argoappv1.SyncStatusCodeSynced,
		ComparedTo: argoappv1.ComparedTo{
			Source:      app.Spec.Source,
			Destination: app.Spec.Destination,
		},
	}

	// no need to refresh just reconciled application
	needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour)
	assert.False(t, needRefresh)

	// refresh app using the 'deepest' requested comparison level
	ctrl.requestAppRefresh(app.Name, CompareWithRecent)
	ctrl.requestAppRefresh(app.Name, ComparisonWithNothing)

	needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour)
	assert.True(t, needRefresh)
	assert.Equal(t, argoappv1.RefreshTypeNormal, refreshType)
	assert.Equal(t, CompareWithRecent, compareWith)

	// refresh application which status is not reconciled using latest commit
	app.Status.Sync = argoappv1.SyncStatus{Status: argoappv1.SyncStatusCodeUnknown}

	needRefresh, refreshType, compareWith = ctrl.needRefreshAppStatus(app, 1*time.Hour)
	assert.True(t, needRefresh)
	assert.Equal(t, argoappv1.RefreshTypeNormal, refreshType)
	assert.Equal(t, CompareWithLatest, compareWith)

	// execute hard refresh if app has refresh annotation
	app.Annotations = map[string]string{
		common.AnnotationKeyRefresh: string(argoappv1.RefreshTypeHard),
	}
	needRefresh, refreshType, compareWith = ctrl.needRefreshAppStatus(app, 1*time.Hour)
	assert.True(t, needRefresh)
	assert.Equal(t, argoappv1.RefreshTypeHard, refreshType)
	assert.Equal(t, CompareWithLatest, compareWith)

}
