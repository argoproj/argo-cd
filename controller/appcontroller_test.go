package controller

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/resource"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"

	"github.com/argoproj/argo-cd/v2/common"
	statecache "github.com/argoproj/argo-cd/v2/controller/cache"

	"github.com/argoproj/gitops-engine/pkg/cache/mocks"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
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
	"sigs.k8s.io/yaml"

	mockstatecache "github.com/argoproj/argo-cd/v2/controller/cache/mocks"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/test"
	cacheutil "github.com/argoproj/argo-cd/v2/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/settings"
)

type namespacedResource struct {
	v1alpha1.ResourceNode
	AppName string
}

type fakeData struct {
	apps                   []runtime.Object
	manifestResponse       *apiclient.ManifestResponse
	manifestResponses      []*apiclient.ManifestResponse
	managedLiveObjs        map[kube.ResourceKey]*unstructured.Unstructured
	namespacedResources    map[kube.ResourceKey]namespacedResource
	configMapData          map[string]string
	metricsCacheExpiration time.Duration
	applicationNamespaces  []string
}

func newFakeController(data *fakeData) *ApplicationController {
	var clust corev1.Secret
	err := yaml.Unmarshal([]byte(fakeCluster), &clust)
	if err != nil {
		panic(err)
	}

	// Mock out call to GenerateManifest
	mockRepoClient := mockrepoclient.RepoServerServiceClient{}

	if len(data.manifestResponses) > 0 {
		for _, response := range data.manifestResponses {
			mockRepoClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(response, nil).Once()
		}
	} else {
		mockRepoClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(data.manifestResponse, nil)
	}

	mockRepoClientset := mockrepoclient.Clientset{RepoServerServiceClient: &mockRepoClient}

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
	kubectl := &kubetest.MockKubectlCmd{}
	ctrl, err := NewApplicationController(
		test.FakeArgoCDNamespace,
		settingsMgr,
		kubeClient,
		appclientset.NewSimpleClientset(data.apps...),
		&mockRepoClientset,
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Minute)),
			1*time.Minute,
		),
		kubectl,
		time.Minute,
		time.Hour,
		time.Minute,
		common.DefaultPortArgoCDMetrics,
		data.metricsCacheExpiration,
		[]string{},
		0,
		true,
		nil,
		data.applicationNamespaces,
	)
	if err != nil {
		panic(err)
	}
	cancelProj := test.StartInformer(ctrl.projInformer)
	defer cancelProj()
	cancelApp := test.StartInformer(ctrl.appInformer)
	defer cancelApp()
	clusterCacheMock := mocks.ClusterCache{}
	clusterCacheMock.On("IsNamespaced", mock.Anything).Return(true, nil)
	clusterCacheMock.On("GetOpenAPISchema").Return(nil, nil)
	clusterCacheMock.On("GetGVKParser").Return(nil)

	mockStateCache := mockstatecache.LiveStateCache{}
	ctrl.appStateManager.(*appStateManager).liveStateCache = &mockStateCache
	ctrl.stateCache = &mockStateCache
	mockStateCache.On("IsNamespaced", mock.Anything, mock.Anything).Return(true, nil)
	mockStateCache.On("GetManagedLiveObjs", mock.Anything, mock.Anything).Return(data.managedLiveObjs, nil)
	mockStateCache.On("GetVersionsInfo", mock.Anything).Return("v1.2.3", nil, nil)
	response := make(map[kube.ResourceKey]v1alpha1.ResourceNode)
	for k, v := range data.namespacedResources {
		response[k] = v.ResourceNode
	}
	mockStateCache.On("GetNamespaceTopLevelResources", mock.Anything, mock.Anything).Return(response, nil)
	mockStateCache.On("IterateResources", mock.Anything, mock.Anything).Return(nil)
	mockStateCache.On("GetClusterCache", mock.Anything).Return(&clusterCacheMock, nil)
	mockStateCache.On("IterateHierarchy", mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		key := args[1].(kube.ResourceKey)
		action := args[2].(func(child v1alpha1.ResourceNode, appName string) bool)
		appName := ""
		if res, ok := data.namespacedResources[key]; ok {
			appName = res.AppName
		}
		_ = action(v1alpha1.ResourceNode{ResourceRef: v1alpha1.ResourceRef{Kind: key.Kind, Group: key.Group, Namespace: key.Namespace, Name: key.Name}}, appName)
	}).Return(nil)
	return ctrl
}

var fakeCluster = `
apiVersion: v1
data:
  # {"bearerToken":"fake","tlsClientConfig":{"insecure":true},"awsAuthConfig":null}
  config: eyJiZWFyZXJUb2tlbiI6ImZha2UiLCJ0bHNDbGllbnRDb25maWciOnsiaW5zZWN1cmUiOnRydWV9LCJhd3NBdXRoQ29uZmlnIjpudWxsfQ==
  # minikube
  name: bWluaWt1YmU=
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

var fakeMultiSourceApp = `
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
  sources:
  - path: some/path
    helm:
      valueFiles:
      - $values_test/values.yaml
    repoURL: https://github.com/argoproj/argocd-example-apps.git
  - path: some/other/path
    repoURL: https://github.com/argoproj/argocd-example-apps-fake.git
  - ref: values_test
    repoURL: https://github.com/argoproj/argocd-example-apps-fake-ref.git
  syncPolicy:
    automated: {}
