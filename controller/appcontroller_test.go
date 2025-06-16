package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube/kubetest"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/v3/common"
	statecache "github.com/argoproj/argo-cd/v3/controller/cache"
	"github.com/argoproj/argo-cd/v3/controller/sharding"

	"github.com/argoproj/gitops-engine/pkg/cache/mocks"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/yaml"

	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"

	mockcommitclient "github.com/argoproj/argo-cd/v3/commitserver/apiclient/mocks"
	mockstatecache "github.com/argoproj/argo-cd/v3/controller/cache/mocks"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/v3/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/argo/normalizers"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	appstatecache "github.com/argoproj/argo-cd/v3/util/cache/appstate"
	"github.com/argoproj/argo-cd/v3/util/settings"
	utilTest "github.com/argoproj/argo-cd/v3/util/test"
)

var testEnableEventList []string = argo.DefaultEnableEventList()

type namespacedResource struct {
	v1alpha1.ResourceNode
	AppName string
}

type fakeData struct {
	apps                            []runtime.Object
	manifestResponse                *apiclient.ManifestResponse
	manifestResponses               []*apiclient.ManifestResponse
	managedLiveObjs                 map[kube.ResourceKey]*unstructured.Unstructured
	namespacedResources             map[kube.ResourceKey]namespacedResource
	configMapData                   map[string]string
	metricsCacheExpiration          time.Duration
	applicationNamespaces           []string
	updateRevisionForPathsResponse  *apiclient.UpdateRevisionForPathsResponse
	updateRevisionForPathsResponses []*apiclient.UpdateRevisionForPathsResponse
	additionalObjs                  []runtime.Object
}

type MockKubectl struct {
	kube.Kubectl

	DeletedResources []kube.ResourceKey
	CreatedResources []*unstructured.Unstructured
}

func (m *MockKubectl) CreateResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, obj *unstructured.Unstructured, createOptions metav1.CreateOptions, subresources ...string) (*unstructured.Unstructured, error) {
	m.CreatedResources = append(m.CreatedResources, obj)
	return m.Kubectl.CreateResource(ctx, config, gvk, name, namespace, obj, createOptions, subresources...)
}

func (m *MockKubectl) DeleteResource(ctx context.Context, config *rest.Config, gvk schema.GroupVersionKind, name string, namespace string, deleteOptions metav1.DeleteOptions) error {
	m.DeletedResources = append(m.DeletedResources, kube.NewResourceKey(gvk.Group, gvk.Kind, namespace, name))
	return m.Kubectl.DeleteResource(ctx, config, gvk, name, namespace, deleteOptions)
}

func newFakeController(ctx context.Context, data *fakeData, repoErr error) *ApplicationController {
	return newFakeControllerWithResync(ctx, data, time.Minute, repoErr, nil)
}

