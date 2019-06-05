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
		Create().
		PatchFile("pod-1.yaml", `[{"op": "replace", "path": "/spec/containers/0/image", "value": "rubbish"}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(HealthStatusMissing)).
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
		Expect(ResourceSyncStatusIs("Pod", "pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("Pod", "pod-2", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("Pod", "pod-2", HealthStatusHealthy))
}
