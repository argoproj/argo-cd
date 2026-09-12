package e2e

import (
	"testing"

	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func TestSyncReportsWarnings(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		When().
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.Run("", "kubectl", "label", "namespace", ctx.DeploymentNamespace(),
				"pod-security.kubernetes.io/warn=restricted"))
		}).
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(Condition(ApplicationConditionSyncWarning, "The last sync completed successfully but reported warnings for"))
}