func newFakeControllerWithResync(ctx context.Context, data *fakeData, appResyncPeriod time.Duration, repoErr, revisionPathsErr error) *ApplicationController {
	var clust corev1.Secret
	err := yaml.Unmarshal([]byte(fakeCluster), &clust)
	if err != nil {
		panic(err)
	}

	// Mock out call to GenerateManifest
	mockRepoClient := &mockrepoclient.RepoServerServiceClient{}

	if len(data.manifestResponses) > 0 {
		for _, response := range data.manifestResponses {
			if repoErr != nil {
				mockRepoClient.EXPECT().GenerateManifest(mock.Anything, mock.Anything).Return(response, repoErr).Once()
			} else {
				mockRepoClient.EXPECT().GenerateManifest(mock.Anything, mock.Anything).Return(response, nil).Once()
			}
		}
	} else {
		if repoErr != nil {
			mockRepoClient.EXPECT().GenerateManifest(mock.Anything, mock.Anything).Return(data.manifestResponse, repoErr).Once()
		} else {
			mockRepoClient.EXPECT().GenerateManifest(mock.Anything, mock.Anything).Return(data.manifestResponse, nil).Once()
		}
	}

	if len(data.updateRevisionForPathsResponses) > 0 {
		for _, response := range data.updateRevisionForPathsResponses {
			if revisionPathsErr != nil {
				mockRepoClient.EXPECT().UpdateRevisionForPaths(mock.Anything, mock.Anything).Return(response, revisionPathsErr)
			} else {
				mockRepoClient.EXPECT().UpdateRevisionForPaths(mock.Anything, mock.Anything).Return(response, nil)
			}
		}
	} else {
		if revisionPathsErr != nil {
			mockRepoClient.EXPECT().UpdateRevisionForPaths(mock.Anything, mock.Anything).Return(nil, revisionPathsErr)
		} else {
			mockRepoClient.EXPECT().UpdateRevisionForPaths(mock.Anything, mock.Anything).Return(data.updateRevisionForPathsResponse, nil)
		}
	}

	mockRepoClientset := &mockrepoclient.Clientset{RepoServerServiceClient: mockRepoClient}

	mockCommitClientset := &mockcommitclient.Clientset{}

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
	runtimeObjs := []runtime.Object{&clust, &secret, &cm}
	runtimeObjs = append(runtimeObjs, data.additionalObjs...)
	kubeClient := fake.NewClientset(runtimeObjs...)
	settingsMgr := settings.NewSettingsManager(ctx, kubeClient, test.FakeArgoCDNamespace)
	// Initialize the settings manager to ensure cluster cache is ready
	_ = settingsMgr.ResyncInformers()
	kubectl := &MockKubectl{Kubectl: &kubetest.MockKubectlCmd{}}
	ctrl, err := NewApplicationController(
		test.FakeArgoCDNamespace,
		settingsMgr,
		kubeClient,
		appclientset.NewSimpleClientset(data.apps...),
		mockRepoClientset,
		mockCommitClientset,
		appstatecache.NewCache(
			cacheutil.NewCache(cacheutil.NewInMemoryCache(1*time.Minute)),
			1*time.Minute,
		),
		kubectl,
		appResyncPeriod,
		time.Hour,
		time.Second,
		time.Minute,
		nil,
		0,
		time.Second*10,
		common.DefaultPortArgoCDMetrics,
		data.metricsCacheExpiration,
		[]string{},
		[]string{},
		[]string{},
		0,
		true,
		nil,
		data.applicationNamespaces,
		nil,
		false,
		false,
		normalizers.IgnoreNormalizerOpts{},
		testEnableEventList,
		false,
	)
	db := &dbmocks.ArgoDB{}
	db.EXPECT().GetApplicationControllerReplicas().Return(1).Maybe()
	// Setting a default sharding algorithm for the tests where we cannot set it.
	ctrl.clusterSharding = sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm)
	if err != nil {
		panic(err)
	}
	cancelProj := test.StartInformer(ctrl.projInformer)
	defer cancelProj()
	cancelApp := test.StartInformer(ctrl.appInformer)
	defer cancelApp()
	clusterCacheMock := &mocks.ClusterCache{}
	clusterCacheMock.EXPECT().IsNamespaced(mock.Anything).Return(true, nil)
	clusterCacheMock.EXPECT().GetOpenAPISchema().Return(nil)
	clusterCacheMock.EXPECT().GetGVKParser().Return(nil)

	mockStateCache := &mockstatecache.LiveStateCache{}
	ctrl.appStateManager.(*appStateManager).liveStateCache = mockStateCache
	ctrl.stateCache = mockStateCache
	mockStateCache.EXPECT().IsNamespaced(mock.Anything, mock.Anything).Return(true, nil)
	mockStateCache.EXPECT().GetManagedLiveObjs(mock.Anything, mock.Anything, mock.Anything).Return(data.managedLiveObjs, nil)
	mockStateCache.EXPECT().GetVersionsInfo(mock.Anything).Return("v1.2.3", nil, nil)
	response := make(map[kube.ResourceKey]v1alpha1.ResourceNode)
	for k, v := range data.namespacedResources {
		response[k] = v.ResourceNode
	}
	mockStateCache.EXPECT().GetNamespaceTopLevelResources(mock.Anything, mock.Anything).Return(response, nil)
	mockStateCache.EXPECT().IterateResources(mock.Anything, mock.Anything).Return(nil)
	mockStateCache.EXPECT().GetClusterCache(mock.Anything).Return(clusterCacheMock, nil)
	mockStateCache.EXPECT().IterateHierarchyV2(mock.Anything, mock.Anything, mock.Anything).Run(func(_ *v1alpha1.Cluster, keys []kube.ResourceKey, action func(_ v1alpha1.ResourceNode, _ string) bool) {
		for _, key := range keys {
			appName := ""
			if res, ok := data.namespacedResources[key]; ok {
				appName = res.AppName
			}
			_ = action(v1alpha1.ResourceNode{ResourceRef: v1alpha1.ResourceRef{Kind: key.Kind, Group: key.Group, Namespace: key.Namespace, Name: key.Name}}, appName)
		}
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
        helm:
          valueFiles:
          - $values_test/values.yaml
        repoURL: https://github.com/argoproj/argocd-example-apps.git
      - path: some/other/path
        repoURL: https://github.com/argoproj/argocd-example-apps-fake.git
      - ref: values_test
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

var fakePreDeleteHook = `
{
  "apiVersion": "v1",
  "kind": "Pod",
  "metadata": {
    "name": "pre-delete-hook",
    "namespace": "default",
    "labels": {
      "app.kubernetes.io/instance": "my-app"
    },
    "annotations": {
      "argocd.argoproj.io/hook": "PreDelete"
    }
  },
  "spec": {
    "containers": [
      {
        "name": "pre-delete-hook",
        "image": "busybox",
        "restartPolicy": "Never",
        "command": [
          "/bin/sh",
          "-c",
          "sleep 5 && echo hello from the pre-delete-hook pod"
        ]
      }
    ]
  }
}
`

var fakePostDeleteHook = `
{
  "apiVersion": "batch/v1",
  "kind": "Job",
  "metadata": {
    "name": "post-delete-hook",
    "namespace": "default",
    "labels": {
      "app.kubernetes.io/instance": "my-app"
    },
    "annotations": {
      "argocd.argoproj.io/hook": "PostDelete",
      "argocd.argoproj.io/hook-delete-policy": "HookSucceeded"
    }
  },
  "spec": {
    "template": {
      "metadata": {
        "name": "post-delete-hook"
      },
      "spec": {
        "containers": [
          {
            "name": "post-delete-hook",
            "image": "busybox",
            "command": [
              "/bin/sh",
              "-c",
              "sleep 5 && echo hello from the post-delete-hook job"
            ]
          }
        ],
        "restartPolicy": "Never"
      }
    }
  }
}
`

var fakeServiceAccount = `
{
  "apiVersion": "v1",
  "kind": "ServiceAccount",
  "metadata": {
    "name": "hook-serviceaccount",
    "namespace": "default",
    "annotations": {
      "argocd.argoproj.io/hook": "PostDelete",
      "argocd.argoproj.io/hook-delete-policy": "BeforeHookCreation,HookSucceeded"
    }
  }
}
`

var fakeRole = `
{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "Role",
  "metadata": {
    "name": "hook-role",
    "namespace": "default",
    "annotations": {
      "argocd.argoproj.io/hook": "PostDelete",
      "argocd.argoproj.io/hook-delete-policy": "BeforeHookCreation,HookSucceeded"
    }
  },
  "rules": [
    {
      "apiGroups": [""],
      "resources": ["secrets"],
      "verbs": ["get", "delete", "list"]
    }
  ]
}
`

var fakeRoleBinding = `
{
  "apiVersion": "rbac.authorization.k8s.io/v1",
  "kind": "RoleBinding",
  "metadata": {
    "name": "hook-rolebinding",
    "namespace": "default",
    "annotations": {
      "argocd.argoproj.io/hook": "PostDelete",
      "argocd.argoproj.io/hook-delete-policy": "BeforeHookCreation,HookSucceeded"
    }
  },
  "roleRef": {
    "apiGroup": "rbac.authorization.k8s.io",
    "kind": "Role",
    "name": "hook-role"
  },
  "subjects": [
    {
      "kind": "ServiceAccount",
      "name": "hook-serviceaccount",
      "namespace": "default"
    }
  ]
}
`

func newFakeApp() *v1alpha1.Application {
	return createFakeApp(fakeApp)
}

func newFakeAppWithHealthAndTime(status health.HealthStatusCode, timestamp metav1.Time) *v1alpha1.Application {
	return createFakeAppWithHealthAndTime(fakeApp, status, timestamp)
}

func newFakeMultiSourceApp() *v1alpha1.Application {
	return createFakeApp(fakeMultiSourceApp)
}

func createFakeAppWithHealthAndTime(testApp string, status health.HealthStatusCode, timestamp metav1.Time) *v1alpha1.Application {
	app := createFakeApp(testApp)
	app.Status.Health = v1alpha1.AppHealthStatus{
		Status:             status,
		LastTransitionTime: &timestamp,
	}
	return app
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

func newFakeCM() map[string]any {
	var cm map[string]any
	err := yaml.Unmarshal([]byte(fakeStrayResource), &cm)
	if err != nil {
		panic(err)
	}
	return cm
}

func newFakePreDeleteHook() map[string]any {
	var cm map[string]any
	err := yaml.Unmarshal([]byte(fakePreDeleteHook), &cm)
	if err != nil {
		panic(err)
	}
	return cm
}

func newFakePostDeleteHook() map[string]any {
	var hook map[string]any
	err := yaml.Unmarshal([]byte(fakePostDeleteHook), &hook)
	if err != nil {
		panic(err)
	}
	return hook
}

func newFakeRoleBinding() map[string]any {
	var roleBinding map[string]any
	err := yaml.Unmarshal([]byte(fakeRoleBinding), &roleBinding)
	if err != nil {
		panic(err)
	}
	return roleBinding
}

func newFakeRole() map[string]any {
	var role map[string]any
	err := yaml.Unmarshal([]byte(fakeRole), &role)
	if err != nil {
		panic(err)
	}
	return role
}

func newFakeServiceAccount() map[string]any {
	var serviceAccount map[string]any
	err := yaml.Unmarshal([]byte(fakeServiceAccount), &serviceAccount)
	if err != nil {
		panic(err)
	}
	return serviceAccount
}

func TestAutoSync(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, app.Operation)
	assert.NotNil(t, app.Operation.Sync)
	assert.False(t, app.Operation.Sync.Prune)
}

func TestAutoSyncEnabledSetToTrue(t *testing.T) {
	app := newFakeApp()
	enable := true
	app.Spec.SyncPolicy.Automated = &v1alpha1.SyncPolicyAutomated{Enabled: &enable}
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
	require.NoError(t, err)
	assert.NotNil(t, app.Operation)
	assert.NotNil(t, app.Operation.Sync)
	assert.False(t, app.Operation.Sync.Prune)
}

func TestAutoSyncMultiSourceWithoutSelfHeal(t *testing.T) {
	// Simulate OutOfSync caused by object change in cluster
	// So our Sync Revisions and SyncStatus Revisions should deep equal
	t.Run("ClusterObjectChangeShouldNotTriggerAutoSync", func(t *testing.T) {
		app := newFakeMultiSourceApp()
		app.Spec.SyncPolicy.Automated.SelfHeal = false
		app.Status.OperationState.SyncResult.Revisions = []string{"z", "x", "v"}
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:    v1alpha1.SyncStatusCodeOutOfSync,
			Revisions: []string{"z", "x", "v"},
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook-1", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})
	t.Run("NewRevisionChangeShouldTriggerAutoSync", func(t *testing.T) {
		app := newFakeMultiSourceApp()
		app.Spec.SyncPolicy.Automated.SelfHeal = false
		app.Status.OperationState.SyncResult.Revisions = []string{"z", "x", "v"}
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:    v1alpha1.SyncStatusCodeOutOfSync,
			Revisions: []string{"a", "b", "c"},
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook-1", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, app.Operation)
	})
}

func TestAutoSyncNotAllowEmpty(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy.Automated.Prune = true
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
	assert.NotNil(t, cond)
}

func TestAutoSyncAllowEmpty(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy.Automated.Prune = true
	app.Spec.SyncPolicy.Automated.AllowEmpty = true
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	syncStatus := v1alpha1.SyncStatus{
		Status:   v1alpha1.SyncStatusCodeOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
	assert.Nil(t, cond)
}

func TestSkipAutoSync(t *testing.T) {
	// Verify we skip when we previously synced to it in our most recent history
	// Set current to 'aaaaa', desired to 'aaaa' and mark system OutOfSync
	t.Run("PreviouslySyncedToRevision", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when we are already Synced (even if revision is different)
	t.Run("AlreadyInSyncedState", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeSynced,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when auto-sync is disabled
	t.Run("AutoSyncIsDisabled", func(t *testing.T) {
		app := newFakeApp()
		app.Spec.SyncPolicy = nil
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when auto-sync is disabled
	t.Run("AutoSyncEnableFieldIsSetFalse", func(t *testing.T) {
		app := newFakeApp()
		enable := false
		app.Spec.SyncPolicy.Automated = &v1alpha1.SyncPolicyAutomated{Enabled: &enable}
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	// Verify we skip when application is marked for deletion
	t.Run("ApplicationIsMarkedForDeletion", func(t *testing.T) {
		app := newFakeApp()
		now := metav1.Now()
		app.DeletionTimestamp = &now
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
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
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
		assert.NotNil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	t.Run("PreviousSyncAttemptError", func(t *testing.T) {
		app := newFakeApp()
		app.Status.OperationState = &v1alpha1.OperationState{
			Operation: v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{},
			},
			Phase: synccommon.OperationError,
			SyncResult: &v1alpha1.SyncOperationResult{
				Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				Source:   *app.Spec.Source.DeepCopy(),
			},
		}
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
		assert.NotNil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.Nil(t, app.Operation)
	})

	t.Run("NeedsToPruneResourcesOnlyButAutomatedPruneDisabled", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{
			{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync, RequiresPruning: true},
		}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
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
	cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
	assert.NotNil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
	require.NoError(t, err)
	assert.Nil(t, app.Operation)
}

// TestAutoSyncParameterOverrides verifies we auto-sync if revision is same but parameter overrides are different
func TestAutoSyncParameterOverrides(t *testing.T) {
	t.Run("Single source", func(t *testing.T) {
		app := newFakeApp()
		app.Spec.Source.Helm = &v1alpha1.ApplicationSourceHelm{
			Parameters: []v1alpha1.HelmParameter{
				{
					Name:  "a",
					Value: "1",
				},
			},
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
		syncStatus := v1alpha1.SyncStatus{
			Status:   v1alpha1.SyncStatusCodeOutOfSync,
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		}
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, app.Operation)
	})

	t.Run("Multi sources", func(t *testing.T) {
		app := newFakeMultiSourceApp()
		app.Spec.Sources[0].Helm = &v1alpha1.ApplicationSourceHelm{
			Parameters: []v1alpha1.HelmParameter{
				{
					Name:  "a",
					Value: "1",
				},
			},
		}
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
		app.Status.OperationState.SyncResult.Revisions = []string{"z", "x", "v"}
		app.Status.OperationState.SyncResult.Sources[0].Helm = &v1alpha1.ApplicationSourceHelm{
			Parameters: []v1alpha1.HelmParameter{
				{
					Name:  "a",
					Value: "2", // this value changed
				},
			},
		}
		syncStatus := v1alpha1.SyncStatus{
			Status:    v1alpha1.SyncStatusCodeOutOfSync,
			Revisions: []string{"z", "x", "v"},
		}
		cond, _ := ctrl.autoSync(app, &syncStatus, []v1alpha1.ResourceStatus{{Name: "guestbook", Kind: kube.DeploymentKind, Status: v1alpha1.SyncStatusCodeOutOfSync}}, true)
		assert.Nil(t, cond)
		app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(t.Context(), "my-app", metav1.GetOptions{})
		require.NoError(t, err)
		assert.NotNil(t, app.Operation)
	})
}

// TestFinalizeAppDeletion verifies application deletion
func TestFinalizeAppDeletion(t *testing.T) {
	now := metav1.Now()
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
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{}}, nil)
		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		assert.True(t, patched)
	})

	// Ensure any stray resources irregularly labeled with instance label of app are not deleted upon deleting,
	// when app project restriction is in place
	t.Run("ProjectRestrictionEnforced", func(t *testing.T) {
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
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		app.Spec.Project = "restricted"
		appObj := kube.MustToUnstructured(&app)
		cm := newFakeCM()
		strayObj := kube.MustToUnstructured(&cm)
		ctrl := newFakeController(t.Context(), &fakeData{
			apps: []runtime.Object{app, &defaultProj, &restrictedProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(appObj):   appObj,
				kube.GetResourceKey(strayObj): strayObj,
			},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		assert.True(t, patched)
		objsMap, err := ctrl.stateCache.GetManagedLiveObjs(&v1alpha1.Cluster{Server: "test", Name: "test"}, app, []*unstructured.Unstructured{})
		if err != nil {
			require.NoError(t, err)
		}
		// Managed objects must be empty
		assert.Empty(t, objsMap)

		// Loop through all deleted objects, ensure that test-cm is none of them
		for _, o := range ctrl.kubectl.(*MockKubectl).DeletedResources {
			assert.NotEqual(t, "test-cm", o.Name)
		}
	})

	t.Run("DeleteWithDestinationClusterName", func(t *testing.T) {
		app := newFakeAppWithDestName()
		app.SetCascadedDeletion(v1alpha1.ResourcesFinalizerName)
		app.DeletionTimestamp = &now
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{}}, nil)
		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		assert.True(t, patched)
	})

	// Create an Application with a cluster that doesn't exist
	// Ensure it can be deleted.
	t.Run("DeleteWithInvalidClusterName", func(t *testing.T) {
		appTemplate := newFakeAppWithDestName()

		testShouldDelete := func(app *v1alpha1.Application) {
			appObj := kube.MustToUnstructured(&app)
			ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}, managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(appObj): appObj,
			}}, nil)

			fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
			defaultReactor := fakeAppCs.ReactionChain[0]
			fakeAppCs.ReactionChain = nil
			fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
				return defaultReactor.React(action)
			})
			err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
				return []*v1alpha1.Cluster{}, nil
			})
			require.NoError(t, err)
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

	t.Run("PreDelete_HookIsCreated", func(t *testing.T) {
		app := newFakeApp()
		app.SetPreDeleteFinalizer()
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		ctrl := newFakeController(context.Background(), &fakeData{
			manifestResponses: []*apiclient.ManifestResponse{{
				Manifests: []string{fakePreDeleteHook},
			}},
			apps:            []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		// finalizer is not deleted
		assert.False(t, patched)
		// pre-delete hook is created
		require.Len(t, ctrl.kubectl.(*MockKubectl).CreatedResources, 1)
		require.Equal(t, "pre-delete-hook", ctrl.kubectl.(*MockKubectl).CreatedResources[0].GetName())
	})

	t.Run("PostDelete_HookIsCreated", func(t *testing.T) {
		app := newFakeApp()
		app.SetPostDeleteFinalizer()
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		ctrl := newFakeController(t.Context(), &fakeData{
			manifestResponses: []*apiclient.ManifestResponse{{
				Manifests: []string{fakePostDeleteHook},
			}},
			apps:            []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		// finalizer is not deleted
		assert.False(t, patched)
		// post-delete hook is created
		require.Len(t, ctrl.kubectl.(*MockKubectl).CreatedResources, 1)
		require.Equal(t, "post-delete-hook", ctrl.kubectl.(*MockKubectl).CreatedResources[0].GetName())
	})

	t.Run("PreDelete_HookIsExecuted", func(t *testing.T) {
		app := newFakeApp()
		app.SetPreDeleteFinalizer()
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		liveHook := &unstructured.Unstructured{Object: newFakePreDeleteHook()}
		require.NoError(t, unstructured.SetNestedField(liveHook.Object, "Succeeded", "status", "phase"))
		ctrl := newFakeController(context.Background(), &fakeData{
			manifestResponses: []*apiclient.ManifestResponse{{
				Manifests: []string{fakePreDeleteHook},
			}},
			apps: []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(liveHook): liveHook,
			},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		// finalizer is removed
		assert.True(t, patched)
	})

	t.Run("PostDelete_HookIsExecuted", func(t *testing.T) {
		app := newFakeApp()
		app.SetPostDeleteFinalizer()
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		liveHook := &unstructured.Unstructured{Object: newFakePostDeleteHook()}
		conditions := []any{
			map[string]any{
				"type":   "Complete",
				"status": "True",
			},
		}
		require.NoError(t, unstructured.SetNestedField(liveHook.Object, conditions, "status", "conditions"))
		ctrl := newFakeController(t.Context(), &fakeData{
			manifestResponses: []*apiclient.ManifestResponse{{
				Manifests: []string{fakePostDeleteHook},
			}},
			apps: []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(liveHook): liveHook,
			},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		// finalizer is removed
		assert.True(t, patched)
	})

	t.Run("PostDelete_HookIsDeleted", func(t *testing.T) {
		app := newFakeApp()
		app.SetPostDeleteFinalizer("cleanup")
		app.Spec.Destination.Namespace = test.FakeArgoCDNamespace
		liveRoleBinding := &unstructured.Unstructured{Object: newFakeRoleBinding()}
		liveRole := &unstructured.Unstructured{Object: newFakeRole()}
		liveServiceAccount := &unstructured.Unstructured{Object: newFakeServiceAccount()}
		liveHook := &unstructured.Unstructured{Object: newFakePostDeleteHook()}
		conditions := []any{
			map[string]any{
				"type":   "Complete",
				"status": "True",
			},
		}
		require.NoError(t, unstructured.SetNestedField(liveHook.Object, conditions, "status", "conditions"))
		ctrl := newFakeController(t.Context(), &fakeData{
			manifestResponses: []*apiclient.ManifestResponse{{
				Manifests: []string{fakeRoleBinding, fakeRole, fakeServiceAccount, fakePostDeleteHook},
			}},
			apps: []runtime.Object{app, &defaultProj},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
				kube.GetResourceKey(liveRoleBinding):    liveRoleBinding,
				kube.GetResourceKey(liveRole):           liveRole,
				kube.GetResourceKey(liveServiceAccount): liveServiceAccount,
				kube.GetResourceKey(liveHook):           liveHook,
			},
		}, nil)

		patched := false
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		defaultReactor := fakeAppCs.ReactionChain[0]
		fakeAppCs.ReactionChain = nil
		fakeAppCs.AddReactor("get", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return defaultReactor.React(action)
		})
		fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			patched = true
			return true, &v1alpha1.Application{}, nil
		})
		err := ctrl.finalizeApplicationDeletion(app, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})
		require.NoError(t, err)
		// post-delete hooks are deleted
		require.Len(t, ctrl.kubectl.(*MockKubectl).DeletedResources, 4)
		deletedResources := []string{}
		for _, res := range ctrl.kubectl.(*MockKubectl).DeletedResources {
			deletedResources = append(deletedResources, res.Name)
		}
		expectedNames := []string{"hook-rolebinding", "hook-role", "hook-serviceaccount", "post-delete-hook"}
		require.ElementsMatch(t, expectedNames, deletedResources, "Deleted resources should match expected names")
		// finalizer is not removed
		assert.False(t, patched)
	})
}

func TestFinalizeAppDeletionWithImpersonation(t *testing.T) {
	type fixture struct {
		application *v1alpha1.Application
		controller  *ApplicationController
	}

	setup := func(destinationNamespace, serviceAccountName string) *fixture {
		app := newFakeApp()
		app.Status.OperationState = nil
		app.Status.History = nil
		now := metav1.Now()
		app.DeletionTimestamp = &now

		project := &v1alpha1.AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: test.FakeArgoCDNamespace,
				Name:      "default",
			},
			Spec: v1alpha1.AppProjectSpec{
				SourceRepos: []string{"*"},
				Destinations: []v1alpha1.ApplicationDestination{
					{
						Server:    "*",
						Namespace: "*",
					},
				},
				DestinationServiceAccounts: []v1alpha1.ApplicationDestinationServiceAccount{
					{
						Server:                "https://localhost:6443",
						Namespace:             destinationNamespace,
						DefaultServiceAccount: serviceAccountName,
					},
				},
			},
		}

		additionalObjs := []runtime.Object{}
		if serviceAccountName != "" {
			syncServiceAccount := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      serviceAccountName,
					Namespace: test.FakeDestNamespace,
				},
			}
			additionalObjs = append(additionalObjs, syncServiceAccount)
		}

		data := fakeData{
			apps: []runtime.Object{app, project},
			manifestResponse: &apiclient.ManifestResponse{
				Manifests: []string{},
				Namespace: test.FakeDestNamespace,
				Server:    "https://localhost:6443",
				Revision:  "abc123",
			},
			managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{},
			configMapData: map[string]string{
				"application.sync.impersonation.enabled": strconv.FormatBool(true),
			},
			additionalObjs: additionalObjs,
		}
		ctrl := newFakeController(t.Context(), &data, nil)
		return &fixture{
			application: app,
			controller:  ctrl,
		}
	}

	t.Run("no matching impersonation service account is configured", func(t *testing.T) {
		// given impersonation is enabled but no matching service account exists
		f := setup(test.FakeDestNamespace, "")

		// when
		err := f.controller.finalizeApplicationDeletion(f.application, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})

		// then deletion should fail due to impersonation error
		require.Error(t, err)
		assert.Contains(t, err.Error(), "error deriving service account to impersonate")
	})

	t.Run("valid impersonation service account is configured", func(t *testing.T) {
		// given impersonation is enabled with valid service account
		f := setup(test.FakeDestNamespace, "test-sa")

		// when
		err := f.controller.finalizeApplicationDeletion(f.application, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})

		// then deletion should succeed
		require.NoError(t, err)
	})

	t.Run("invalid application destination cluster", func(t *testing.T) {
		// given impersonation is enabled but destination cluster does not exist
		f := setup(test.FakeDestNamespace, "test-sa")
		f.application.Spec.Destination.Server = "https://invalid-cluster:6443"
		f.application.Spec.Destination.Name = "invalid"

		// when
		err := f.controller.finalizeApplicationDeletion(f.application, func(_ string) ([]*v1alpha1.Cluster, error) {
			return []*v1alpha1.Cluster{}, nil
		})

		// then deletion should still succeed by removing finalizers
		require.NoError(t, err)
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
		ctrl := newFakeController(t.Context(), &data, nil)
		key, _ := cache.MetaNamespaceKeyFunc(app)
		ctrl.appRefreshQueue.AddRateLimited(key)
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		fakeAppCs.ReactionChain = nil
		normalized := false
		fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if patchAction, ok := action.(kubetesting.PatchAction); ok {
				if string(patchAction.GetPatch()) == `{"spec":{"project":"default"}}` {
					normalized = true
				}
			}
			return true, &v1alpha1.Application{}, nil
		})
		ctrl.processAppRefreshQueueItem()
		assert.True(t, normalized)
	}

	{
		// Verify we don't unnecessarily normalize app when project is set
		app.Spec.Project = "default"
		data.apps[0] = app
		ctrl := newFakeController(t.Context(), &data, nil)
		key, _ := cache.MetaNamespaceKeyFunc(app)
		ctrl.appRefreshQueue.AddRateLimited(key)
		fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
		fakeAppCs.ReactionChain = nil
		normalized := false
		fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if patchAction, ok := action.(kubetesting.PatchAction); ok {
				if string(patchAction.GetPatch()) == `{"spec":{"project":"default"},"status":{"sync":{"comparedTo":{"destination":{},"source":{"repoURL":""}}}}}` {
					normalized = true
				}
			}
			return true, &v1alpha1.Application{}, nil
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, proj}}, nil)

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

	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app1, app2, proj}}, nil)

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
		Health: &v1alpha1.HealthStatus{
			Status: health.HealthStatusMissing,
		},
	}
	orphanedDeploy1 := v1alpha1.ResourceNode{
		ResourceRef: v1alpha1.ResourceRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "deploy1"},
	}
	orphanedDeploy2 := v1alpha1.ResourceNode{
		ResourceRef: v1alpha1.ResourceRef{Group: "apps", Kind: "Deployment", Namespace: "default", Name: "deploy2"},
	}

	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, proj},
		namespacedResources: map[kube.ResourceKey]namespacedResource{
			kube.NewResourceKey("apps", "Deployment", "default", "nginx-deployment"): {ResourceNode: managedDeploy},
			kube.NewResourceKey("apps", "Deployment", "default", "deploy1"):          {ResourceNode: orphanedDeploy1},
			kube.NewResourceKey("apps", "Deployment", "default", "deploy2"):          {ResourceNode: orphanedDeploy2},
		},
	}, nil)
	tree, err := ctrl.getResourceTree(&v1alpha1.Cluster{Server: "https://localhost:6443", Name: "fake-cluster"}, app, []*v1alpha1.ResourceDiff{{
		Namespace:   "default",
		Name:        "nginx-deployment",
		Kind:        "Deployment",
		Group:       "apps",
		LiveState:   "null",
		TargetState: test.DeploymentManifest,
	}})

	require.NoError(t, err)
	assert.Equal(t, []v1alpha1.ResourceNode{managedDeploy}, tree.Nodes)
	assert.Equal(t, []v1alpha1.ResourceNode{orphanedDeploy1, orphanedDeploy2}, tree.OrphanedNodes)
}

