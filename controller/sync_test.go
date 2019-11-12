package controller

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/argoproj/argo-cd/engine/util/settings"

	"github.com/argoproj/argo-cd/engine/pkg"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	fakedisco "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
	testcore "k8s.io/client-go/testing"

	"github.com/argoproj/argo-cd/engine/common"
	"github.com/argoproj/argo-cd/engine/util/kube"
	"github.com/argoproj/argo-cd/engine/util/kube/kubetest"

	. "github.com/argoproj/argo-cd/engine/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test"
)

func newTestSyncCtx(resources ...*v1.APIResourceList) *syncContext {
	fakeDisco := &fakedisco.FakeDiscovery{Fake: &testcore.Fake{}}
	fakeDisco.Resources = append(resources,
		&v1.APIResourceList{
			GroupVersion: "v1",
			APIResources: []v1.APIResource{
				{Kind: "Pod", Group: "", Version: "v1", Namespaced: true},
				{Kind: "Service", Group: "", Version: "v1", Namespaced: true},
			},
		},
		&v1.APIResourceList{
			GroupVersion: "apps/v1",
			APIResources: []v1.APIResource{
				{Kind: "Deployment", Group: "apps", Version: "v1", Namespaced: true},
			},
		})
	sc := syncContext{
		config:    &rest.Config{},
		namespace: test.FakeArgoCDNamespace,
		server:    test.FakeClusterURL,
		syncRes: &SyncOperationResult{
			Revision: "FooBarBaz",
		},
		syncOp: &SyncOperation{
			Prune: true,
			SyncStrategy: &SyncStrategy{
				Apply: &SyncStrategyApply{},
			},
		},
		proj: &AppProject{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Spec: AppProjectSpec{
				Destinations: []ApplicationDestination{{
					Server:    test.FakeClusterURL,
					Namespace: test.FakeArgoCDNamespace,
				}},
				ClusterResourceWhitelist: []v1.GroupKind{
					{Group: "*", Kind: "*"},
				},
			},
		},
		opState:   &OperationState{},
		disco:     fakeDisco,
		log:       log.WithFields(log.Fields{"application": "fake-app"}),
		callbacks: settings.NewNoOpCallbacks(),
	}
	sc.kubectl = &kubetest.MockKubectlCmd{}
	return &sc
}

func newManagedResource(live *unstructured.Unstructured) managedResource {
	return managedResource{
		Live:      live,
		Group:     live.GroupVersionKind().Group,
		Version:   live.GroupVersionKind().Version,
		Kind:      live.GroupVersionKind().Kind,
		Namespace: live.GetNamespace(),
		Name:      live.GetName(),
	}
}

func TestSyncNotPermittedNamespace(t *testing.T) {
	syncCtx := newTestSyncCtx()
	targetPod := test.NewPod()
	targetPod.SetNamespace("kube-system")
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: targetPod,
		}, {
			Live:   nil,
			Target: test.NewService(),
		}},
	}
	syncCtx.sync()
	assert.Equal(t, OperationFailed, syncCtx.opState.Phase)
	assert.Contains(t, syncCtx.syncRes.Resources[0].Message, "not permitted in project")
}

