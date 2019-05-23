package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "k8s.io/api/core/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestPreSyncHookSuccessful(t *testing.T) {
	testHookSuccessful(t, HookTypePreSync)
}

func TestSyncHookSuccessful(t *testing.T) {
	testHookSuccessful(t, HookTypeSync)
}

func TestPostSyncHookSuccessful(t *testing.T) {
	testHookSuccessful(t, HookTypePostSync)
}

func testHookSuccessful(t *testing.T, hookType HookType) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "%s"}}]`, hookType)).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("pod", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod", HealthStatusHealthy)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestPreSyncHookFailure(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PreSync"}}]`).
		// make hook fail
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command", "value": ["false"]}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		// if a pre-sync hook fails, we should not start the main sync
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceSyncStatusIs("pod", SyncStatusCodeOutOfSync))
}

func TestSyncHookFailure(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		// make hook fail
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		// even thought the hook failed, we expect the pod to be in sync
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("pod", SyncStatusCodeSynced))
}

func TestPostSyncHookFailure(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PostSync"}}]`).
		// make hook fail
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("pod", SyncStatusCodeSynced))
}

func TestPostSyncHookPodFailure(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PostSync"}}]`).
		// make pod fail
		PatchFile("pod.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		// TODO - I feel like this should be a failure, not success
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("pod", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod", HealthStatusDegraded)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestHookDeletePolicyHookSucceededHookExit0(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations/argocd.argoproj.io~1hook-delete-policy", "value": "HookSucceeded"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestHookDeletePolicyHookSucceededHookExit1(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations/argocd.argoproj.io~1hook-delete-policy", "value": "HookSucceeded"}]`).
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestHookDeletePolicyHookFailedHookExit0(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations/argocd.argoproj.io~1hook-delete-policy", "value": "HookFailed"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestHookDeletePolicyHookFailedHookExit1(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations/argocd.argoproj.io~1hook-delete-policy", "value": "HookFailed"}]`).
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestHookSkip(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		// should not create this pod
		PatchFile("pod.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "Skip"}}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "pod" }))
}

func TestNamingNonHookResource(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("pod.yaml", `[{"op": "remove", "path": "/metadata/name"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		And(func(app *Application) {
			assert.Equal(t, ResultCodeSyncFailed, app.Status.OperationState.SyncResult.Resources[0].Status)
		})
}

func TestAutomaticallyNamingUnnamedHook(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "remove", "path": "/metadata/name"}]`).
		// make this part of two syncs
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PreSync,PostSync"}}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			assert.Contains(t, "presync", app.Status.Resources[0].Name)
			assert.Contains(t, "postsync", app.Status.Resources[2].Name)
		})
}