func TestSetOperationStateOnDeletedApp(t *testing.T) {
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	patched := false
	fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, &v1alpha1.Application{}, apierrors.NewNotFound(schema.GroupResource{}, "my-app")
	})
	ctrl.setOperationState(newFakeApp(), &v1alpha1.OperationState{Phase: synccommon.OperationSucceeded})
	assert.True(t, patched)
}

func TestSetOperationStateLogRetries(t *testing.T) {
	hook := utilTest.LogHook{}
	logrus.AddHook(&hook)
	t.Cleanup(func() {
		logrus.StandardLogger().ReplaceHooks(logrus.LevelHooks{})
	})
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	patched := false
	fakeAppCs.AddReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if !patched {
			patched = true
			return true, &v1alpha1.Application{}, errors.New("fake error")
		}
		return true, &v1alpha1.Application{}, nil
	})
	ctrl.setOperationState(newFakeApp(), &v1alpha1.OperationState{Phase: synccommon.OperationSucceeded})
	assert.True(t, patched)
	require.GreaterOrEqual(t, len(hook.Entries), 1)
	entry := hook.Entries[0]
	require.Contains(t, entry.Data, "error")
	errorVal, ok := entry.Data["error"].(error)
	require.True(t, ok, "error field should be of type error")
	assert.Contains(t, errorVal.Error(), "fake error")
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

			ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)

			t.Run("no need to refresh just reconciled application", func(t *testing.T) {
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)
			})

			t.Run("requested refresh is respected", func(t *testing.T) {
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)

				// use a one-off controller so other tests don't have a manual refresh request
				ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)

				// refresh app using the 'deepest' requested comparison level
				ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)
				ctrl.requestAppRefresh(app.Name, ComparisonWithNothing.Pointer(), nil)

				needRefresh, refreshType, compareWith := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.True(t, needRefresh)
				assert.Equal(t, v1alpha1.RefreshTypeNormal, refreshType)
				assert.Equal(t, CompareWithRecent, compareWith)
			})

			t.Run("requesting refresh with delay gives correct compression level", func(t *testing.T) {
				needRefresh, _, _ := ctrl.needRefreshAppStatus(app, 1*time.Hour, 2*time.Hour)
				assert.False(t, needRefresh)

				// use a one-off controller so other tests don't have a manual refresh request
				ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)

				// refresh app with a non-nil delay
				// use zero-second delay to test the add later logic without waiting in the test
				delay := time.Duration(0)
				ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), &delay)

				ctrl.processAppComparisonTypeQueueItem()
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
				ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)

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
				ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)

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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{}}, nil)
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
		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}}, nil)

		_, hasErrors := ctrl.refreshAppConditions(app)
		assert.False(t, hasErrors)
		assert.Empty(t, app.Status.Conditions)
	})

	t.Run("PreserveExistingWarningCondition", func(t *testing.T) {
		app := newFakeApp()
		app.Status.SetConditions([]v1alpha1.ApplicationCondition{{Type: v1alpha1.ApplicationConditionExcludedResourceWarning}}, nil)

		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}}, nil)

		_, hasErrors := ctrl.refreshAppConditions(app)
		assert.False(t, hasErrors)
		assert.Len(t, app.Status.Conditions, 1)
		assert.Equal(t, v1alpha1.ApplicationConditionExcludedResourceWarning, app.Status.Conditions[0].Type)
	})

	t.Run("ReplacesSpecErrorCondition", func(t *testing.T) {
		app := newFakeApp()
		app.Spec.Project = "wrong project"
		app.Status.SetConditions([]v1alpha1.ApplicationCondition{{Type: v1alpha1.ApplicationConditionInvalidSpecError, Message: "old message"}}, nil)

		ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &defaultProj}}, nil)

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
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, nil)
	key, _ := cache.MetaNamespaceKeyFunc(app)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	receivedPatch := map[string]any{}
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	t.Run("UpdatedOnFullReconciliation", func(t *testing.T) {
		receivedPatch = map[string]any{}
		ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
		ctrl.appRefreshQueue.AddRateLimited(key)

		ctrl.processAppRefreshQueueItem()

		_, updated, err := unstructured.NestedString(receivedPatch, "status", "reconciledAt")
		require.NoError(t, err)
		assert.True(t, updated)

		_, updated, err = unstructured.NestedString(receivedPatch, "status", "observedAt")
		require.NoError(t, err)
		assert.False(t, updated)
	})

	t.Run("NotUpdatedOnPartialReconciliation", func(t *testing.T) {
		receivedPatch = map[string]any{}
		ctrl.appRefreshQueue.AddRateLimited(key)
		ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)

		ctrl.processAppRefreshQueueItem()

		_, updated, err := unstructured.NestedString(receivedPatch, "status", "reconciledAt")
		require.NoError(t, err)
		assert.False(t, updated)

		_, updated, err = unstructured.NestedString(receivedPatch, "status", "observedAt")
		require.NoError(t, err)
		assert.False(t, updated)
	})
}

