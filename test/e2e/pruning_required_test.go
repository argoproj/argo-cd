package e2e

import (
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// check we fail with message if we delete a non-prunable resource
func TestPruningRequired(t *testing.T) {
	Given(t).
		Path("two-nice-pods").
		Prune(false).
		When().
		IgnoreErrors().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		When().
		DeleteFile("pod-2.yaml").
		Sync().
		Then().
		Expect(Error("", "1 resources require pruning"))
}
