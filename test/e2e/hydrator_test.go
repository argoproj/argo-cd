package e2e

import (
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

func TestSimpleHydrator(t *testing.T) {
	Given(t).
		DrySourcePath("guestbook").
		DrySourceRevision("HEAD").
		SyncSourcePath("guestbook").
		SyncSourceBranch("env/test").
		When().
		CreateApp().
		Refresh(RefreshTypeNormal).
		Wait("--hydrated").
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