func TestUpdateHealthStatusTransitionTime(t *testing.T) {
	deployment := kube.MustToUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
	})
	testCases := []struct {
		name           string
		app            *v1alpha1.Application
		configMapData  map[string]string
		expectedStatus health.HealthStatusCode
	}{
		{
			name: "Degraded to Missing",
			app:  newFakeAppWithHealthAndTime(health.HealthStatusDegraded, testTimestamp),
			configMapData: map[string]string{
				"resource.customizations": `
apps/Deployment:
  health.lua: |
    hs = {}
    hs.status = "Missing"
    hs.message = ""
    return hs`,
			},
			expectedStatus: health.HealthStatusMissing,
		},
		{
			name: "Missing to Progressing",
			app:  newFakeAppWithHealthAndTime(health.HealthStatusMissing, testTimestamp),
			configMapData: map[string]string{
				"resource.customizations": `
apps/Deployment:
  health.lua: |
    hs = {}
    hs.status = "Progressing"
    hs.message = ""
    return hs`,
			},
			expectedStatus: health.HealthStatusProgressing,
		},
		{
			name: "Progressing to Healthy",
			app:  newFakeAppWithHealthAndTime(health.HealthStatusProgressing, testTimestamp),
			configMapData: map[string]string{
				"resource.customizations": `
apps/Deployment:
  health.lua: |
    hs = {}
    hs.status = "Healthy"
    hs.message = ""
    return hs`,
			},
			expectedStatus: health.HealthStatusHealthy,
		},
		{
			name: "Healthy  to Degraded",
			app:  newFakeAppWithHealthAndTime(health.HealthStatusHealthy, testTimestamp),
			configMapData: map[string]string{
				"resource.customizations": `
apps/Deployment:
  health.lua: |
    hs = {}
    hs.status = "Degraded"
    hs.message = ""
    return hs`,
			},
			expectedStatus: health.HealthStatusDegraded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := newFakeController(t.Context(), &fakeData{
				apps: []runtime.Object{tc.app, &defaultProj},
				manifestResponse: &apiclient.ManifestResponse{
					Manifests: []string{},
					Namespace: test.FakeDestNamespace,
					Server:    test.FakeClusterURL,
					Revision:  "abc123",
				},
				managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
					kube.GetResourceKey(deployment): deployment,
				},
				configMapData: tc.configMapData,
			}, nil)

			ctrl.processAppRefreshQueueItem()
			apps, err := ctrl.appLister.List(labels.Everything())
			require.NoError(t, err)
			assert.NotEmpty(t, apps)
			assert.Equal(t, tc.expectedStatus, apps[0].Status.Health.Status)
			assert.NotEqual(t, testTimestamp, *apps[0].Status.Health.LastTransitionTime)
		})
	}
}