status:
  operationState:
    finishedAt: 2018-09-21T23:50:29Z
    message: successfully synced
    operation:
      sync:
        revisions:
        - HEAD
        - HEAD
        - HEAD
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
      revisions:
      - aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
      - bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
      - cccccccccccccccccccccccccccccccccccccccc
      sources:
      - path: some/path
        repoURL: https://github.com/argoproj/argocd-example-apps.git
      - path: some/other/path
        repoURL: https://github.com/argoproj/argocd-example-apps-fake.git
      - path: some/other/path
        repoURL: https://github.com/argoproj/argocd-example-apps-fake-ref.git
`

var fakeAppWithDestName = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  uid: "123"
  name: my-app
  namespace: ` + test.FakeArgoCDNamespace + `
spec:
  destination:
    namespace: ` + test.FakeDestNamespace + `
    name: minikube
  project: default
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
  syncPolicy:
    automated: {}
`

var fakeAppWithDestMismatch = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  uid: "123"
  name: my-app
  namespace: ` + test.FakeArgoCDNamespace + `
spec:
  destination:
    namespace: ` + test.FakeDestNamespace + `
    name: another-cluster
    server: https://localhost:6443
  project: default
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
  syncPolicy:
    automated: {}
`

var fakeStrayResource = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
  namespace: invalid
  labels:
    app.kubernetes.io/instance: my-app
data:
`

func newFakeApp() *v1alpha1.Application {
	return createFakeApp(fakeApp)
}

func newFakeMultiSourceApp() *v1alpha1.Application {
	return createFakeApp(fakeMultiSourceApp)
}

func newFakeAppWithDestMismatch() *v1alpha1.Application {
	return createFakeApp(fakeAppWithDestMismatch)
}

func newFakeAppWithDestName() *v1alpha1.Application {
	return createFakeApp(fakeAppWithDestName)
}

func createFakeApp(testApp string) *v1alpha1.Application {
	var app v1alpha1.Application
	err := yaml.Unmarshal([]byte(testApp), &app)
	if err != nil {
		panic(err)
	}
	return &app
}

func newFakeCM() map[string]interface{} {
	var cm map[string]interface{}
	err := yaml.Unmarshal([]byte(fakeStrayResource), &cm)
	if err != nil {
		panic(err)
	}
	return cm
}

func TestAutoSync(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}})
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, app.Operation)
	assert.NotNil(t, app.Operation.Sync)
	assert.False(t, app.Operation.Sync.Prune)
}

func TestAutoSyncNotAllowEmpty(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy.Automated.Prune = true
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{})
	assert.NotNil(t, cond)
}

func TestAutoSyncAllowEmpty(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy.Automated.Prune = true
	app.Spec.SyncPolicy.Automated.AllowEmpty = true
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{})
	assert.Nil(t, cond)
}

func TestSkipAutoSync(t *testing.T) {
	// Verify we skip when we previously synced to it in our most recent history
	// Set current to 'aaaaa', desired to 'aaaa' and mark system OutOfSync
	t.Run("PreviouslySyncedToRevision", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when we are already Synced (even if revision is different)
	t.Run("AlreadyInSyncedState", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeSynced,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when auto-sync is disabled
	t.Run("AutoSyncIsDisabled", func(t *testing.T) {
		app := newFakeApp()
		app.Spec.SyncPolicy = nil
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when application is marked for deletion
	t.Run("ApplicationIsMarkedForDeletion", func(t *testing.T) {
		app := newFakeApp()
		now := metav1.Now()
		app.DeletionTimestamp = &now
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when previous sync attempt failed and return error condition
	// Set current to 'aaaaa', desired to 'bbbbb' and add 'bbbbb' to failure history
	t.Run("PreviousSyncAttemptFailed", func(t *testing.T) {
		app := newFakeApp()
		app.Status.OperationState = &v1alpha1.OperationState{
			Operation: v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{},
			},
			Phase: synccommon.OperationFailed,
			SyncResult: &v1alpha1.SyncOperationResult{
				Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Source:   *app.Spec.Source.DeepCopy(),
			},
		}
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}})
		assert.NotNil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	t.Run("NeedsToPruneResourcesOnlyButAutomatedPruneDisabled", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{
			{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync, RequiresPruning: true},
		})
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Nil(t, app.Operation)
	})
}

// TestAutoSyncIndicateError verifies we skip auto-sync and return error condition if previous sync failed
func TestAutoSyncIndicateError(t *testing.T) {
	app := newFakeApp()
	app.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Parameters: []v1alpha1.HelmParameter{
			{
				Name:  "a",
				Value: "1",
			},
		},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	app.Status.OperationState = &v1alpha1.OperationState{
		Operation: v1alpha1.Operation{
			Sync: &v1alpha1.SyncOperation{
				Source: app.Spec.Source.DeepCopy(),
			},
		},
		Phase: synccommon.OperationFailed,
		SyncResult: &v1alpha1.SyncOperationResult{
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Source:   *app.Spec.Source.DeepCopy(),
		},
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}})
	assert.NotNil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)
}