func TestSyncCreateInSortedOrder(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: test.NewPod(),
		}, {
			Live:   nil,
			Target: test.NewService(),
		}},
	}
	syncCtx.sync()
	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	for i := range syncCtx.syncRes.Resources {
		result := syncCtx.syncRes.Resources[i]
		if result.Kind == "Pod" {
			assert.Equal(t, ResultCodeSynced, result.Status)
			assert.Equal(t, "", result.Message)
		} else if result.Kind == "Service" {
			assert.Equal(t, "", result.Message)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncCreateNotWhitelistedClusterResources(t *testing.T) {
	syncCtx := newTestSyncCtx(&v1.APIResourceList{
		GroupVersion: SchemeGroupVersion.String(),
		APIResources: []v1.APIResource{
			{Name: "workflows", Namespaced: false, Kind: "Workflow", Group: "argoproj.io"},
			{Name: "application", Namespaced: false, Kind: "Application", Group: "argoproj.io"},
		},
	}, &v1.APIResourceList{
		GroupVersion: "rbac.authorization.k8s.io/v1",
		APIResources: []v1.APIResource{
			{Name: "clusterroles", Namespaced: false, Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
		},
	})

	syncCtx.proj.Spec.ClusterResourceWhitelist = []v1.GroupKind{
		{Group: "argoproj.io", Kind: "*"},
	}

	syncCtx.kubectl = &kubetest.MockKubectlCmd{}
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live: nil,
			Target: kube.MustToUnstructured(&rbacv1.ClusterRole{
				TypeMeta:   metav1.TypeMeta{Kind: "ClusterRole", APIVersion: "rbac.authorization.k8s.io/v1"},
				ObjectMeta: metav1.ObjectMeta{Name: "argo-ui-cluster-role"}}),
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	result := syncCtx.syncRes.Resources[0]
	assert.Equal(t, ResultCodeSyncFailed, result.Status)
	assert.Contains(t, result.Message, "not permitted in project")
}

func TestSyncBlacklistedNamespacedResources(t *testing.T) {
	syncCtx := newTestSyncCtx()

	syncCtx.proj.Spec.NamespaceResourceBlacklist = []v1.GroupKind{
		{Group: "*", Kind: "Deployment"},
	}

	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: test.NewDeployment(),
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	result := syncCtx.syncRes.Resources[0]
	assert.Equal(t, ResultCodeSyncFailed, result.Status)
	assert.Contains(t, result.Message, "not permitted in project")
}

func TestSyncSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	pod := test.NewPod()
	pod.SetNamespace(test.FakeArgoCDNamespace)
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: test.NewService(),
		}, {
			Live:   pod,
			Target: nil,
		}},
	}
	syncCtx.sync()
	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
	assert.Len(t, syncCtx.syncRes.Resources, 2)
	for i := range syncCtx.syncRes.Resources {
		result := syncCtx.syncRes.Resources[i]
		if result.Kind == "Pod" {
			assert.Equal(t, ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		} else if result.Kind == "Service" {
			assert.Equal(t, ResultCodeSynced, result.Status)
			assert.Equal(t, "", result.Message)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncDeleteSuccessfully(t *testing.T) {
	syncCtx := newTestSyncCtx()
	svc := test.NewService()
	svc.SetNamespace(test.FakeArgoCDNamespace)
	pod := test.NewPod()
	pod.SetNamespace(test.FakeArgoCDNamespace)
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   svc,
			Target: nil,
		}, {
			Live:   pod,
			Target: nil,
		}},
	}
	syncCtx.sync()
	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
	for i := range syncCtx.syncRes.Resources {
		result := syncCtx.syncRes.Resources[i]
		if result.Kind == "Pod" {
			assert.Equal(t, ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		} else if result.Kind == "Service" {
			assert.Equal(t, ResultCodePruned, result.Status)
			assert.Equal(t, "pruned", result.Message)
		} else {
			t.Error("Resource isn't a pod or a service")
		}
	}
}

func TestSyncCreateFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	testSvc := test.NewService()
	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			testSvc.GetName(): {
				Output: "",
				Err:    fmt.Errorf("foo"),
			},
		},
	}
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   nil,
			Target: testSvc,
		}},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	result := syncCtx.syncRes.Resources[0]
	assert.Equal(t, ResultCodeSyncFailed, result.Status)
	assert.Equal(t, "foo", result.Message)
}

func TestSyncPruneFailure(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			"test-service": {
				Output: "",
				Err:    fmt.Errorf("foo"),
			},
		},
	}
	testSvc := test.NewService()
	testSvc.SetName("test-service")
	testSvc.SetNamespace(test.FakeArgoCDNamespace)
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{
			Live:   testSvc,
			Target: nil,
		}},
	}
	syncCtx.sync()
	assert.Equal(t, OperationFailed, syncCtx.opState.Phase)
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	result := syncCtx.syncRes.Resources[0]
	assert.Equal(t, ResultCodeSyncFailed, result.Status)
	assert.Equal(t, "foo", result.Message)
}

