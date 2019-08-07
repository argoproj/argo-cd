package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// mostly provided as a way to easily create an app of apps for manual testing
func TestAppOfApps(t *testing.T) {
	t.SkipNow()
	Given(t).
		Path("app-of-apps").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// we are missing the child apps, as we do not auto-sync them
		Expect(HealthIs(HealthStatusMissing)).
		When().
		PatchFile("templates/guestbook.yaml", `[
	{"op": "add", "path": "/spec/syncPolicy", "value": {"automated": {"prune": true}}}
]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// now we are in sync
		Expect(HealthIs(HealthStatusMissing))
}