// TestAutoSyncParameterOverrides verifies we auto-sync if revision is same but parameter overrides are different
func TestAutoSyncParameterOverrides(t *testing.T) {
	app := newFakeApp()
	app.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
		Parameters: []v1alpha1.HelmParameter{
			{
				Name:  "a",
				Value: "1",
			},
		},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	app.Status.OperationState = &v1alpha1.OperationState{
		Operation: v1alpha1.Operation{
			Sync: &v1alpha1.SyncOperation{
				Source: &v1alpha1.ApplicationSource{
					Helm: &v1alpha1.ApplicationSourceHelm{
						Parameters: []v1alpha1.HelmParameter{
							{
								Name:  "a",
								Value: "2", // this value changed
							},
						},
					},
				},
			},
		},
		Phase: synccommon.OperationFailed,
		SyncResult: &v1alpha1.SyncOperationResult{
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}})
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, app.Operation)
}

// TestFinalizeAppDeletion verifies application deletion
func TestFinalizeAppDeletion(t *testing.T) {
	defaultProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	}

	// Ensure app can be deleted cascading
	t.Run("CascadingDelete", func(t *testing.T) {
		app := newFakeApp()
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		appObj := kube.MustToUnstructured(&app)
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &defaultProj}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
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
		_, err := ctrl.finalizeApplicationDeletion(app, func(project string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		assert.NoError(t, err)
		assert.True(t, patched)
	})

	// Ensure any stray resources irregularly labeled with instance label of app are not deleted upon deleting,
	// when app project restriction is in place
	t.Run("ProjectRestrictionEnforced", func(*testing.T) {
		restrictedProj := v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "restricted",
				Namespace: test.FakeArgoCDNamespace,
			},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos: []string{"*"},
				Destinations: []v1alpha1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "my-app",
					},
				},
			},
		}
		app := newFakeApp()
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		app.Spec.Project = "restricted"
		appObj := kube.MustToUnstructured(&app)
		cm := newFakeCM()
		strayObj := kube.MustToUnstructured(&cm)
		ctrl := newFakeController(&fakeData{
			apps: []runtime.Object{app, &defaultProj, &restrictedProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(appObj):   appObj,
				kube.GetResourceKey(strayObj): strayObj,
			},
		})

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
		objs, err := ctrl.finalizeApplicationDeletion(app, func(project string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		assert.NoError(t, err)
		assert.True(t, patched)
		objsMap, err := ctrl.stateCache.GetManagedLiveObjs(app, []*unstructured.Unstructured{})
		if err != nil {
			assert.NoError(t, err)
		}
		// Managed objects must be empty
		assert.Empty(t, objsMap)
		// Loop through all deleted objects, ensure that test-cm is none of them
		for _, o := range objs {
			assert.NotEqual(t, "test-cm", o.GetName())
		}
	})

	t.Run("DeleteWithDestinationClusterName", func(t *testing.T) {
		app := newFakeAppWithDestName()
		appObj := kube.MustToUnstructured(&app)
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &defaultProj}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
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
		_, err := ctrl.finalizeApplicationDeletion(app, func(project string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		assert.NoError(t, err)
		assert.True(t, patched)
	})

	// Create an Application with a cluster that doesn't exist
	// Ensure it can be deleted.
	t.Run("DeleteWithInvalidClusterName", func(t *testing.T) {

		appTemplate := newFakeAppWithDestName()

		testShouldDelete := func(app *v1alpha1.Application) {
			appObj := kube.MustToUnstructured(&app)
			ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &defaultProj}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(appObj): appObj,
			}})

			fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
			defaultReactor := fakeAppCs.ReactionChain[0]
			fakeAppCs.ReactionChain = nil
			fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				return defaultReactor.React(action)
			})
			_, err := ctrl.finalizeApplicationDeletion(app, func(project string) ([]*v1alpha1.Cluster, error) {
				return []*v1alpha1.Cluster{}, nil
			})
			assert.NoError(t, err)
		}

		app1 := appTemplate.DeepCopy()
		app1.Spec.Destination.Server = "https://invalid"
		testShouldDelete(app1)

		app2 := appTemplate.DeepCopy()
		app2.Spec.Destination.Name = "invalid"
		testShouldDelete(app2)

		app3 := appTemplate.DeepCopy()
		app3.Spec.Destination.Name = "invalid"
		app3.Spec.Destination.Server = "https://invalid"
		testShouldDelete(app3)

	})

}