func TestUpdateHealthStatusProgression(t *testing.T) {
	app := newFakeAppWithHealthAndTime(health.HealthStatusDegraded, testTimestamp)
	deployment := kube.MustToUnstructured(&appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
		Status: appsv1.DeploymentStatus{
			ObservedGeneration: 0,
		},
	})
	configMapData := map[string]string{
		"resource.customizations": `
apps/Deployment:
  health.lua: |
    hs = {}
    hs.status = ""
    hs.message = ""

    if obj.metadata ~= nil then
      if obj.metadata.labels ~= nil then
        current_status = obj.metadata.labels["status"]
        if current_status == "Degraded" then
          hs.status = "Missing"
        elseif current_status == "Missing" then
          hs.status = "Progressing"
        elseif current_status == "Progressing" then
          hs.status = "Healthy"
        elseif current_status == "Healthy" then
          hs.status = "Degraded"
        end
      end
    end

    return hs`,
	}
	ctrl := newFakeControllerWithResync(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(deployment): deployment,
		},
		configMapData: configMapData,
		manifestResponses: []*apiclient.ManifestResponse{
			{},
			{},
			{},
			{},
		},
	}, time.Millisecond*10, nil, nil)

	testCases := []struct {
		name           string
		initialStatus  string
		expectedStatus health.HealthStatusCode
	}{
		{
			name:           "Degraded to Missing",
			initialStatus:  "Degraded",
			expectedStatus: health.HealthStatusMissing,
		},
		{
			name:           "Missing to Progressing",
			initialStatus:  "Missing",
			expectedStatus: health.HealthStatusProgressing,
		},
		{
			name:           "Progressing to Healthy",
			initialStatus:  "Progressing",
			expectedStatus: health.HealthStatusHealthy,
		},
		{
			name:           "Healthy to Degraded",
			initialStatus:  "Healthy",
			expectedStatus: health.HealthStatusDegraded,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			deployment.SetLabels(map[string]string{"status": tc.initialStatus})
			ctrl.processAppRefreshQueueItem()
			apps, err := ctrl.appLister.List(labels.Everything())
			require.NoError(t, err)
			if assert.NotEmpty(t, apps) {
				assert.Equal(t, tc.expectedStatus, apps[0].Status.Health.Status)
				assert.NotEqual(t, testTimestamp, *apps[0].Status.Health.LastTransitionTime)
			}

			ctrl.requestAppRefresh(app.Name, nil, nil)
			time.Sleep(time.Millisecond * 15)
		})
	}
}

func TestProjectErrorToCondition(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "wrong project"
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}, nil)
	key, _ := cache.MetaNamespaceKeyFunc(app)
	ctrl.appRefreshQueue.AddRateLimited(key)
	ctrl.requestAppRefresh(app.Name, CompareWithRecent.Pointer(), nil)

	ctrl.processAppRefreshQueueItem()

	obj, ok, err := ctrl.appInformer.GetIndexer().GetByKey(key)
	assert.True(t, ok)
	require.NoError(t, err)
	updatedApp := obj.(*v1alpha1.Application)
	assert.Equal(t, v1alpha1.ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[0].Type)
	assert.Equal(t, "Application referencing project wrong project which does not exist", updatedApp.Status.Conditions[0].Message)
	assert.Equal(t, v1alpha1.ApplicationConditionInvalidSpecError, updatedApp.Status.Conditions[0].Type)
}

func TestFinalizeProjectDeletion_HasApplications(t *testing.T) {
	app := newFakeApp()
	proj := &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: test.FakeArgoCDNamespace}}
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, proj}}, nil)

	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	patched := false
	fakeAppCs.PrependReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, &v1alpha1.Application{}, nil
	})

	err := ctrl.finalizeProjectDeletion(proj)
	require.NoError(t, err)
	assert.False(t, patched)
}

func TestFinalizeProjectDeletion_DoesNotHaveApplications(t *testing.T) {
	proj := &v1alpha1.AppProject{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: test.FakeArgoCDNamespace}}
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{&defaultProj}}, nil)

	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.AppProject{}, nil
	})

	err := ctrl.finalizeProjectDeletion(proj)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"metadata": map[string]any{
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Equal(t, string(synccommon.OperationError), phase)
	assert.Equal(t, "Failed to load application project: error getting app project \"default\": appproject.argoproj.io \"default\" not found", message)
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app, &proj}}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	func() {
		fakeAppCs.Lock()
		defer fakeAppCs.Unlock()
		fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			if patchAction, ok := action.(kubetesting.PatchAction); ok {
				require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
			}
			return true, &v1alpha1.Application{}, nil
		})
	}()

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Equal(t, string(synccommon.OperationError), phase)
	assert.Contains(t, message, "application destination can't have both name and server defined: another-cluster https://localhost:6443")
}

