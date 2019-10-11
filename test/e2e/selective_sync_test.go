package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// when you selectively sync, only seleceted resources should be synced, but the app will be out of sync
func TestSelectiveSync(t *testing.T) {
	Given(t).
		Path("guestbook").
		SelectedResource(":Service:guestbook-ui").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("Service", "guestbook-ui", HealthStatusHealthy)).
		Expect(ResourceHealthIs("Deployment", "guestbook-ui", HealthStatusMissing))
}

// when running selective sync, hooks do not run
func TestSelectiveSyncDoesRunHooks(t *testing.T) {
	Given(t).
		Path("hook").
		SelectedResource(":Pod:pod").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusHealthy)).
		Expect(ResourceHealthIs("Pod", "pod", HealthStatusHealthy)).
		Expect(ResourceHealthIs("Pod", "hook", HealthStatusHealthy))
}