// TestNormalizeApplication verifies we normalize an application during reconciliation
func TestNormalizeApplication(t *testing.T) {
	defaultProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	}
	app := newFakeApp()
	app.Spec.Project = ""
	app.Spec.Source.Kustomize = &v1alpha1.ApplicationSourceKustomize{NamePrefix: "foo-"}
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
	app.Spec.Destination.Server = v1alpha1.KubernetesInternalAPIServerAddr
	proj := defaultProj.DeepCopy()
	proj.Spec.SourceNamespaces = []string{test.FakeArgoCDNamespace}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, proj}})

	ctrl.handleObjectUpdated(map[string]bool{app.InstanceName(ctrl.namespace): true}, kube.GetObjectRef(kube.MustToUnstructured(app)))
	isRequested, level := ctrl.isRefreshRequested(app.QualifiedName())
	assert.False(t, isRequested)
	assert.Equal(t, ComparisonWithNothing, level)

	ctrl.handleObjectUpdated(map[string]bool{app.InstanceName(ctrl.namespace): true}, corev1.ObjectReference{UID: "test", Kind: kube.DeploymentKind, Name: "test", Namespace: "default"})
	isRequested, level = ctrl.isRefreshRequested(app.QualifiedName())
	assert.True(t, isRequested)
	assert.Equal(t, CompareWithRecent, level)
}

func TestHandleOrphanedResourceUpdated(t *testing.T) {
	app1 := newFakeApp()
	app1.Name = "app1"
	app1.Spec.Destination.Namespace = test.FakeArgoCDNamespace
	app1.Spec.Destination.Server = v1alpha1.KubernetesInternalAPIServerAddr

	app2 := newFakeApp()
	app2.Name = "app2"
	app2.Spec.Destination.Namespace = test.FakeArgoCDNamespace
	app2.Spec.Destination.Server = v1alpha1.KubernetesInternalAPIServerAddr

	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &v1alpha1.OrphanedResourcesMonitorSettings{}

	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app1, app2, proj}})

	ctrl.handleObjectUpdated(map[string]bool{}, corev1.ObjectReference{UID: "test", Kind: kube.DeploymentKind, Name: "test", Namespace: test.FakeArgoCDNamespace})

	isRequested, level := ctrl.isRefreshRequested(app1.QualifiedName())
	assert.True(t, isRequested)
	assert.Equal(t, CompareWithRecent, level)

	isRequested, level = ctrl.isRefreshRequested(app2.QualifiedName())
	assert.True(t, isRequested)
	assert.Equal(t, CompareWithRecent, level)
}

func TestGetResourceTree_HasOrphanedResources(t *testing.T) {
	app := newFakeApp()
	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &v1alpha1.OrphanedResourcesMonitorSettings{}

	managedDeploy := v1alpha1.ResourceNode{
		ResourceRef: v1alpha1.ResourceRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "nginx-deployment", Version: "v1"},
	}
	orphanedDeploy1 := v1alpha1.ResourceNode{
		ResourceRef: v1alpha1.ResourceRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "deploy1"},
	}
	orphanedDeploy2 := v1alpha1.ResourceNode{
		ResourceRef: v1alpha1.ResourceRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "deploy2"},
	}

	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, proj},
		namespacedResources: map[kube.ResourceKey]namespacedResource{
			kube.NewResourceKey("apps", "Deployment", "default", "nginx-deployment"): {ResourceNode: managedDeploy},
			kube.NewResourceKey("apps", "Deployment", "default", "deploy1"):          {ResourceNode: orphanedDeploy1},
			kube.NewResourceKey("apps", "Deployment", "default", "deploy2"):          {ResourceNode: orphanedDeploy2},
		},
	})
	tree, err := ctrl.getResourceTree(app, []*v1alpha1.ResourceDiff{{
		Namespace:   "default",
		Name:        "nginx-deployment",
		Kind:        "Deployment",
		Group:       "apps",
		LiveState:   "null",
		TargetState: test.DeploymentManifest,
	}})

	assert.NoError(t, err)
	assert.Equal(t, tree.Nodes, []v1alpha1.ResourceNode{managedDeploy})
	assert.Equal(t, tree.OrphanedNodes, []v1alpha1.ResourceNode{orphanedDeploy1, orphanedDeploy2})
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
	ctrl.setOperationState(newFakeApp(), &v1alpha1.OperationState{Phase: synccommon.OperationSucceeded})
	assert.True(t, patched)
}

type logHook struct {
	entries []logrus.Entry
}

func (h *logHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.WarnLevel}
}

func (h *logHook) Fire(entry *logrus.Entry) error {
	h.entries = append(h.entries, *entry)
	return nil
}

func TestSetOperationStateLogRetries(t *testing.T) {
	hook := logHook{}
	logrus.AddHook(&hook)
	t.Cleanup(func() {
		logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})
	})
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	patched := false
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if !patched {
			patched = true
			return true, nil, errors.New("fake error")
		}
		return true, nil, nil
	})
	ctrl.setOperationState(newFakeApp(), &v1alpha1.OperationState{Phase: synccommon.OperationSucceeded})
	assert.True(t, patched)
	assert.Contains(t, hook.entries[0].Message, "fake error")
}

