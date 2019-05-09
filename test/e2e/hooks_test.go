package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixtures"
)

func TestSyncWavesStopsOnDegraded(t *testing.T) {
	fixture.NewApp(t, "sync-waves").
		Sync().
		Expect(ResourceSyncStatusIs("pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-2", HealthStatusMissing)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(HealthStatusMissing))
}

func TestFixingDegradedApp(t *testing.T) {
	fixture.NewApp(t, "sync-waves").
		Sync().
		Expect(ResourceSyncStatusIs("pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("pod-2", SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("pod-2", HealthStatusMissing)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(HealthIs(HealthStatusMissing)).
		TerminateOp().
		Expect(OperationPhaseIs(OperationFailed)).
		Patch("pod-1.yaml", `[{"op": "replace", "path": "/spec/containers/0/image", "value": "nginx"}]`).
		Sync().
		Expect(ResourceSyncStatusIs("pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-1", HealthStatusHealthy)).
		Expect(ResourceSyncStatusIs("pod-2", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-2", HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(OperationPhaseIs(OperationSucceeded))
}

func TestHooks(t *testing.T) {
	fixture.NewApp(t, "hooks/happy-path").
		Sync().
		Expect(ResourceHealthIs("hook", HealthStatusHealthy)).
		Expect(ResourceHealthIs("pod", HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy))
}
