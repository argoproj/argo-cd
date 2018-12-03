package controller

import (
	"strings"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	mockstatecache "github.com/argoproj/argo-cd/controller/cache/mocks"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	mockreposerver "github.com/argoproj/argo-cd/reposerver/mocks"
	"github.com/argoproj/argo-cd/reposerver/repository"
	mockrepoclient "github.com/argoproj/argo-cd/reposerver/repository/mocks"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
)

type fakeData struct {
	apps             []runtime.Object
	manifestResponse *repository.ManifestResponse
	managedLiveObjs  map[kube.ResourceKey]*unstructured.Unstructured
}

func newFakeController(data *fakeData) *ApplicationController {
	var clust corev1.Secret
	err := yaml.Unmarshal([]byte(fakeCluster), &clust)
	if err != nil {
		panic(err)
	}

	// Mock out call to GenerateManifest
	mockRepoClient := mockrepoclient.RepositoryServiceClient{}
	mockRepoClient.On("GenerateManifest", mock.Anything, mock.Anything).Return(data.manifestResponse, nil)
	mockRepoClientset := mockreposerver.Clientset{}
	mockRepoClientset.On("NewRepositoryClient").Return(&fakeCloser{}, &mockRepoClient, nil)

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
		},
		Data: nil,
	}
	ctrl := NewApplicationController(
		test.FakeArgoCDNamespace,
		fake.NewSimpleClientset(&clust, &cm, &secret),
		appclientset.NewSimpleClientset(data.apps...),
		&mockRepoClientset,
		time.Minute,
	)

	// Mock out call to GetManagedLiveObjs if fake data supplied
	if data.managedLiveObjs != nil {
		mockStateCache := mockstatecache.LiveStateCache{}
		mockStateCache.On("GetManagedLiveObjs", mock.Anything, mock.Anything).Return(data.managedLiveObjs, nil)
		mockStateCache.On("IsNamespaced", mock.Anything, mock.Anything).Return(true, nil)
		ctrl.stateCache = &mockStateCache
		ctrl.appStateManager.(*appStateManager).liveStateCache = &mockStateCache

	}

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
  server: aHR0cHM6Ly9rdWJlcm5ldGVzLmRlZmF1bHQuc3Zj
kind: Secret
metadata:
  labels:
    argocd.argoproj.io/secret-type: cluster
  name: localhost-6443
  namespace: ` + test.FakeArgoCDNamespace + `