func TestNeedRefreshAppStatus(t *testing.T) {
	testCases := []struct {
		name string
		app  *v1alpha1.Application
	}{
		{
			name: "single-source app",
			app:  newFakeApp(),
		},
		{
			name: "multi-source app",
			app:  newFakeMultiSourceApp(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			app := tc.app
			now := metav1.Now()
			app.Status.ReconciledAt = &now

			app.Status.Sync = v1alpha1.SyncStatus{
				Status: v1alpha1.SyncStatusCodeSynced,
				ComparedTo: v1alpha1.ComparedTo{
					Destination:       app.Spec.Destination,
					IgnoreDifferences: app.Spec.IgnoreDifferences,
				},
			}

			if app.Spec.HasMultipleSources() {
				app.Status.Sync.ComparedTo.Sources = app.Spec.Sources
			} else {
				app.Status.Sync.ComparedTo.Source = app.Spec.GetSource()
			}

			ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})

			t.Run("no need to refresh just reconciled application", func(t *testing.T) {
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)
			})

			t.Run("requested refresh is respected", func(t *testing.T) {
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)

				// use a one-off controller so other tests don't have a manual refresh request
				ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})

				// refresh app using the 'deepest' requested comparison level
				ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)
				ctrl.requestAppRefresh(app.Name, ComparisonWithNothing.Pointer(), nil)

				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
				assert.Equal(t, CompareWithRecent, compareWith)
			})

			t.Run("refresh application which status is not reconciled using latest commit", func(t *testing.T) {
				app := app.DeepCopy()
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)
				app.Status.Sync = v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeUnknown}

				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
				assert.Equal(t, CompareWithLatestForceResolve, compareWith)
			})

			t.Run("refresh app using the 'latest' level if comparison expired", func(t *testing.T) {
				app := app.DeepCopy()

				// use a one-off controller so other tests don't have a manual refresh request
				ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})

				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)

				ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)
				reconciledAt := metav1.NewTime(time.Now().UTC().Add(-1 * time.Hour))
				app.Status.ReconciledAt = &reconciledAt
				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Minute, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
				assert.Equal(t, CompareWithLatest, compareWith)
			})

			t.Run("refresh app using the 'latest' level if comparison expired for hard refresh", func(t *testing.T) {
				app := app.DeepCopy()
				app.Status.Sync = v1alpha1.SyncStatus{
					Status: v1alpha1.SyncStatusCodeSynced,
					ComparedTo: v1alpha1.ComparedTo{
						Destination:       app.Spec.Destination,
						IgnoreDifferences: app.Spec.IgnoreDifferences,
					},
				}
				if app.Spec.HasMultipleSources() {
					app.Status.Sync.ComparedTo.Sources = app.Spec.Sources
				} else {
					app.Status.Sync.ComparedTo.Source = app.Spec.GetSource()
				}

				// use a one-off controller so other tests don't have a manual refresh request
				ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})

				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)
				ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)
				reconciledAt := metav1.NewTime(time.Now().UTC().Add(-1 * time.Hour))
				app.Status.ReconciledAt = &reconciledAt
				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 2*time.Hour, 1*time.Minute)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeHard, refreshType)
				assert.Equal(t, CompareWithLatest, compareWith)
			})

			t.Run("execute hard refresh if app has refresh annotation", func(t *testing.T) {
				app := app.DeepCopy()
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)
				reconciledAt := metav1.NewTime(time.Now().UTC().Add(-1 * time.Hour))
				app.Status.ReconciledAt = &reconciledAt
				app.Annotations = map[string]string{
					v1alpha1.AnnotationKeyRefresh: string(v1alpha1.RefreshTypeHard),
				}
				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeHard, refreshType)
				assert.Equal(t, CompareWithLatestForceResolve, compareWith)
			})

			t.Run("ensure that CompareWithLatest level is used if application source has changed", func(t *testing.T) {
				app := app.DeepCopy()
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)
				// sample app source change
				if app.Spec.HasMultipleSources() {
					app.Spec.Sources[0].Helm = &v1alpha1.ApplicationSourceHelm{
						Parameters: []v1alpha1.HelmParameter{{
							Name:  "foo",
							Value: "bar",
						}},
					}
				} else {
					app.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
						Parameters: []v1alpha1.HelmParameter{{
							Name:  "foo",
							Value: "bar",
						}},
					}
				}

				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
				assert.Equal(t, CompareWithLatestForceResolve, compareWith)
			})

			t.Run("ensure that CompareWithLatest level is used if ignored differences change", func(t *testing.T) {
				app := app.DeepCopy()
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)

				app.Spec.IgnoreDifferences = []v1alpha1.ResourceIgnoreDifferences{
					{
						Group: "apps",
						Kind:  "Deployment",
						JSONPointers: []string{
							"/spec/template/spec/containers/0/image",
						},
					},
				}

				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
				assert.Equal(t, CompareWithLatest, compareWith)
			})
		})
	}
}

func TestUpdatedManagedNamespaceMetadata(t *testing.T) {
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})
	app := newFakeApp()
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &v1alpha1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	app.Status.Sync.ComparedTo.Source = app.Spec.GetSource()
	app.Status.Sync.ComparedTo.Destination = app.Spec.Destination

	// Ensure that hard/soft refresh isn't triggered due to reconciledAt being expired
	reconciledAt := metav1.NewTime(time.Now().UTC().Add(15 * time.Minute))
	app.Status.ReconciledAt = &reconciledAt
	needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 30*time.Minute, 2*time.Hour)

	assert.True(t, needRefresh)
	assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
	assert.Equal(t, CompareWithLatest, compareWith)
}

