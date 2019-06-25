package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestPreSyncHookSuccessful(t *testing.T) {
	// special-case that the pod remains in the running state, but we don't really care, because this is only used for
	// determining overall operation status is a sync with >1 wave/phase
	testHookSuccessful(t, HookTypePreSync, OperationRunning)
}

func TestSyncHookSuccessful(t *testing.T) {
	testHookSuccessful(t, HookTypeSync, OperationSucceeded)
}

func TestPostSyncHookSuccessful(t *testing.T) {
	testHookSuccessful(t, HookTypePostSync, OperationSucceeded)
}

// make sure we can run a standard sync hook
func testHookSuccessful(t *testing.T, hookType HookType, podHookPhase OperationPhase) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "%s"}}]`, hookType)).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceSyncStatusIs("Pod", "pod", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod", HealthStatusHealthy)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "hook" })).
		Expect(ResourceResultNumbering(2)).
		Expect(ResourceResultIs(ResourceResult{Version: "v1", Kind: "Pod", Namespace: DeploymentNamespace(), Name: "pod", Status: ResultCodeSynced, Message: "pod/pod created", HookPhase: podHookPhase, SyncPhase: SyncPhaseSync})).
		Expect(ResourceResultIs(ResourceResult{Version: "v1", Kind: "Pod", Namespace: DeploymentNamespace(), Name: "hook", Message: "pod/hook created", HookType: hookType, HookPhase: OperationSucceeded, SyncPhase: SyncPhase(hookType)}))
}

// make sure that if pre-sync fails, we fail the app and we do not create the pod
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
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceSyncStatusIs("Pod", "pod", SyncStatusCodeOutOfSync))
}

// make sure that if pre-sync fails, we fail the app and we did create the pod
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
		Expect(ResourceResultNumbering(2)).
		Expect(ResourceSyncStatusIs("Pod", "pod", SyncStatusCodeSynced))
}

// make sure that if post-sync fails, we fail the app and we did not create the pod
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
		Expect(ResourceResultNumbering(2)).
		Expect(ResourceSyncStatusIs("Pod", "pod", SyncStatusCodeSynced))
}

// make sure that if the pod fails, we do not run the post-sync hook
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
		Expect(ResourceSyncStatusIs("Pod", "pod", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod", HealthStatusDegraded)).
		Expect(ResourceResultNumbering(1)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

func TestSyncFailHookPodFailure(t *testing.T) {
	failureHookManifest := `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    argocd.argoproj.io/hook: SyncFail
  name: failure-hook
spec:
  containers:
    - command:
        - "true"
      image: "alpine:latest"
      imagePullPolicy: IfNotPresent
      name: main
  restartPolicy: Never
`
	Given(t).
		Path("hook").
		When().
		AddFile("failure-hook.yaml", failureHookManifest).
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PostSync"}}]`).
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		Expect(ResourceResultIs(ResourceResult{Version: "v1", Kind: "Pod", Namespace: DeploymentNamespace(), Name: "hook2", Message: "pod/failure-hook created", HookType: HookTypeSyncFail, HookPhase: OperationSucceeded, SyncPhase: SyncPhaseSyncFail})).
		Expect(OperationPhaseIs(OperationFailed))
}

func TestSyncFailHookPodFailureSyncFailFailure(t *testing.T) {
	failureHoook1Manifest := `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    argocd.argoproj.io/hook: SyncFail
  name: failure-hook
spec:
  containers:
    - command:
        - "true"
      image: "alpine:latest"
      imagePullPolicy: IfNotPresent
      name: main
  restartPolicy: Never
`
	failureHook2Manifest := `
apiVersion: v1
kind: Pod
metadata:
  annotations:
    argocd.argoproj.io/hook: SyncFail
  name: hook3
spec:
  containers:
    - command:
        - "false"
      image: "alpine:latest"
      imagePullPolicy: IfNotPresent
      name: main
  restartPolicy: Never
`
	Given(t).
		Path("hook").
		When().
		AddFile("failure-hook-1.yaml", failureHoook1Manifest).
		AddFile("failure-hook-2.yaml", failureHook2Manifest).
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PostSync"}}]`).
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command/0", "value": "false"}]`).
		Create().
		Sync().
		Then().
		Expect(ResourceResultIs(ResourceResult{Version: "v1", Kind: "Pod", Namespace: DeploymentNamespace(), Name: "failure-hook-1", Message: "pod/hook2 created", HookType: HookTypeSyncFail, HookPhase: OperationSucceeded, SyncPhase: SyncPhaseSyncFail})).
		Expect(ResourceResultIs(ResourceResult{Version: "v1", Kind: "Pod", Namespace: DeploymentNamespace(), Name: "failure-hook-2", Message: `container "main" failed with exit code 1`, HookType: HookTypeSyncFail, HookPhase: OperationFailed, SyncPhase: SyncPhaseSyncFail})).
		Expect(OperationPhaseIs(OperationFailed))
}

// make sure that we delete the hook on success
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

// make sure that we delete the hook on failure, if policy is set
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
		Expect(ResourceResultNumbering(2)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

// make sure that we do NOT delete the hook on success if failure policy is set
func TestHookDeletePolicyHookFailedHookExit0(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "add", "path": "/metadata/annotations/argocd.argoproj.io~1hook-delete-policy", "value": "HookFailed"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(ResourceResultNumbering(2)).
		Expect(Pod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

// make sure that we do delete the hook on failure if failure policy is set
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
		Expect(ResourceResultNumbering(2)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "hook" }))
}

// make sure that we never create something annotated with Skip
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
		Expect(ResourceResultNumbering(1)).
		Expect(NotPod(func(p v1.Pod) bool { return p.Name == "pod" }))
}

// make sure that we do NOT name non-hook resources in they are unnamed
func TestNamingNonHookResource(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("pod.yaml", `[{"op": "remove", "path": "/metadata/name"}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed))
}

// make sure that we name hook resources in they are unnamed
func TestAutomaticallyNamingUnnamedHook(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "remove", "path": "/metadata/name"}]`).
		// make this part of two sync tasks
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/metadata/annotations", "value": {"argocd.argoproj.io/hook": "PreSync,PostSync"}}]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			resources := app.Status.OperationState.SyncResult.Resources
			assert.Equal(t, 3, len(resources))
			// make sure we don't use the same name
			assert.Contains(t, resources[0].Name, "presync")
			assert.Contains(t, resources[2].Name, "postsync")
		})
}
