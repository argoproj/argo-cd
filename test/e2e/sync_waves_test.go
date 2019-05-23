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
		Sync().
		Then().
		Expect(ResourceSyncStatusIs("pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("pod-2", SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("pod-2", HealthStatusMissing)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(HealthStatusMissing)).
		When().
		TerminateOp().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		When().
		PatchFile("pod-1.yaml", `[{"op": "replace", "path": "/spec/containers/0/image", "value": "nginx"}]`).
		Sync().
		Then().
		Expect(ResourceSyncStatusIs("pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("pod-2", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-2", HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(OperationPhaseIs(OperationSucceeded))
}