func TestUnchangedManagedNamespaceMetadata(t *testing.T) {
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{}})
	app := newFakeApp()
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &v1alpha1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	app.Status.Sync.ComparedTo.Source = app.Spec.GetSource()
	app.Status.Sync.ComparedTo.Destination = app.Spec.Destination
	app.Status.OperationState.SyncResult.ManagedNamespaceMetadata = app.Spec.SyncPolicy.ManagedNamespaceMetadata

	// Ensure that hard/soft refresh isn't triggered due to reconciledAt being expired
	reconciledAt := metav1.NewTime(time.Now().UTC().Add(15 * time.Minute))
	app.Status.ReconciledAt = &reconciledAt
	needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 30*time.Minute, 2*time.Hour)

	assert.False(t, needRefresh)
	assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
	assert.Equal(t, CompareWithLatest, compareWith)
}

func TestRefreshAppConditions(t *testing.T) {
	defaultProj := v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	}

	t.Run("NoErrorConditions", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &defaultProj}})

		_, hasErrors := ctrl.refreshAppConditions(app)
		assert.False(t, hasErrors)
		assert.Len(t, app.Status.Conditions, 0)
	})

	t.Run("PreserveExistingWarningCondition", func(t *testing.T) {
		app := newFakeApp()
		app.Status.SetConditions([]v1alpha1.ApplicationCondition{{Type: v1alpha1.ApplicationConditionExcludedResourceWarning}}, nil)

		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &defaultProj}})

		_, hasErrors := ctrl.refreshAppConditions(app)
		assert.False(t, hasErrors)
		assert.Len(t, app.Status.Conditions, 1)
		assert.Equal(t, v1alpha1.ApplicationConditionExcludedResourceWarning, app.Status.Conditions[0].Type)
	})

	t.Run("ReplacesSpecErrorCondition", func(t *testing.T) {
		app := newFakeApp()
		app.Spec.Project = "wrong project"
		app.Status.SetConditions([]v1alpha1.ApplicationCondition{{Type: v1alpha1.ApplicationConditionInvalidSpecError, Message: "old message"}}, nil)

		ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &defaultProj}})

		_, hasErrors := ctrl.refreshAppConditions(app)
		assert.True(t, hasErrors)
		assert.Len(t, app.Status.Conditions, 1)
		assert.Equal(t, v1alpha1.ApplicationConditionInvalidSpecError, app.Status.Conditions[0].Type)
		assert.Equal(t, "Application referencing project wrong project which does not exist", app.Status.Conditions[0].Message)
	})
}

func TestUpdateReconciledAt(t *testing.T) {
	app := newFakeApp()
	reconciledAt := metav1.NewTime(time.Now().Add(-1 * time.Second))
	app.Status = v1alpha1.ApplicationStatus{ReconciledAt: &reconciledAt}
	app.Status.Sync = v1alpha1.SyncStatus{ComparedTo: v1alpha1.ComparedTo{Source: app.Spec.GetSource(), Destination: app.Spec.Destination, IgnoreDifferences: app.Spec.IgnoreDifferences}}
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	})
	key, _ := cache.MetaNamespaceKeyFunc(app)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	receivedPatch := map[string]interface{}{}
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, nil, nil
	})

	t.Run("UpdatedOnFullReconciliation", func(t *testing.T) {
		receivedPatch = map[string]interface{}{}
		ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
		ctrl.appRefreshQueue.Add(key)

		ctrl.processAppRefreshQueueItem()

		_, updated, err := unstructured.NestedString(receivedPatch, "status", "reconciledAt")
		assert.NoError(t, err)
		assert.True(t, updated)

		_, updated, err = unstructured.NestedString(receivedPatch, "status", "observedAt")
		assert.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("NotUpdatedOnPartialReconciliation", func(t *testing.T) {
		receivedPatch = map[string]interface{}{}
		ctrl.appRefreshQueue.Add(key)
		ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)

		ctrl.processAppRefreshQueueItem()

		_, updated, err := unstructured.NestedString(receivedPatch, "status", "reconciledAt")
		assert.NoError(t, err)
		assert.False(t, updated)

		_, updated, err = unstructured.NestedString(receivedPatch, "status", "observedAt")
		assert.NoError(t, err)
		assert.False(t, updated)
	})

}

func TestProjectErrorToCondition(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "wrong project"
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	})
	key, _ := cache.MetaNamespaceKeyFunc(app)
	ctrl.appRefreshQueue.Add(key)
	ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)

	ctrl.processAppRefreshQueueItem()

	obj, ok, err := ctrl.appInformer.GetIndexer().GetByKey(key)
	assert.True(t, ok)
	assert.NoError(t, err)
	updatedApp := obj.(*v1alpha1.Application)
	assert.Equal(t, v1alpha1.ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[0].Type)
	assert.Equal(t, "Application referencing project wrong project which does not exist", updatedApp.Status.Conditions[0].Message)
	assert.Equal(t, v1alpha1.ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[0].Type)
}

