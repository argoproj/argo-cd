package sync

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubetest"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

func TestSync_TrackedResourceConvertedToSameNameHook(t *testing.T) {
	for _, hookType := range []synccommon.HookType{synccommon.HookTypeSync, synccommon.HookTypePostSync} {
		for _, resume := range []bool{false, true} {
			name := string(hookType)
			if resume {
				name += "/resumed"
			}
			t.Run(name, func(t *testing.T) {
				live := testingutils.NewPod()
				live.SetNamespace(testingutils.FakeArgoCDNamespace)
				hookObj := newHook(live.GetName(), hookType, synccommon.HookDeletePolicyBeforeHookCreation)
				require.NoError(t, unstructured.SetNestedField(hookObj.Object, "Never", "spec", "restartPolicy"))
				client := fake.NewSimpleDynamicClient(runtime.NewScheme(), live)
				syncCtx := newTestSyncCtx(nil, WithPrune(true))
				syncCtx.resources = groupResources(ReconciliationResult{
					Live:   []*unstructured.Unstructured{live},
					Target: []*unstructured.Unstructured{nil},
				})
				syncCtx.hooks = []*unstructured.Unstructured{hookObj}
				syncCtx.dynamicIf = client

				// Prune the tracked object and delete it before creating the hook.
				syncCtx.Sync(t.Context())
				phase, message, results := syncCtx.GetState()
				require.Equal(t, synccommon.OperationRunning, phase, message)
				require.Len(t, results, 1)
				assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)
				assert.Empty(t, results[0].HookType)
				if resume {
					syncCtx = newTestSyncCtx(nil, WithPrune(true), WithInitialState(phase, message, results, metav1.NewTime(syncCtx.startedAt)))
					syncCtx.hooks = []*unstructured.Unstructured{hookObj}
					syncCtx.dynamicIf = client
				}

				// The next reconciliation no longer sees the old resource.
				syncCtx.resources = map[kube.ResourceKey]reconciledResource{}
				syncCtx.Sync(t.Context())
				resourceOps := syncCtx.resourceOps.(*kubetest.MockResourceOps)
				require.Equal(t, "apply", resourceOps.GetLastResourceCommand(kube.GetResourceKey(hookObj)), "the hook must be applied in the same operation")
				phase, message, results = syncCtx.GetState()
				require.Equal(t, synccommon.OperationRunning, phase, message)
				require.Len(t, results, 2)
				assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)
				assert.Empty(t, results[0].HookType)
				assert.Equal(t, synccommon.ResultCodeSynced, results[1].Status)
				assert.Equal(t, hookType, results[1].HookType)
				assert.Equal(t, synccommon.OperationRunning, results[1].HookPhase)

				// Restore both saved results while the newly created hook is running.
				if resume {
					syncCtx = newTestSyncCtx(nil, WithPrune(true), WithInitialState(phase, message, results, metav1.NewTime(syncCtx.startedAt)))
					syncCtx.hooks = []*unstructured.Unstructured{hookObj}
					syncCtx.dynamicIf = client
				}
				liveHook := hookObj.DeepCopy()
				require.NoError(t, unstructured.SetNestedField(liveHook.Object, "Running", "status", "phase"))
				syncCtx.resources = groupResources(ReconciliationResult{
					Live:   []*unstructured.Unstructured{liveHook},
					Target: []*unstructured.Unstructured{nil},
				})
				syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme(), liveHook)
				syncCtx.Sync(t.Context())
				phase, message, _ = syncCtx.GetState()
				assert.Equal(t, synccommon.OperationRunning, phase, message)
				if resume {
					resourceOps = syncCtx.resourceOps.(*kubetest.MockResourceOps)
					assert.Empty(t, resourceOps.GetLastResourceCommand(kube.GetResourceKey(hookObj)), "a restored running hook must not be applied again")
				}

				require.NoError(t, unstructured.SetNestedField(liveHook.Object, "Succeeded", "status", "phase"))
				syncCtx.Sync(t.Context())
				phase, message, results = syncCtx.GetState()
				assert.Equal(t, synccommon.OperationSucceeded, phase, message)
				require.Len(t, results, 2)
				assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)
				assert.Empty(t, results[0].HookType)
				assert.Equal(t, hookType, results[1].HookType)
				assert.Equal(t, synccommon.OperationSucceeded, results[1].HookPhase)
			})
		}
	}
}

func TestSync_ResumesConvertedHookFromLegacyResult(t *testing.T) {
	for _, phase := range []synccommon.OperationPhase{
		synccommon.OperationRunning, synccommon.OperationSucceeded, synccommon.OperationFailed,
	} {
		t.Run(string(phase), func(t *testing.T) {
			hookObj := newHook("init-job", synccommon.HookTypeSync, synccommon.HookDeletePolicyBeforeHookCreation)
			require.NoError(t, unstructured.SetNestedField(hookObj.Object, "Never", "spec", "restartPolicy"))
			liveHook := hookObj.DeepCopy()
			require.NoError(t, unstructured.SetNestedField(liveHook.Object, string(phase), "status", "phase"))
			// Before hook types were part of the key, applying a hook after an
			// immediate prune updated the prune result but kept its empty HookType.
			legacyResult := synccommon.ResourceSyncResult{
				ResourceKey: kube.GetResourceKey(hookObj),
				Status:      synccommon.ResultCodeSynced,
				HookPhase:   phase,
				SyncPhase:   synccommon.SyncPhaseSync,
				Order:       1,
			}
			syncCtx := newTestSyncCtx(nil, WithPrune(true), WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{legacyResult}, metav1.Now()))
			syncCtx.hooks = []*unstructured.Unstructured{hookObj}
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{liveHook},
				Target: []*unstructured.Unstructured{nil},
			})
			client := fake.NewSimpleDynamicClient(runtime.NewScheme(), liveHook)
			syncCtx.dynamicIf = client
			syncCtx.Sync(t.Context())
			actualPhase, message, _ := syncCtx.GetState()
			assert.Equal(t, phase, actualPhase, message)
			for _, action := range client.Actions() {
				assert.NotEqual(t, "delete", action.GetVerb(), "resuming a previously applied hook must not delete it")
			}
		})
	}
}

