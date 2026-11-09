package sync

// Reproduces a race between BeforeHookCreation and a lagging cluster cache.
//
// A hook with a fixed name and the default BeforeHookCreation delete policy is
// synced again. The engine deletes the previous hook object, re-creates it,
// and on the next pass matches the hook task to its live object by
// group/kind/namespace/name only (syncContext.liveObj). While the cluster cache
// still holds the deleted, completed predecessor, the engine judges the new
// hook by the predecessor: it reports the hook Succeeded and finishes the
// operation while the new hook is still running, and removeHookFinalizer
// no-ops on the predecessor's copy, which has no finalizer, so the new hook
// keeps argocd.argoproj.io/hook-finalizer.
//
// Each pass builds a fresh sync context from the previous pass's state, the
// way the application controller does (controller/sync.go: NewSyncContext with
// WithInitialState and the reconciliation result of the cluster cache). The
// fake dynamic client plays the API server; the `cache` argument of each pass
// plays the cluster cache, which may lag behind it.

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	testcore "k8s.io/client-go/testing"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"

	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/hook"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube/kubetest"
	testingutils "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/testing"
)

var jobsGVR = schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

const smokeJobName = "web-smoke"

// smokeHookManifest is the hook as rendered from Git: a PostSync Job with a
// fixed name and no delete policy, so BeforeHookCreation applies.
func smokeHookManifest() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]any{
			"name":      smokeJobName,
			"namespace": testingutils.FakeArgoCDNamespace,
			"annotations": map[string]any{
				synccommon.AnnotationKeyHook: string(synccommon.HookTypePostSync),
			},
		},
		"spec": map[string]any{
			"backoffLimit": int64(0),
			"template": map[string]any{"spec": map[string]any{
				"restartPolicy": "Never",
				"containers":    []any{map[string]any{"name": "smoke", "image": "busybox", "command": []any{"true"}}},
			}},
		},
	}}
}

// smokeJob is the hook as it exists in the cluster.
func smokeJob(uid string, complete bool, finalizers ...string) *unstructured.Unstructured {
	obj := smokeHookManifest()
	obj.SetUID(types.UID(uid))
	obj.SetFinalizers(finalizers)
	if complete {
		obj.Object["status"] = map[string]any{
			"succeeded": int64(1),
			"conditions": []any{map[string]any{
				"type":    "Complete",
				"status":  "True",
				"reason":  "CompletionsReached",
				"message": "Reached expected number of succeeded pods",
			}},
		}
	} else {
		obj.Object["status"] = map[string]any{"active": int64(1)}
	}
	return obj
}

// apiBackedResourceOps applies objects to the fake API server, so the hook the
// engine creates really exists there with a new UID.
type apiBackedResourceOps struct {
	*kubetest.MockResourceOps
	client *fake.FakeDynamicClient
	uid    string
}

func (r *apiBackedResourceOps) ApplyResource(ctx context.Context, obj *unstructured.Unstructured, dryRun cmdutil.DryRunStrategy, force, validate, serverSideApply bool, manager string) (string, error) {
	msg, err := r.MockResourceOps.ApplyResource(ctx, obj, dryRun, force, validate, serverSideApply, manager)
	if err != nil || dryRun != cmdutil.DryRunNone {
		return msg, err
	}
	created := obj.DeepCopy()
	created.SetUID(types.UID(r.uid))
	created.Object["status"] = map[string]any{"active": int64(1)}
	_, err = r.client.Resource(jobsGVR).Namespace(obj.GetNamespace()).Create(ctx, created, metav1.CreateOptions{})
	return "job.batch/" + obj.GetName() + " created", err
}

type hookRaceHarness struct {
	t       *testing.T
	client  *fake.FakeDynamicClient
	phase   synccommon.OperationPhase
	message string
	results []synccommon.ResourceSyncResult
	started metav1.Time
}

func newHookRaceHarness(t *testing.T, apiObjects ...runtime.Object) *hookRaceHarness {
	t.Helper()
	return &hookRaceHarness{
		t:       t,
		client:  fake.NewSimpleDynamicClient(runtime.NewScheme(), apiObjects...),
		started: metav1.Now(),
	}
}