func TestFinalizeProjectDeletion_HasApplications(t *testing.T) {
	app := newFakeApp()
	proj := &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: test.FakeArgoCDNamespace}}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, proj}})

	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	patched := false
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, nil, nil
	})

	err := ctrl.finalizeProjectDeletion(proj)
	assert.NoError(t, err)
	assert.False(t, patched)
}

func TestFinalizeProjectDeletion_DoesNotHaveApplications(t *testing.T) {
	proj := &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: test.FakeArgoCDNamespace}}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{&defaultProj}})

	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]interface{}{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, nil, nil
	})

	err := ctrl.finalizeProjectDeletion(proj)
	assert.NoError(t, err)
	assert.Equal(t, map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": nil,
		},
	}, receivedPatch)
}

func TestProcessRequestedAppOperation_FailedNoRetries(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "default"
	app.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]interface{}{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, nil, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	assert.Equal(t, string(synccommon.OperationError), phase)
}

func TestProcessRequestedAppOperation_InvalidDestination(t *testing.T) {
	app := newFakeAppWithDestMismatch()
	app.Spec.Project = "test-project"
	app.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}
	proj := defaultProj
	proj.Name = "test-project"
	proj.Spec.SourceNamespaces = []string{test.FakeArgoCDNamespace}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app, &proj}})
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]interface{}{}
	func() {
		fakeAppCs.Lock()
		defer fakeAppCs.Unlock()
		fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if patchAction, ok := action.(kubetesting.PatchAction); ok {
				assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
			}
			return true, nil, nil
		})
	}()

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	assert.Equal(t, string(synccommon.OperationFailed), phase)
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Contains(t, message, "application destination can't have both name and server defined: another-cluster https://localhost:6443")
}

func TestProcessRequestedAppOperation_FailedHasRetries(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "invalid-project"
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 1},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]interface{}{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, nil, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	assert.Equal(t, string(synccommon.OperationRunning), phase)
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Contains(t, message, "Retrying attempt #1")
	retryCount, _, _ := unstructured.NestedFloat64(receivedPatch, "status", "operationState", "retryCount")
	assert.Equal(t, float64(1), retryCount)
}

func TestProcessRequestedAppOperation_RunningPreviouslyFailed(t *testing.T) {
	app := newFakeApp()
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 1},
	}
	app.Status.OperationState.Phase = synccommon.OperationRunning
	app.Status.OperationState.SyncResult.Resources = []*v1alpha1.ResourceResult{{
		Name:   "guestbook",
		Kind:   "Deployment",
		Group:  "apps",
		Status: synccommon.ResultCodeSyncFailed,
	}}

	data := &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
	}
	ctrl := newFakeController(data)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]interface{}{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, nil, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	assert.Equal(t, string(synccommon.OperationSucceeded), phase)
}

func TestProcessRequestedAppOperation_HasRetriesTerminated(t *testing.T) {
	app := newFakeApp()
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 10},
	}
	app.Status.OperationState.Phase = synccommon.OperationTerminating

	data := &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
	}
	ctrl := newFakeController(data)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]interface{}{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			assert.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, nil, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	assert.Equal(t, string(synccommon.OperationFailed), phase)
}

func TestGetAppHosts(t *testing.T) {
	app := newFakeApp()
	data := &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
	}
	ctrl := newFakeController(data)
	mockStateCache := &mockstatecache.LiveStateCache{}
	mockStateCache.On("IterateResources", mock.Anything, mock.MatchedBy(func(callback func(res *clustercache.Resource, info *statecache.ResourceInfo)) bool {
		// node resource
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "minikube", Kind: "Node", APIVersion: "v1"},
		}, &statecache.ResourceInfo{NodeInfo: &statecache.NodeInfo{
			Name:       "minikube",
			SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
			Capacity:   map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("5")},
		}})

		// app pod
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "pod1", Kind: kube.PodKind, APIVersion: "v1", Namespace: "default"},
		}, &statecache.ResourceInfo{PodInfo: &statecache.PodInfo{
			NodeName:         "minikube",
			ResourceRequests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("1")},
		}})
		// neighbor pod
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "pod2", Kind: kube.PodKind, APIVersion: "v1", Namespace: "default"},
		}, &statecache.ResourceInfo{PodInfo: &statecache.PodInfo{
			NodeName:         "minikube",
			ResourceRequests: map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("2")},
		}})
		return true
	})).Return(nil)
	ctrl.stateCache = mockStateCache

	hosts, err := ctrl.getAppHosts(app, []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{Name: "pod1", Namespace: "default", Kind: kube.PodKind},
		Info: []v1alpha1.InfoItem{{
			Name:  "Host",
			Value: "Minikube",
		}},
	}})

	assert.NoError(t, err)
	assert.Equal(t, []v1alpha1.HostInfo{{
		Name:       "minikube",
		SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
		ResourcesInfo: []v1alpha1.HostResourceInfo{{
			ResourceName: corev1.ResourceCPU, Capacity: 5000, RequestedByApp: 1000, RequestedByNeighbors: 2000},
		}}}, hosts)
}