type: Opaque
`

var fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
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
	compRes := argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond := ctrl.autoSync(app, &compRes)
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
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	compRes := argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	cond := ctrl.autoSync(app, &compRes)
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)

	// Verify we skip when we are already Synced (even if revision is different)
	app = newFakeApp()
	ctrl = newFakeController(&fakeData{apps: []runtime.Object{app}})
	compRes = argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusSynced,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond = ctrl.autoSync(app, &compRes)
	assert.Nil(t, cond)
	app, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)

	// Verify we skip when auto-sync is disabled
	app = newFakeApp()
	app.Spec.SyncPolicy = nil
	ctrl = newFakeController(&fakeData{apps: []runtime.Object{app}})
	compRes = argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond = ctrl.autoSync(app, &compRes)
	assert.Nil(t, cond)
	app, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)

	// Verify we skip when previous sync attempt failed and return error condition
	// Set current to 'aaaaa', desired to 'bbbbb' and add 'bbbbb' to failure history
	app = newFakeApp()
	app.Status.OperationState = &argoappv1.OperationState{
		Operation: argoappv1.Operation{
			Sync: &argoappv1.SyncOperation{},
		},
		Phase: argoappv1.OperationFailed,
		SyncResult: &argoappv1.SyncOperationResult{
			Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
	ctrl = newFakeController(&fakeData{apps: []runtime.Object{app}})
	compRes = argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusOutOfSync,
		Revision: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	cond = ctrl.autoSync(app, &compRes)
	assert.NotNil(t, cond)
	app, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)
}

// TestAutoSyncIndicateError verifies we skip auto-sync and return error condition if previous sync failed
func TestAutoSyncIndicateError(t *testing.T) {
	app := newFakeApp()
	app.Spec.Source.ComponentParameterOverrides = []argoappv1.ComponentParameter{
		{
			Name:  "a",
			Value: "1",
		},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	compRes := argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	app.Status.OperationState = &argoappv1.OperationState{
		Operation: argoappv1.Operation{
			Sync: &argoappv1.SyncOperation{
				ParameterOverrides: argoappv1.ParameterOverrides{
					{
						Name:  "a",
						Value: "1",
					},
				},
			},
		},
		Phase: argoappv1.OperationFailed,
		SyncResult: &argoappv1.SyncOperationResult{
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	cond := ctrl.autoSync(app, &compRes)
	assert.NotNil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Nil(t, app.Operation)
}

// TestAutoSyncParameterOverrides verifies we auto-sync if revision is same but parameter overrides are different
func TestAutoSyncParameterOverrides(t *testing.T) {
	app := newFakeApp()
	app.Spec.Source.ComponentParameterOverrides = []argoappv1.ComponentParameter{
		{
			Name:  "a",
			Value: "1",
		},
	}
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	compRes := argoappv1.ComparisonResult{
		Status:   argoappv1.ComparisonStatusOutOfSync,
		Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	app.Status.OperationState = &argoappv1.OperationState{
		Operation: argoappv1.Operation{
			Sync: &argoappv1.SyncOperation{
				ParameterOverrides: argoappv1.ParameterOverrides{
					{
						Name:  "a",
						Value: "2", // this value changed
					},
				},
			},
		},
		Phase: argoappv1.OperationFailed,
		SyncResult: &argoappv1.SyncOperationResult{
			Revision: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
	}
	cond := ctrl.autoSync(app, &compRes)
	assert.Nil(t, cond)
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(test.FakeArgoCDNamespace).Get("my-app", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.NotNil(t, app.Operation)
}

// TestFinalizeAppDeletion verifies application deletion
func TestFinalizeAppDeletion(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})

	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	patched := false
	fakeAppCs.ReactionChain = nil
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		patched = true
		return true, nil, nil
	})
	err := ctrl.finalizeApplicationDeletion(app)
	// TODO: use an interface to fake out the calls to GetResourcesWithLabel and DeleteResourceWithLabel
	// For now just ensure we have an expected error condition
	assert.Error(t, err)     // Change this to assert.Nil when we stub out GetResourcesWithLabel/DeleteResourceWithLabel
	assert.False(t, patched) // Change this to assert.True when we stub out GetResourcesWithLabel/DeleteResourceWithLabel
}

// TestNormalizeApplication verifies we normalize an application during reconciliation
func TestNormalizeApplication(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = ""
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	cancel := test.StartInformer(ctrl.appInformer)
	defer cancel()
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

// TestDontNormalizeApplication verifies we dont unnecessarily normalize an application
func TestDontNormalizeApplication(t *testing.T) {
	app := newFakeApp()
	app.Spec.Project = "default"
	ctrl := newFakeController(&fakeData{apps: []runtime.Object{app}})
	cancel := test.StartInformer(ctrl.appInformer)
	defer cancel()
	key, _ := cache.MetaNamespaceKeyFunc(app)
	ctrl.appRefreshQueue.Add(key)

	fakeAppCs := ctrl.applicationClientset.(*appclientset.Clientset)
	fakeAppCs.ReactionChain = nil
	normalized := false
	fakeAppCs.AddReactor("patch", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
		if patchAction, ok := action.(kubetesting.PatchAction); ok {
			if strings.HasPrefix(string(patchAction.GetPatch()), `{"spec":`) {
				normalized = true
			}
		}
		return true, nil, nil
	})
	ctrl.processAppRefreshQueueItem()
	assert.False(t, normalized)
}