// pass runs one controller pass. cache is what the cluster cache holds for the
// hook's key on this pass (nil: nothing).
func (h *hookRaceHarness) pass(cache *unstructured.Unstructured) {
	h.t.Helper()
	opts := []SyncOpt{}
	if h.phase != "" {
		opts = append(opts, WithInitialState(h.phase, h.message, h.results, h.started))
	}
	sc := newTestSyncCtx(nil, opts...)
	sc.dynamicIf = h.client
	sc.resourceOps = &apiBackedResourceOps{MockResourceOps: &kubetest.MockResourceOps{}, client: h.client, uid: "uid-new"}
	// Hooks are split off the targets by Reconcile; a live hook appears in the
	// resources map with a nil target.
	live := []*unstructured.Unstructured{}
	target := []*unstructured.Unstructured{}
	if cache != nil {
		live = append(live, cache.DeepCopy())
		target = append(target, nil)
	}
	sc.resources = groupResources(ReconciliationResult{Live: live, Target: target})
	sc.hooks = []*unstructured.Unstructured{smokeHookManifest()}
	sc.Sync(context.Background())
	h.phase, h.message, h.results = sc.GetState()
	h.t.Logf("pass: phase=%s message=%q hook=%+v", h.phase, h.message, h.hookResult())
}

func (h *hookRaceHarness) hookResult() synccommon.ResourceSyncResult {
	for _, r := range h.results {
		if r.ResourceKey.Name == smokeJobName {
			return r
		}
	}
	return synccommon.ResourceSyncResult{}
}