func TestMetricsExpiration(t *testing.T) {
	app := newFakeApp()
	// Check expiration is disabled by default
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	assert.False(t, ctrl.metricsServer.HasExpiration())
	// Check expiration is enabled if set
	ctrl = newFakeController(&fakeData{apps: []runtime.Object{app}, metricsCacheExpiration: 10 * time.Second})
	assert.True(t, ctrl.metricsServer.HasExpiration())
}

func TestToAppKey(t *testing.T) {
	ctrl := newFakeController(&fakeData{})
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"From instance name", "foo_bar", "foo/bar"},
		{"From qualified name", "foo/bar", "foo/bar"},
		{"From unqualified name", "bar", ctrl.namespace + "/bar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ctrl.toAppKey(tt.input))
		})
	}
}

func Test_canProcessApp(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	ctrl.applicationNamespaces = []string{"good"}
	t.Run("without cluster filter, good namespace", func(t *testing.T) {
		app.Namespace = "good"
		canProcess := ctrl.canProcessApp(app)
		assert.True(t, canProcess)
	})
	t.Run("without cluster filter, bad namespace", func(t *testing.T) {
		app.Namespace = "bad"
		canProcess := ctrl.canProcessApp(app)
		assert.False(t, canProcess)
	})
	t.Run("with cluster filter, good namespace", func(t *testing.T) {
		app.Namespace = "good"
		ctrl.clusterFilter = func(_ *v1alpha1.Cluster) bool { return true }
		canProcess := ctrl.canProcessApp(app)
		assert.True(t, canProcess)
	})
	t.Run("with cluster filter, bad namespace", func(t *testing.T) {
		app.Namespace = "bad"
		ctrl.clusterFilter = func(_ *v1alpha1.Cluster) bool { return true }
		canProcess := ctrl.canProcessApp(app)
		assert.False(t, canProcess)
	})
}

func Test_canProcessAppSkipReconcileAnnotation(t *testing.T) {
	appSkipReconcileInvalid := newFakeApp()
	appSkipReconcileInvalid.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "invalid-value"}
	appSkipReconcileFalse := newFakeApp()
	appSkipReconcileFalse.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "false"}
	appSkipReconcileTrue := newFakeApp()
	appSkipReconcileTrue.Annotations = map[string]string{common.AnnotationKeyAppSkipReconcile: "true"}
	ctrl := newFakeController(&fakeData{})
	tests := []struct {
		name     string
		input    interface{}
		expected bool
	}{
		{"No skip reconcile annotation", newFakeApp(), true},
		{"Contains skip reconcile annotation ", appSkipReconcileInvalid, true},
		{"Contains skip reconcile annotation value false", appSkipReconcileFalse, true},
		{"Contains skip reconcile annotation value true", appSkipReconcileTrue, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, ctrl.canProcessApp(tt.input))
		})
	}
}

func Test_syncDeleteOption(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	cm := newFakeCM()
	t.Run("without delete option object is deleted", func(t *testing.T) {
		cmObj := kube.MustToUnstructured(&cm)
		delete := ctrl.shouldBeDeleted(app, cmObj)
		assert.True(t, delete)
	})
	t.Run("with delete set to false object is retained", func(t *testing.T) {
		cmObj := kube.MustToUnstructured(&cm)
		cmObj.SetAnnotations(map[string]string{"argocd.argoproj.io/sync-options": "Delete=false"})
		delete := ctrl.shouldBeDeleted(app, cmObj)
		assert.False(t, delete)
	})
	t.Run("with delete set to false object is retained", func(t *testing.T) {
		cmObj := kube.MustToUnstructured(&cm)
		cmObj.SetAnnotations(map[string]string{"helm.sh/resource-policy": "keep"})
		delete := ctrl.shouldBeDeleted(app, cmObj)
		assert.False(t, delete)
	})
}

func TestAddControllerNamespace(t *testing.T) {
	t.Run("set controllerNamespace when the app is in the controller namespace", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(&fakeData{
			apps:             []runtime.Object{app, &defaultProj},
			manifestResponse: &apiclient.ManifestResponse{},
		})

		ctrl.processAppRefreshQueueItem()

		updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(context.Background(), app.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, test.FakeArgoCDNamespace, updatedApp.Status.ControllerNamespace)
	})
	t.Run("set controllerNamespace when the app is in another namespace than the controller", func(t *testing.T) {
		appNamespace := "app-namespace"

		app := newFakeApp()
		app.ObjectMeta.Namespace = appNamespace
		proj := defaultProj
		proj.Spec.SourceNamespaces = []string{appNamespace}
		ctrl := newFakeController(&fakeData{
			apps:                  []runtime.Object{app, &proj},
			manifestResponse:      &apiclient.ManifestResponse{},
			applicationNamespaces: []string{appNamespace},
		})

		ctrl.processAppRefreshQueueItem()

		updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(appNamespace).Get(context.Background(), app.Name, metav1.GetOptions{})
		assert.NoError(t, err)
		assert.Equal(t, test.FakeArgoCDNamespace, updatedApp.Status.ControllerNamespace)
	})
}
