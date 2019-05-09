package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixtures"
)

func TestSyncWavesStopsOnDegraded(t *testing.T) {
	fixture.NewApp(t, "sync-waves").
		Sync().
		Error().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusDegraded)).
		Expect(ResourceSyncStatusIs("pod-1", SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("pod-2", HealthStatusMissing))
}

func TestHooks(t *testing.T) {
	fixture.NewApp(t, "hooks/happy-path").
		Sync().
		Error().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(ResourceHealthIs("hook", HealthStatusHealthy)).
		Expect(ResourceHealthIs("pod", HealthStatusHealthy))
}