func TestProcessRequestedAppOperation_FailedHasRetries(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "invalid-project"
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 1},
	}
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	retryCount, _, _ := unstructured.NestedFloat64(receivedPatch, "status", "operationState", "retryCount")
	assert.Equal(t, string(synccommon.OperationRunning), phase)
	assert.Contains(t, message, "Failed to load application project: error getting app project \"invalid-project\": appproject.argoproj.io \"invalid-project\" not found. Retrying attempt #1")
	assert.InEpsilon(t, float64(1), retryCount, 0.0001)
}

func TestProcessRequestedAppOperation_RunningPreviouslyFailed(t *testing.T) {
	failedAttemptFinisedAt := time.Now().Add(-time.Minute * 5)
	app := newFakeApp()
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 1},
	}
	app.Status.OperationState.Operation = *app.Operation
	app.Status.OperationState.Phase = synccommon.OperationRunning
	app.Status.OperationState.RetryCount = 1
	app.Status.OperationState.FinishedAt = &metav1.Time{Time: failedAttemptFinisedAt}
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
	ctrl := newFakeController(t.Context(), data, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	finishedAtStr, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "finishedAt")
	finishedAt, err := time.Parse(time.RFC3339, finishedAtStr)
	require.NoError(t, err)
	assert.Equal(t, string(synccommon.OperationSucceeded), phase)
	assert.Equal(t, "successfully synced (no more tasks)", message)
	assert.Truef(t, finishedAt.After(failedAttemptFinisedAt), "finishedAt was expected to be updated. The retry was not performed.")
}

func TestProcessRequestedAppOperation_RunningPreviouslyFailedBackoff(t *testing.T) {
	failedAttemptFinisedAt := time.Now().Add(-time.Second)
	app := newFakeApp()
	app.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{
			Limit: 1,
			Backoff: &v1alpha1.Backoff{
				Duration:    "1h",
				Factor:      ptr.To(int64(100)),
				MaxDuration: "1h",
			},
		},
	}
	app.Status.OperationState.Operation = *app.Operation
	app.Status.OperationState.Phase = synccommon.OperationRunning
	app.Status.OperationState.Message = "pending retry"
	app.Status.OperationState.RetryCount = 1
	app.Status.OperationState.FinishedAt = &metav1.Time{Time: failedAttemptFinisedAt}
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
	ctrl := newFakeController(t.Context(), data, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.PrependReactor("patch", "*", func(_ kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		require.FailNow(t, "A patch should not have been called if the backoff has not passed")
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)
}

func TestProcessRequestedAppOperation_HasRetriesTerminated(t *testing.T) {
	app := newFakeApp()
	app.Operation = &v1alpha1.Operation{
		Sync:  &v1alpha1.SyncOperation{},
		Retry: v1alpha1.RetryStrategy{Limit: 10},
	}
	app.Status.OperationState.Operation = *app.Operation
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
	ctrl := newFakeController(t.Context(), data, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Equal(t, string(synccommon.OperationFailed), phase)
	assert.Equal(t, "Operation terminated", message)
}

func TestProcessRequestedAppOperation_Successful(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "default"
	app.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponses: []*apiclient.ManifestResponse{{
			Manifests: []string{},
		}},
	}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Equal(t, string(synccommon.OperationSucceeded), phase)
	assert.Equal(t, "successfully synced (no more tasks)", message)
	ok, level := ctrl.isRefreshRequested(ctrl.toAppKey(app.Name))
	assert.True(t, ok)
	assert.Equal(t, CompareWithLatestForceResolve, level)
}

func TestProcessRequestedAppAutomatedOperation_Successful(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "default"
	app.Operation = &v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
		InitiatedBy: v1alpha1.OperationInitiator{
			Automated: true,
		},
	}
	ctrl := newFakeController(t.Context(), &fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponses: []*apiclient.ManifestResponse{{
			Manifests: []string{},
		}},
	}, nil)
	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	receivedPatch := map[string]any{}
	fakeAppCs.PrependReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &receivedPatch))
		}
		return true, &v1alpha1.Application{}, nil
	})

	ctrl.processRequestedAppOperation(app)

	phase, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "phase")
	message, _, _ := unstructured.NestedString(receivedPatch, "status", "operationState", "message")
	assert.Equal(t, string(synccommon.OperationSucceeded), phase)
	assert.Equal(t, "successfully synced (no more tasks)", message)
	ok, level := ctrl.isRefreshRequested(ctrl.toAppKey(app.Name))
	assert.True(t, ok)
	assert.Equal(t, CompareWithLatest, level)
}

func TestProcessRequestedAppOperation_SyncTimeout(t *testing.T) {
	testCases := []struct {
		name            string
		startedSince    time.Duration
		syncTimeout     time.Duration
		retryAttempt    int
		currentPhase    synccommon.OperationPhase
		expectedPhase   synccommon.OperationPhase
		expectedMessage string
	}{{
		name:            "Continue when running operation has not exceeded timeout",
		syncTimeout:     time.Minute,
		startedSince:    30 * time.Second,
		currentPhase:    synccommon.OperationRunning,
		expectedPhase:   synccommon.OperationSucceeded,
		expectedMessage: "successfully synced (no more tasks)",
	}, {
		name:            "Continue when terminating operation has exceeded timeout",
		syncTimeout:     time.Minute,
		startedSince:    2 * time.Minute,
		currentPhase:    synccommon.OperationTerminating,
		expectedPhase:   synccommon.OperationFailed,
		expectedMessage: "Operation terminated",
	}, {
		name:            "Terminate when running operation exceeded timeout",
		syncTimeout:     time.Minute,
		startedSince:    2 * time.Minute,
		currentPhase:    synccommon.OperationRunning,
		expectedPhase:   synccommon.OperationFailed,
		expectedMessage: "Operation terminated, triggered by controller sync timeout",
	}, {
		name:            "Terminate when retried operation exceeded timeout",
		syncTimeout:     time.Minute,
		startedSince:    15 * time.Minute,
		currentPhase:    synccommon.OperationRunning,
		retryAttempt:    1,
		expectedPhase:   synccommon.OperationFailed,
		expectedMessage: "Operation terminated, triggered by controller sync timeout (retried 1 times).",
	}}
	for i := range testCases {
		tc := testCases[i]
		t.Run(fmt.Sprintf("case %d: %s", i, tc.name), func(t *testing.T) {
			app := newFakeApp()
			app.Spec.Project = "default"
			app.Operation = &v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{
					Revision: "HEAD",
				},
			}
			ctrl := newFakeController(t.Context(), &fakeData{
				apps: []runtime.Object{app, &defaultProj},
				manifestResponses: []*apiclient.ManifestResponse{{
					Manifests: []string{},
				}},
			}, nil)

			ctrl.syncTimeout = tc.syncTimeout
			app.Status.OperationState = &v1alpha1.OperationState{
				Operation: *app.Operation,
				Phase:     tc.currentPhase,
				StartedAt: metav1.NewTime(time.Now().Add(-tc.startedSince)),
			}
			if tc.retryAttempt > 0 {
				app.Status.OperationState.FinishedAt = ptr.To(metav1.NewTime(time.Now().Add(-tc.startedSince)))
				app.Status.OperationState.RetryCount = int64(tc.retryAttempt)
			}

			ctrl.processRequestedAppOperation(app)

			app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.ObjectMeta.Namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, tc.expectedPhase, app.Status.OperationState.Phase)
			assert.Equal(t, tc.expectedMessage, app.Status.OperationState.Message)
		})
	}
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
		configMapData: map[string]string{
			"application.allowedNodeLabels": "label1,label2",
		},
	}
	ctrl := newFakeController(t.Context(), data, nil)
	mockStateCache := &mockstatecache.LiveStateCache{}
	mockStateCache.EXPECT().IterateResources(mock.Anything, mock.MatchedBy(func(callback func(res *clustercache.Resource, info *statecache.ResourceInfo)) bool {
		// node resource
		callback(&clustercache.Resource{
			Ref: corev1.ObjectReference{Name: "minikube", Kind: "Node", APIVersion: "v1"},
		}, &statecache.ResourceInfo{NodeInfo: &statecache.NodeInfo{
			Name:       "minikube",
			SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
			Capacity:   map[corev1.ResourceName]resource.Quantity{corev1.ResourceCPU: resource.MustParse("5")},
			Labels:     map[string]string{"label1": "value1", "label2": "value2"},
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
	})).Return(nil).Maybe()
	ctrl.stateCache = mockStateCache

	hosts, err := ctrl.getAppHosts(&v1alpha1.Cluster{Server: "test", Name: "test"}, app, []v1alpha1.ResourceNode{{
		ResourceRef: v1alpha1.ResourceRef{Name: "pod1", Namespace: "default", Kind: kube.PodKind},
		Info: []v1alpha1.InfoItem{{
			Name:  "Host",
			Value: "Minikube",
		}},
	}})

	require.NoError(t, err)
	assert.Equal(t, []v1alpha1.HostInfo{{
		Name:       "minikube",
		SystemInfo: corev1.NodeSystemInfo{OSImage: "debian"},
		ResourcesInfo: []v1alpha1.HostResourceInfo{
			{
				ResourceName: corev1.ResourceCPU, Capacity: 5000, RequestedByApp: 1000, RequestedByNeighbors: 2000,
			},
		},
		Labels: map[string]string{"label1": "value1", "label2": "value2"},
	}}, hosts)
}

func TestMetricsExpiration(t *testing.T) {
	app := newFakeApp()
	// Check expiration is disabled by default
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	assert.False(t, ctrl.metricsServer.HasExpiration())
	// Check expiration is enabled if set
	ctrl = newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}, metricsCacheExpiration: 10 * time.Second}, nil)
	assert.True(t, ctrl.metricsServer.HasExpiration())
}

