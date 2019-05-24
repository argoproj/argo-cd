package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

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

func TestSelectiveSyncDoesNotRunHooks(t *testing.T) {
	Given(t).
		Path("hook").
		SelectedResource(":Pod:pod").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		// you might not expect this, but yes - it is is sync
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(HealthStatusMissing)).
		Expect(ResourceHealthIs("Pod", "pod", HealthStatusHealthy)).
		Expect(ResourceHealthIs("Pod", "hook", HealthStatusMissing))
}