func TestDontSyncOrPruneHooks(t *testing.T) {
	syncCtx := newTestSyncCtx()
	targetPod := test.NewPod()
	targetPod.SetName("dont-create-me")
	targetPod.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	liveSvc := test.NewService()
	liveSvc.SetName("dont-prune-me")
	liveSvc.SetNamespace(test.FakeArgoCDNamespace)
	liveSvc.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})

	syncCtx.compareResult = &comparisonResult{
		hooks: []*unstructured.Unstructured{targetPod, liveSvc},
	}
	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 0)
	syncCtx.sync()
	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
}

// make sure that we do not prune resources with Prune=false
func TestDontPrunePruneFalse(t *testing.T) {
	syncCtx := newTestSyncCtx()
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationSyncOptions: "Prune=false"})
	pod.SetNamespace(test.FakeArgoCDNamespace)
	syncCtx.compareResult = &comparisonResult{managedResources: []managedResource{{Live: pod}}}

	syncCtx.sync()

	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Equal(t, ResultCodePruneSkipped, syncCtx.syncRes.Resources[0].Status)
	assert.Equal(t, "ignored (no prune)", syncCtx.syncRes.Resources[0].Message)

	syncCtx.sync()

	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
}

// make sure Validate=false means we don't validate
func TestSyncOptionValidate(t *testing.T) {
	tests := []struct {
		name          string
		annotationVal string
		want          bool
	}{
		{"Empty", "", true},
		{"True", "Validate=true", true},
		{"False", "Validate=false", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			syncCtx := newTestSyncCtx()
			pod := test.NewPod()
			pod.SetAnnotations(map[string]string{common.AnnotationSyncOptions: tt.annotationVal})
			pod.SetNamespace(test.FakeArgoCDNamespace)
			syncCtx.compareResult = &comparisonResult{managedResources: []managedResource{{Target: pod, Live: pod}}}

			syncCtx.sync()

			kubectl, _ := syncCtx.kubectl.(*kubetest.MockKubectlCmd)
			assert.Equal(t, tt.want, kubectl.LastValidate)
		})
	}
}

func TestSelectiveSyncOnly(t *testing.T) {
	syncCtx := newTestSyncCtx()
	pod1 := test.NewPod()
	pod1.SetName("pod-1")
	pod2 := test.NewPod()
	pod2.SetName("pod-2")
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{Target: pod1}},
	}
	syncCtx.syncResources = []SyncOperationResource{{Kind: "Pod", Name: "pod-1"}}

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "pod-1", tasks[0].name())
}

func TestUnnamedHooksGetUniqueNames(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.syncOp.SyncStrategy.Apply = nil
	pod := test.NewPod()
	pod.SetName("")
	pod.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync,PostSync"})
	syncCtx.compareResult = &comparisonResult{hooks: []*unstructured.Unstructured{pod}}

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 2)
	assert.Contains(t, tasks[0].name(), "foobarb-presync-")
	assert.Contains(t, tasks[1].name(), "foobarb-postsync-")
	assert.Equal(t, "", pod.GetName())
}

func TestManagedResourceAreNotNamed(t *testing.T) {
	syncCtx := newTestSyncCtx()
	pod := test.NewPod()
	pod.SetName("")
	syncCtx.compareResult = &comparisonResult{managedResources: []managedResource{{Target: pod}}}

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, "", tasks[0].name())
	assert.Equal(t, "", pod.GetName())
}

func TestDeDupingTasks(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.syncOp.SyncStrategy.Apply = nil
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationKeyHook: "Sync"})
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{Target: pod}},
		hooks:            []*unstructured.Unstructured{pod},
	}

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
}

func TestObjectsGetANamespace(t *testing.T) {
	syncCtx := newTestSyncCtx()
	pod := test.NewPod()
	syncCtx.compareResult = &comparisonResult{managedResources: []managedResource{{Target: pod}}}

	tasks, successful := syncCtx.getSyncTasks()

	assert.True(t, successful)
	assert.Len(t, tasks, 1)
	assert.Equal(t, test.FakeArgoCDNamespace, tasks[0].namespace())
	assert.Equal(t, "", pod.GetNamespace())
}

