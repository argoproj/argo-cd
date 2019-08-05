package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// when a app gets stuck in sync, and we try to delete it, it won't delete, instead we must then terminate it
// and deletion will then just happen
func TestDeletingAppStuckInSync(t *testing.T) {
	Given(t).
		Async(true).
		Path("hook").
		When().
		PatchFile("hook.yaml", `[{"op": "replace", "path": "/spec/containers/0/command", "value": ["sh", "-c", "until ls /tmp/done; do sleep 0.1; done"]}]`).
		Create().
		Sync().
		Then().
		// stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceResultNumbering(2)).
		When().
		Delete(true).
		Then().
		// delete is ignored, still stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		When().
		TerminateOp().
		And(func() {
			// force delete the resource. don't fail if whole already deleted
			_, _ = Run("", "kubectl", "-n", DeploymentNamespace(), "exec", "-i", "hook", "touch", "/tmp/done")
		}).
		Then().
		// delete is successful
		Expect(DoesNotExist())
}