func TestToAppKey(t *testing.T) {
	ctrl := newFakeController(t.Context(), &fakeData{}, nil)
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
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
		canProcess := ctrl.canProcessApp(app)
		assert.True(t, canProcess)
	})
	t.Run("with cluster filter, bad namespace", func(t *testing.T) {
		app.Namespace = "bad"
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
	ctrl := newFakeController(t.Context(), &fakeData{}, nil)
	tests := []struct {
		name     string
		input    any
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
	ctrl := newFakeController(t.Context(), &fakeData{apps: []runtime.Object{app}}, nil)
	cm := newFakeCM()
	t.Run("without delete option object is deleted", func(t *testing.T) {
		cmObj := kube.MustToUnstructured(&cm)
		assert.True(t, ctrl.shouldBeDeleted(app, cmObj))
	})
	t.Run("with delete set to false object is retained", func(t *testing.T) {
		cmObj := kube.MustToUnstructured(&cm)
		cmObj.SetAnnotations(map[string]string{"argocd.argoproj.io/sync-options": "Delete=false"})
		assert.False(t, ctrl.shouldBeDeleted(app, cmObj))
	})
	t.Run("with delete set to false object is retained", func(t *testing.T) {
		cmObj := kube.MustToUnstructured(&cm)
		cmObj.SetAnnotations(map[string]string{"helm.sh/resource-policy": "keep"})
		assert.False(t, ctrl.shouldBeDeleted(app, cmObj))
	})
}

func TestAddControllerNamespace(t *testing.T) {
	t.Run("set controllerNamespace when the app is in the controller namespace", func(t *testing.T) {
		app := newFakeApp()
		ctrl := newFakeController(t.Context(), &fakeData{
			apps:             []runtime.Object{app, &defaultProj},
			manifestResponse: &apiclient.ManifestResponse{},
		}, nil)

		ctrl.processAppRefreshQueueItem()

		updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(t.Context(), app.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, test.FakeArgoCDNamespace, updatedApp.Status.ControllerNamespace)
	})
	t.Run("set controllerNamespace when the app is in another namespace than the controller", func(t *testing.T) {
		appNamespace := "app-namespace"

		app := newFakeApp()
		app.Namespace = appNamespace
		proj := defaultProj
		proj.Spec.SourceNamespaces = []string{appNamespace}
		ctrl := newFakeController(t.Context(), &fakeData{
			apps:                  []runtime.Object{app, &proj},
			manifestResponse:      &apiclient.ManifestResponse{},
			applicationNamespaces: []string{appNamespace},
		}, nil)

		ctrl.processAppRefreshQueueItem()

		updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(appNamespace).Get(t.Context(), app.Name, metav1.GetOptions{})
		require.NoError(t, err)
		assert.Equal(t, test.FakeArgoCDNamespace, updatedApp.Status.ControllerNamespace)
	})
}

func TestHelmValuesObjectHasReplaceStrategy(t *testing.T) {
	app := v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{Sync: v1alpha1.SyncStatus{ComparedTo: v1alpha1.ComparedTo{
			Source: v1alpha1.ApplicationSource{
				Helm: &v1alpha1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						Object: &unstructured.Unstructured{Object: map[string]any{"key": []string{"value"}}},
					},
				},
			},
		}}},
	}

	appModified := v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{Sync: v1alpha1.SyncStatus{ComparedTo: v1alpha1.ComparedTo{
			Source: v1alpha1.ApplicationSource{
				Helm: &v1alpha1.ApplicationSourceHelm{
					ValuesObject: &runtime.RawExtension{
						Object: &unstructured.Unstructured{Object: map[string]any{"key": []string{"value-modified1"}}},
					},
				},
			},
		}}},
	}

	patch, _, err := createMergePatch(
		app,
		appModified)
	require.NoError(t, err)
	assert.JSONEq(t, `{"status":{"sync":{"comparedTo":{"source":{"helm":{"valuesObject":{"key":["value-modified1"]}}}}}}}`, string(patch))
}

func TestAppStatusIsReplaced(t *testing.T) {
	original := &v1alpha1.ApplicationStatus{Sync: v1alpha1.SyncStatus{
		ComparedTo: v1alpha1.ComparedTo{
			Destination: v1alpha1.ApplicationDestination{
				Server: "https://mycluster",
			},
		},
	}}

	updated := &v1alpha1.ApplicationStatus{Sync: v1alpha1.SyncStatus{
		ComparedTo: v1alpha1.ComparedTo{
			Destination: v1alpha1.ApplicationDestination{
				Name: "mycluster",
			},
		},
	}}

	patchData, ok, err := createMergePatch(original, updated)

	require.NoError(t, err)
	require.True(t, ok)
	patchObj := map[string]any{}
	require.NoError(t, json.Unmarshal(patchData, &patchObj))

	val, has, err := unstructured.NestedFieldNoCopy(patchObj, "sync", "comparedTo", "destination", "server")
	require.NoError(t, err)
	require.True(t, has)
	require.Nil(t, val)
}

func TestAlreadyAttemptSync(t *testing.T) {
	app := newFakeApp()
	defaultRevision := app.Status.OperationState.SyncResult.Revision

	t.Run("no operation state", func(t *testing.T) {
		app := app.DeepCopy()
		app.Status.OperationState = nil
		attempted, _, _ := alreadyAttemptedSync(app, []string{defaultRevision}, true)
		assert.False(t, attempted)
	})

	t.Run("no sync result for running sync", func(t *testing.T) {
		app := app.DeepCopy()
		app.Status.OperationState.SyncResult = nil
		app.Status.OperationState.Phase = synccommon.OperationRunning
		attempted, _, _ := alreadyAttemptedSync(app, []string{defaultRevision}, true)
		assert.False(t, attempted)
	})

	t.Run("no sync result for completed sync", func(t *testing.T) {
		app := app.DeepCopy()
		app.Status.OperationState.SyncResult = nil
		app.Status.OperationState.Phase = synccommon.OperationError
		attempted, _, _ := alreadyAttemptedSync(app, []string{defaultRevision}, true)
		assert.True(t, attempted)
	})

	t.Run("single source", func(t *testing.T) {
		t.Run("no revision", func(t *testing.T) {
			attempted, _, _ := alreadyAttemptedSync(app, []string{}, true)
			assert.False(t, attempted)
		})

		t.Run("empty revision", func(t *testing.T) {
			attempted, _, _ := alreadyAttemptedSync(app, []string{""}, true)
			assert.False(t, attempted)
		})

		t.Run("too many revision", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revision = "sha"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha", "sha2"}, true)
			assert.False(t, attempted)
		})

		t.Run("same manifest, same SHA with changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revision = "sha"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha"}, true)
			assert.True(t, attempted)
		})

		t.Run("same manifest, different SHA with changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revision = "sha1"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha2"}, true)
			assert.False(t, attempted)
		})

		t.Run("same manifest, different SHA without changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revision = "sha1"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha2"}, false)
			assert.True(t, attempted)
		})

		t.Run("different manifest, same SHA with changes", func(t *testing.T) {
			// This test represents the case where the user changed a source's target revision to a new branch, but it
			// points to the same revision as the old branch. We currently do not consider this as having been "already
			// attempted." In the future we may want to short-circuit the auto-sync in these cases.
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Source = v1alpha1.ApplicationSource{TargetRevision: "branch1"}
			app.Spec.Source = &v1alpha1.ApplicationSource{TargetRevision: "branch2"}
			app.Status.OperationState.SyncResult.Revision = "sha"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha"}, true)
			assert.False(t, attempted)
		})

		t.Run("different manifest, different SHA with changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Source = v1alpha1.ApplicationSource{Path: "folder1"}
			app.Spec.Source = &v1alpha1.ApplicationSource{Path: "folder2"}
			app.Status.OperationState.SyncResult.Revision = "sha1"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha2"}, true)
			assert.False(t, attempted)
		})

		t.Run("different manifest, different SHA without changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Source = v1alpha1.ApplicationSource{Path: "folder1"}
			app.Spec.Source = &v1alpha1.ApplicationSource{Path: "folder2"}
			app.Status.OperationState.SyncResult.Revision = "sha1"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha2"}, false)
			assert.False(t, attempted)
		})

		t.Run("different manifest, same SHA without changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Source = v1alpha1.ApplicationSource{Path: "folder1"}
			app.Spec.Source = &v1alpha1.ApplicationSource{Path: "folder2"}
			app.Status.OperationState.SyncResult.Revision = "sha"
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha"}, false)
			assert.False(t, attempted)
		})
	})

	t.Run("multi-source", func(t *testing.T) {
		app := app.DeepCopy()
		app.Status.OperationState.SyncResult.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder2"}}
		app.Spec.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder2"}}

		t.Run("same manifest, same SHAs with changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a", "sha_b"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a", "sha_b"}, true)
			assert.True(t, attempted)
		})

		t.Run("same manifest, different SHAs with changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a_=", "sha_b_1"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a_2", "sha_b_2"}, true)
			assert.False(t, attempted)
		})

		t.Run("same manifest, different SHA without changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a_=", "sha_b_1"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a_2", "sha_b_2"}, false)
			assert.True(t, attempted)
		})

		t.Run("different manifest, same SHA with changes", func(t *testing.T) {
			// This test represents the case where the user changed a source's target revision to a new branch, but it
			// points to the same revision as the old branch. We currently do not consider this as having been "already
			// attempted." In the future we may want to short-circuit the auto-sync in these cases.
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Sources = []v1alpha1.ApplicationSource{{TargetRevision: "branch1"}, {TargetRevision: "branch2"}}
			app.Spec.Sources = []v1alpha1.ApplicationSource{{TargetRevision: "branch1"}, {TargetRevision: "branch3"}}
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a_2", "sha_b_2"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a_2", "sha_b_2"}, false)
			assert.False(t, attempted)
		})

		t.Run("different manifest, different SHA with changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder2"}}
			app.Spec.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder3"}}
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a", "sha_b"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a", "sha_b_2"}, true)
			assert.False(t, attempted)
		})

		t.Run("different manifest, different SHA without changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder2"}}
			app.Spec.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder3"}}
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a", "sha_b"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a", "sha_b_2"}, false)
			assert.False(t, attempted)
		})

		t.Run("different manifest, same SHA without changes", func(t *testing.T) {
			app := app.DeepCopy()
			app.Status.OperationState.SyncResult.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder2"}}
			app.Spec.Sources = []v1alpha1.ApplicationSource{{Path: "folder1"}, {Path: "folder3"}}
			app.Status.OperationState.SyncResult.Revisions = []string{"sha_a", "sha_b"}
			attempted, _, _ := alreadyAttemptedSync(app, []string{"sha_a", "sha_b"}, false)
			assert.False(t, attempted)
		})
	})
}

