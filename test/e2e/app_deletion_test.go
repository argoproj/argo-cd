package e2e

import (
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// when a app gets stuck in sync, and we try to delete it, it won't delete, instead we must then terminate it
// and deletion will then just happen
func TestDeletingAppStuckInSync(t *testing.T) {
	Given(t).
		And(func() {
			SetResourceOverrides(map[string]ResourceOverride{
				"ConfigMap": {
					HealthLua: `return { status = obj.annotations and obj.annotations['health'] or 'Progressing' }`,
				},
			})
		}).
		Async(true).
		Path("hook-custom-health").
		When().
		Create().
		Sync().
		Then().
		// stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Delete(true).
		Then().
		// delete is ignored, still stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		When().
		TerminateOp().
		Then().
		// delete is successful
		Expect(DoesNotExist())
}