func TestPersistRevisionHistory(t *testing.T) {
	app := newFakeApp()
	app.Status.OperationState = nil
	app.Status.History = nil

	defaultProject := &AppProject{
		ObjectMeta: v1.ObjectMeta{
			Namespace: test.FakeArgoCDNamespace,
			Name:      "default",
		},
	}
	data := fakeData{
		apps: []runtime.Object{app, defaultProject},
		manifestResponse: &pkg.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	// Sync with source unspecified
	opState := &OperationState{Operation: Operation{
		Sync: &SyncOperation{},
	}}
	ctrl.appStateManager.SyncAppState(app, opState)
	// Ensure we record spec.source into sync result
	assert.Equal(t, app.Spec.Source, opState.SyncResult.Source)

	updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.Equal(t, app.Spec.Source, updatedApp.Status.History[0].Source)
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)
}

func TestPersistRevisionHistoryRollback(t *testing.T) {
	app := newFakeApp()
	app.Status.OperationState = nil
	app.Status.History = nil
	defaultProject := &AppProject{
		ObjectMeta: v1.ObjectMeta{
			Namespace: test.FakeArgoCDNamespace,
			Name:      "default",
		},
	}
	data := fakeData{
		apps: []runtime.Object{app, defaultProject},
		manifestResponse: &pkg.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	// Sync with source specified
	source := ApplicationSource{
		Helm: &ApplicationSourceHelm{
			Parameters: []HelmParameter{
				{
					Name:  "test",
					Value: "123",
				},
			},
		},
	}
	opState := &OperationState{Operation: Operation{
		Sync: &SyncOperation{
			Source: &source,
		},
	}}
	ctrl.appStateManager.SyncAppState(app, opState)
	// Ensure we record opState's source into sync result
	assert.Equal(t, source, opState.SyncResult.Source)

	updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.Equal(t, source, updatedApp.Status.History[0].Source)
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)
}

func TestSyncFailureHookWithSuccessfulSync(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.syncOp.SyncStrategy.Apply = nil
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{Target: test.NewPod()}},
		hooks:            []*unstructured.Unstructured{test.NewHook(HookTypeSyncFail)},
	}

	syncCtx.sync()

	assert.Equal(t, OperationSucceeded, syncCtx.opState.Phase)
	// only one result, we did not run the failure failureHook
	assert.Len(t, syncCtx.syncRes.Resources, 1)
}

func TestSyncFailureHookWithFailedSync(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.syncOp.SyncStrategy.Apply = nil
	pod := test.NewPod()
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{Target: pod}},
		hooks:            []*unstructured.Unstructured{test.NewHook(HookTypeSyncFail)},
	}
	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{pod.GetName(): {Err: fmt.Errorf("")}},
	}

	syncCtx.sync()
	syncCtx.sync()

	assert.Equal(t, OperationFailed, syncCtx.opState.Phase)
	assert.Len(t, syncCtx.syncRes.Resources, 2)
}

func TestBeforeHookCreation(t *testing.T) {
	syncCtx := newTestSyncCtx()
	syncCtx.syncOp.SyncStrategy.Apply = nil
	hook := test.Annotate(test.Annotate(test.NewPod(), common.AnnotationKeyHook, "Sync"), common.AnnotationKeyHookDeletePolicy, "BeforeHookCreation")
	hook.SetNamespace(test.FakeArgoCDNamespace)
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{newManagedResource(hook)},
		hooks:            []*unstructured.Unstructured{hook},
	}
	syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())

	syncCtx.sync()
	assert.Len(t, syncCtx.syncRes.Resources, 1)
	assert.Empty(t, syncCtx.syncRes.Resources[0].Message)
}