func assertDurationAround(t *testing.T, expected time.Duration, actual time.Duration) {
	t.Helper()
	delta := time.Second / 2
	assert.GreaterOrEqual(t, expected, actual-delta)
	assert.LessOrEqual(t, expected, actual+delta)
}

func TestSelfHealRemainingBackoff(t *testing.T) {
	ctrl := newFakeController(t.Context(), &fakeData{}, nil)
	ctrl.selfHealBackoff = &wait.Backoff{
		Factor:   3,
		Duration: 2 * time.Second,
		Cap:      2 * time.Minute,
	}
	app := &v1alpha1.Application{
		Status: v1alpha1.ApplicationStatus{
			OperationState: &v1alpha1.OperationState{
				Operation: v1alpha1.Operation{
					Sync: &v1alpha1.SyncOperation{},
				},
			},
		},
	}

	testCases := []struct {
		attempts         int
		finishedAt       *metav1.Time
		expectedDuration time.Duration
		shouldSelfHeal   bool
	}{{
		attempts:         0,
		finishedAt:       ptr.To(metav1.Now()),
		expectedDuration: 0,
		shouldSelfHeal:   true,
	}, {
		attempts:         1,
		finishedAt:       ptr.To(metav1.Now()),
		expectedDuration: 2 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         2,
		finishedAt:       ptr.To(metav1.Now()),
		expectedDuration: 6 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         3,
		finishedAt:       nil,
		expectedDuration: 18 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         4,
		finishedAt:       nil,
		expectedDuration: 54 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         5,
		finishedAt:       nil,
		expectedDuration: 120 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         6,
		finishedAt:       nil,
		expectedDuration: 120 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         6,
		finishedAt:       ptr.To(metav1.Now()),
		expectedDuration: 120 * time.Second,
		shouldSelfHeal:   false,
	}, {
		attempts:         40,
		finishedAt:       &metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
		expectedDuration: 60 * time.Second,
		shouldSelfHeal:   false,
	}}

	for i := range testCases {
		tc := testCases[i]
		t.Run(fmt.Sprintf("test case %d", i), func(t *testing.T) {
			app.Status.OperationState.FinishedAt = tc.finishedAt
			duration := ctrl.selfHealRemainingBackoff(app, tc.attempts)
			shouldSelfHeal := duration <= 0
			require.Equal(t, tc.shouldSelfHeal, shouldSelfHeal)
			assertDurationAround(t, tc.expectedDuration, duration)
		})
	}
}

func TestConversionWebhookFailureIsolation(t *testing.T) {
	app1 := newFakeApp()
	app1.Name = "app1"

	app2 := newFakeApp()
	app2.Name = "app2"
	app2.Spec.Destination.Namespace = "different-namespace"

	// Create a conversion webhook error
	conversionError := errors.New("failed to sync cluster: failed to load initial state of resource BucketServerSideEncryptionConfiguration.s3.aws.upbound.io: conversion webhook for s3.aws.upbound.io/v1beta1, Kind=BucketServerSideEncryptionConfiguration failed: Post \"https://provider-aws-s3.crossplane-system.svc:9443/convert?timeout=30s\": no endpoints available for service \"provider-aws-s3\"")

	data := &fakeData{
		apps: []runtime.Object{app1, app2, &defaultProj},
		// Use manifestResponses to allow multiple calls
		manifestResponses: []*apiclient.ManifestResponse{
			{
				Manifests: []string{},
				Namespace: test.FakeDestNamespace,
				Server:    test.FakeClusterURL,
				Revision:  "abc123",
			},
			{
				Manifests: []string{},
				Namespace: test.FakeDestNamespace,
				Server:    test.FakeClusterURL,
				Revision:  "abc123",
			},
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}

	ctrl := newFakeController(t.Context(), data, nil)

	// Create a new mock state cache for conversion webhook failures
	customMockStateCache := &mockstatecache.LiveStateCache{}

	// Set up all the necessary methods for basic functionality
	customMockStateCache.On("IsNamespaced", mock.Anything, mock.Anything).Return(true, nil)
	customMockStateCache.On("GetVersionsInfo", mock.Anything).Return("v1.2.3", nil, nil)

	// Set up conversion webhook failures for specific operations
	customMockStateCache.On("GetManagedLiveObjs", mock.Anything, mock.Anything, mock.Anything).Return(nil, conversionError)
	customMockStateCache.On("GetNamespaceTopLevelResources", mock.Anything, mock.Anything).Return(nil, conversionError)
	customMockStateCache.On("IterateHierarchyV2", mock.Anything, mock.Anything, mock.Anything).Return(conversionError)
	customMockStateCache.On("IterateResources", mock.Anything, mock.Anything).Return(nil)

	// Set up cluster cache mock
	clusterCacheMock := &mocks.ClusterCache{}
	clusterCacheMock.EXPECT().IsNamespaced(mock.Anything).Return(true, nil)
	clusterCacheMock.EXPECT().GetOpenAPISchema().Return(nil, nil)
	clusterCacheMock.EXPECT().GetGVKParser().Return(nil)
	customMockStateCache.EXPECT().GetClusterCache(mock.Anything).Return(clusterCacheMock, nil)

	// Replace the state cache
	ctrl.stateCache = customMockStateCache
	ctrl.appStateManager.(*appStateManager).liveStateCache = customMockStateCache

	// Process queue items for both applications
	ctrl.processAppRefreshQueueItem() // app1
	ctrl.processAppRefreshQueueItem() // app2

	updatedApp1, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "app1", metav1.GetOptions{})
	require.NoError(t, err)

	updatedApp2, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get(context.Background(), "app2", metav1.GetOptions{})
	require.NoError(t, err)

	// Test that applications don't go to Unknown state due to conversion webhook failures
	assert.NotEqual(t, v1alpha1.SyncStatusCodeUnknown, updatedApp1.Status.Sync.Status,
		"App1 should not be in Unknown sync state due to conversion webhook failure")
	assert.NotEqual(t, v1alpha1.SyncStatusCodeUnknown, updatedApp2.Status.Sync.Status,
		"App2 should not be in Unknown sync state due to conversion webhook failure")
	assert.NotEqual(t, health.HealthStatusUnknown, updatedApp1.Status.Health.Status,
		"App1 should not be in Unknown health state due to conversion webhook failure")
	assert.NotEqual(t, health.HealthStatusUnknown, updatedApp2.Status.Health.Status,
		"App2 should not be in Unknown health state due to conversion webhook failure")

	// Verify that conversion webhook errors are properly handled and reported
	hasConversionError := false
	for _, app := range []*v1alpha1.Application{updatedApp1, updatedApp2} {
		for _, condition := range app.Status.Conditions {
			if condition.Type == v1alpha1.ApplicationConditionComparisonError &&
				strings.Contains(condition.Message, "conversion webhook") {
				hasConversionError = true
				break
			}
		}
	}

	// We expect the error to be reported, but applications should still function
	assert.True(t, hasConversionError, "Conversion webhook errors should be reported in application conditions")
}