func TestSync_ResourceApplyResultDoesNotCompleteHook(t *testing.T) {
	for _, resource := range []*unstructured.Unstructured{testingutils.NewPod(), testingutils.NewClusterRole()} {
		t.Run(resource.GetKind(), func(t *testing.T) {
			if resource.GetKind() == "Pod" {
				resource.SetNamespace(testingutils.FakeArgoCDNamespace)
			}
			hookObj := resource.DeepCopy()
			testingutils.Annotate(hookObj, synccommon.AnnotationKeyHook, string(synccommon.HookTypeSync))
			// getSyncTasks fills in the target namespace even for cluster resources.
			key := kube.GetResourceKey(resource)
			key.Namespace = testingutils.FakeArgoCDNamespace
			result := synccommon.ResourceSyncResult{
				ResourceKey: key,
				Status:      synccommon.ResultCodeSynced,
				HookPhase:   synccommon.OperationSucceeded,
				SyncPhase:   synccommon.SyncPhaseSync,
			}
			syncCtx := newTestSyncCtx(nil, WithInitialState(synccommon.OperationRunning, "", []synccommon.ResourceSyncResult{result}, metav1.Now()))
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{resource},
				Target: []*unstructured.Unstructured{resource},
			})
			syncCtx.hooks = []*unstructured.Unstructured{hookObj}
			tasks, valid := syncCtx.getSyncTasks(t.Context())
			require.True(t, valid)
			require.Len(t, tasks, 2)
			for _, task := range tasks {
				if task.isHook() {
					assert.True(t, task.pending(), "a regular resource's apply result must not complete the hook")
				} else {
					assert.True(t, task.completed())
				}
			}
		})
	}
}

func TestSync_TrackedJobConvertedToHookAfterPrune(t *testing.T) {
	for _, tc := range []struct {
		condition string
		phase     synccommon.OperationPhase
	}{
		{condition: "Complete", phase: synccommon.OperationSucceeded},
		{condition: "Failed", phase: synccommon.OperationFailed},
	} {
		t.Run(tc.condition, func(t *testing.T) {
			live := testingutils.Unstructured(`
apiVersion: batch/v1
kind: Job
metadata:
  name: init-job
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: init
        image: busybox
        command: ["true"]
`)
			live.SetNamespace(testingutils.FakeArgoCDNamespace)
			hookObj := live.DeepCopy()
			testingutils.Annotate(hookObj, synccommon.AnnotationKeyHook, string(synccommon.HookTypeSync))
			testingutils.Annotate(hookObj, synccommon.AnnotationKeyHookDeletePolicy, string(synccommon.HookDeletePolicyBeforeHookCreation))
			syncCtx := newTestSyncCtx(nil, WithPrune(true))
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{live},
				Target: []*unstructured.Unstructured{nil},
			})
			syncCtx.hooks = []*unstructured.Unstructured{hookObj}
			// The cache still has the old Job, but it is gone by the time the
			// hook's BeforeHookCreation deletion reaches the API server.
			syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme())
			syncCtx.Sync(t.Context())
			resourceOps := syncCtx.resourceOps.(*kubetest.MockResourceOps)
			require.Equal(t, "apply", resourceOps.GetLastResourceCommand(kube.GetResourceKey(hookObj)))
			phase, message, results := syncCtx.GetState()
			require.Equal(t, synccommon.OperationRunning, phase, message)
			require.Len(t, results, 2)
			assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)
			assert.Empty(t, results[0].HookType)
			assert.Equal(t, synccommon.ResultCodeSynced, results[1].Status)
			assert.Equal(t, synccommon.HookTypeSync, results[1].HookType)

			syncCtx = newTestSyncCtx(nil, WithPrune(true), WithInitialState(phase, message, results, metav1.NewTime(syncCtx.startedAt)))
			syncCtx.hooks = []*unstructured.Unstructured{hookObj}
			liveHook := hookObj.DeepCopy()
			require.NoError(t, unstructured.SetNestedSlice(liveHook.Object, []any{
				map[string]any{"type": tc.condition, "status": "True"},
			}, "status", "conditions"))
			syncCtx.resources = groupResources(ReconciliationResult{
				Live:   []*unstructured.Unstructured{liveHook},
				Target: []*unstructured.Unstructured{nil},
			})
			syncCtx.dynamicIf = fake.NewSimpleDynamicClient(runtime.NewScheme(), liveHook)
			syncCtx.Sync(t.Context())
			phase, message, results = syncCtx.GetState()
			assert.Equal(t, tc.phase, phase, message)
			require.Len(t, results, 2)
			assert.Equal(t, synccommon.OperationSucceeded, results[0].HookPhase)
			assert.Equal(t, synccommon.ResultCodePruned, results[0].Status)
			assert.Empty(t, results[0].HookType)
			assert.Equal(t, tc.phase, results[1].HookPhase)
			assert.Equal(t, synccommon.HookTypeSync, results[1].HookType)
			resourceOps = syncCtx.resourceOps.(*kubetest.MockResourceOps)
			assert.Empty(t, resourceOps.GetLastResourceCommand(kube.GetResourceKey(hookObj)))
		})
	}
}