func (h *hookRaceHarness) apiJob() *unstructured.Unstructured {
	h.t.Helper()
	obj, err := h.client.Resource(jobsGVR).Namespace(testingutils.FakeArgoCDNamespace).Get(context.Background(), smokeJobName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	require.NoError(h.t, err)
	return obj
}

// writesSince lists the update and patch calls on Jobs since mark.
func (h *hookRaceHarness) writesSince(mark int) []string {
	var out []string
	for _, a := range h.client.Actions()[mark:] {
		if a.GetResource() == jobsGVR && (a.GetVerb() == "update" || a.GetVerb() == "patch") {
			out = append(out, a.GetVerb())
		}
	}
	return out
}

func (h *hookRaceHarness) logActions(mark int) {
	h.t.Helper()
	for _, a := range h.client.Actions()[mark:] {
		name := ""
		switch act := a.(type) {
		case testcore.GetAction:
			name = act.GetName()
		case testcore.DeleteAction:
			name = act.GetName()
		case testcore.CreateAction:
			name = act.GetObject().(*unstructured.Unstructured).GetName()
		case testcore.UpdateAction:
			name = act.GetObject().(*unstructured.Unstructured).GetName()
		}
		h.t.Logf("  api: %s %s %s", a.GetVerb(), a.GetResource().Resource, name)
	}
}

// TestSync_RecreatedHookNotJudgedByStaleCachedPredecessor drives the controller
// passes of a second sync of a fixed-name BeforeHookCreation hook, with a
// cluster cache that has not yet observed the predecessor's deletion.
func TestSync_RecreatedHookNotJudgedByStaleCachedPredecessor(t *testing.T) {
	// The previous sync's Job: completed, its hook finalizer removed at the end of that sync.
	predecessor := smokeJob("uid-old", true)
	h := newHookRaceHarness(t, predecessor.DeepCopy())

	// Pass 1: the hook is pending and its live object is the predecessor, so
	// BeforeHookCreation deletes it and the pass waits for the deletion.
	mark := len(h.client.Actions())
	h.pass(predecessor)
	h.logActions(mark)
	require.Equal(t, synccommon.OperationRunning, h.phase)
	require.Equal(t, "waiting for deletion of hook batch/Job/web-smoke", h.message)
	require.Nil(t, h.apiJob(), "the predecessor is gone from the API server")

	// Pass 2: the cache still holds the predecessor. The delete returns
	// NotFound, liveObj is cleared, and the new Job is created in the same pass.
	mark = len(h.client.Actions())
	h.pass(predecessor)
	h.logActions(mark)
	require.Equal(t, synccommon.OperationRunning, h.phase)
	require.Equal(t, synccommon.OperationRunning, h.hookResult().HookPhase)
	created := h.apiJob()
	require.NotNil(t, created)
	require.Equal(t, types.UID("uid-new"), created.GetUID())
	require.True(t, hook.HasHookFinalizer(created), "the engine adds the hook finalizer on creation")

	// Pass 3: the cache STILL holds the predecessor (the race). The new Job is
	// running in the API server. The operation must keep waiting for it.
	mark = len(h.client.Actions())
	h.pass(predecessor)
	h.logActions(mark)
	writes := h.writesSince(mark)
	assert.Equal(t, synccommon.OperationRunning, h.phase,
		"operation must not finish while the re-created hook is running (message %q)", h.message)
	assert.Equal(t, synccommon.OperationRunning, h.hookResult().HookPhase,
		"hook judged by the deleted predecessor (message %q)", h.hookResult().Message)
	if h.phase.Completed() {
		// The operation is over: nothing will ever remove the new Job's finalizer.
		live := h.apiJob()
		t.Logf("API server after pass 3: job/%s uid=%s finalizers=%v status=%v", live.GetName(), live.GetUID(), live.GetFinalizers(), live.Object["status"])
		assert.Empty(t, writes, "no update or patch was issued against the new Job")
		assert.True(t, hook.HasHookFinalizer(h.apiJob()), "the running new Job keeps the hook finalizer")
		assert.Equal(t, types.UID("uid-new"), h.apiJob().GetUID())
		require.FailNow(t, "operation completed early and leaked argocd.argoproj.io/hook-finalizer on the re-created hook")
	}

	// Pass 4: the cache has caught up with the new, running Job.
	h.pass(h.apiJob())
	require.Equal(t, synccommon.OperationRunning, h.phase)

	// Pass 5: the new Job completes; the hook succeeds and its finalizer is removed.
	done := smokeJob("uid-new", true, hook.HookFinalizer)
	_, err := h.client.Resource(jobsGVR).Namespace(testingutils.FakeArgoCDNamespace).Update(context.Background(), done, metav1.UpdateOptions{})
	require.NoError(t, err)
	cache := h.apiJob()
	mark = len(h.client.Actions())
	h.pass(cache)
	h.logActions(mark)
	assert.Equal(t, synccommon.OperationSucceeded, h.phase)
	assert.Equal(t, synccommon.OperationSucceeded, h.hookResult().HookPhase)
	assert.False(t, hook.HasHookFinalizer(h.apiJob()), "the hook finalizer is removed once the hook completes")
}

// TestSync_RecreatedHookWithTerminatingCachedPredecessor is the same race when
// the cache has already observed the start of the predecessor's (foreground)
// deletion: the cached copy carries a deletionTimestamp, health reports
// "Pending deletion", and the engine keeps waiting. The bug needs a cached
// copy without a deletionTimestamp.
func TestSync_RecreatedHookWithTerminatingCachedPredecessor(t *testing.T) {
	predecessor := smokeJob("uid-old", true)
	h := newHookRaceHarness(t, predecessor.DeepCopy())

	h.pass(predecessor)
	require.Equal(t, "waiting for deletion of hook batch/Job/web-smoke", h.message)

	terminating := predecessor.DeepCopy()
	now := metav1.Now()
	terminating.SetDeletionTimestamp(&now)
	terminating.SetFinalizers([]string{"foregroundDeletion"})

	h.pass(terminating)
	require.Equal(t, synccommon.OperationRunning, h.hookResult().HookPhase)
	require.NotNil(t, h.apiJob())

	h.pass(terminating)
	assert.Equal(t, synccommon.OperationRunning, h.phase)
	assert.Equal(t, synccommon.OperationRunning, h.hookResult().HookPhase)
	assert.Contains(t, h.hookResult().Message, "Pending deletion")
}
