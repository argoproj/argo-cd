package e2e

import (
	"testing"
	//. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	//. "github.com/argoproj/gitops-engine/pkg/sync/common"
)

func TestSimpleHydrator(t *testing.T) {
	// FIXME: This test almost works. The problem is that the sync happens too quickly after the refresh, before
	// hydration is finished. I think we need a `wait` command for hydration.
	t.Skipped()

	// Given(t).
	//	DrySourcePath("guestbook").
	//	DrySourceRevision("HEAD").
	//	SyncSourcePath("guestbook").
	//	SyncSourceBranch("env/test").
	//	When().
	//	CreateApp().
	//	Refresh(RefreshTypeNormal).
	//	Sync().
	//	Then().
	//	Expect(OperationPhaseIs(OperationSucceeded)).
	//	Expect(SyncStatusIs(SyncStatusCodeSynced))
}
