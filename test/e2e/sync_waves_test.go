package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

func TestFixingDegradedApp(t *testing.T) {
	Given(t).
		Path("sync-waves").
		When().
		IgnoreErrors().
		Create().
		PatchFile("pod-1.yaml", `[{"op": "replace", "path": "/spec/containers/0/image", "value": "rubbish"}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(HealthStatusMissing)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-1", HealthStatusDegraded)).
		Expect(ResourceSyncStatusIs("Pod", "pod-2", SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("Pod", "pod-2", HealthStatusMissing)).
		When().
		PatchFile("pod-1.yaml", `[{"op": "replace", "path": "/spec/containers/0/image", "value": "nginx"}]`).
		// need to force a refresh here
		Refresh(RefreshTypeNormal).
		Then().
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeOutOfSync)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(HealthStatusMissing)).
		Expect(ResourceResultNumbering(1)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("Pod", "pod-2", SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("Pod", "pod-2", HealthStatusMissing)).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(ResourceResultNumbering(2)).
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("Pod", "pod-2", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-2", HealthStatusHealthy))
}

func TestOneProgressingDeploymentIsSucceededAndSynced(t *testing.T) {
	Given(t).
		Path("one-deployment").
		When().
		// make this deployment get stuck in progressing due to "invalidimagename"
		PatchFile("deployment.yaml", `[
    {
        "op": "replace",
        "path": "/spec/template/spec/containers/0/image",
        "value": "alpine:ops!"
    }
]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusProgressing)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(1))
}

func TestDegradedDeploymentIsSucceededAndSynced(t *testing.T) {
	Given(t).
		Path("one-deployment").
		When().
		// make this deployment get stuck in progressing due to "invalidimagename"
		PatchFile("deployment.yaml", `[
    {
        "op": "replace",
        "path": "/spec/progressDeadlineSeconds",
        "value": 1
    },
    {
        "op": "replace",
        "path": "/spec/template/spec/containers/0/image",
        "value": "alpine:ops!"
    }
]`).
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(HealthStatusDegraded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(1))
}