func TestRunSyncFailHooksFailed(t *testing.T) {
	// Tests that other SyncFail Hooks run even if one of them fail.

	syncCtx := newTestSyncCtx()
	syncCtx.syncOp.SyncStrategy.Apply = nil
	pod := test.NewPod()
	successfulSyncFailHook := test.NewHook(HookTypeSyncFail)
	successfulSyncFailHook.SetName("successful-sync-fail-hook")
	failedSyncFailHook := test.NewHook(HookTypeSyncFail)
	failedSyncFailHook.SetName("failed-sync-fail-hook")
	syncCtx.compareResult = &comparisonResult{
		managedResources: []managedResource{{Target: pod}},
		hooks:            []*unstructured.Unstructured{successfulSyncFailHook, failedSyncFailHook},
	}

	syncCtx.kubectl = &kubetest.MockKubectlCmd{
		Commands: map[string]kubetest.KubectlOutput{
			// Fail operation
			pod.GetName(): {Err: fmt.Errorf("")},
			// Fail a single SyncFail hook
			failedSyncFailHook.GetName(): {Err: fmt.Errorf("")}},
	}

	syncCtx.sync()
	syncCtx.sync()

	fmt.Println(syncCtx.syncRes.Resources)
	fmt.Println(syncCtx.opState.Phase)
	// Operation as a whole should fail
	assert.Equal(t, OperationFailed, syncCtx.opState.Phase)
	// failedSyncFailHook should fail
	assert.Equal(t, OperationFailed, syncCtx.syncRes.Resources[1].HookPhase)
	assert.Equal(t, ResultCodeSyncFailed, syncCtx.syncRes.Resources[1].Status)
	// successfulSyncFailHook should be synced running (it is an nginx pod)
	assert.Equal(t, OperationRunning, syncCtx.syncRes.Resources[2].HookPhase)
	assert.Equal(t, ResultCodeSynced, syncCtx.syncRes.Resources[2].Status)
}

func Test_syncContext_isSelectiveSync(t *testing.T) {
	type fields struct {
		compareResult *comparisonResult
		syncResources []SyncOperationResource
	}
	oneSyncResource := []SyncOperationResource{{}}
	oneResource := func(group, kind, name string, hook bool) *comparisonResult {
		return &comparisonResult{resources: []ResourceStatus{{Group: group, Kind: kind, Name: name, Hook: hook}}}
	}
	tests := []struct {
		name   string
		fields fields
		want   bool
	}{
		{"Empty", fields{}, false},
		{"OneCompareResult", fields{oneResource("", "", "", false), []SyncOperationResource{}}, true},
		{"OneSyncResource", fields{&comparisonResult{}, oneSyncResource}, true},
		{"Equal", fields{oneResource("", "", "", false), oneSyncResource}, false},
		{"EqualOutOfOrder", fields{&comparisonResult{resources: []ResourceStatus{{Group: "a"}, {Group: "b"}}}, []SyncOperationResource{{Group: "b"}, {Group: "a"}}}, false},
		{"KindDifferent", fields{oneResource("foo", "", "", false), oneSyncResource}, true},
		{"GroupDifferent", fields{oneResource("", "foo", "", false), oneSyncResource}, true},
		{"NameDifferent", fields{oneResource("", "", "foo", false), oneSyncResource}, true},
		{"HookIgnored", fields{oneResource("", "", "", true), []SyncOperationResource{}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &syncContext{
				compareResult: tt.fields.compareResult,
				syncResources: tt.fields.syncResources,
				callbacks:     settings.NewNoOpCallbacks(),
			}
			if got := sc.isSelectiveSync(); got != tt.want {
				t.Errorf("syncContext.isSelectiveSync() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_syncContext_liveObj(t *testing.T) {
	type fields struct {
		compareResult *comparisonResult
	}
	type args struct {
		obj *unstructured.Unstructured
	}
	obj := test.NewPod()
	obj.SetNamespace("my-ns")

	found := test.NewPod()

	tests := []struct {
		name   string
		fields fields
		args   args
		want   *unstructured.Unstructured
	}{
		{"None", fields{compareResult: &comparisonResult{managedResources: []managedResource{}}}, args{obj: &unstructured.Unstructured{}}, nil},
		{"Found", fields{compareResult: &comparisonResult{managedResources: []managedResource{{Group: obj.GroupVersionKind().Group, Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName(), Live: found}}}}, args{obj: obj}, found},
		{"EmptyNamespace", fields{compareResult: &comparisonResult{managedResources: []managedResource{{Group: obj.GroupVersionKind().Group, Kind: obj.GetKind(), Name: obj.GetName(), Live: found}}}}, args{obj: obj}, found},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sc := &syncContext{
				compareResult: tt.fields.compareResult,
				callbacks:     settings.NewNoOpCallbacks(),
			}
			if got := sc.liveObj(tt.args.obj); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("syncContext.liveObj() = %v, want %v", got, tt.want)
			}
		})
	}
}
