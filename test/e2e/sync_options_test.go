package e2e

import (
	"os"
	"testing"

	. "github.com/argoproj/argo-cd/engine/pkg/utils/kube/sync/common"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
)

// TestSyncOptionsValidateFalse verifies we can disable validation during kubectl apply, using the
// 'argocd.argoproj.io/sync-options: Validate=false' sync option
func TestSyncOptionsValidateFalse(t *testing.T) {
	Given(t).
		Path("sync-options-validate-false").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded))
	// NOTE: it is a bug that we do not detect this as OutOfSync. This is because we
	// are dropping fields as part of remarshalling. See: https://github.com/argoproj/argo-cd/issues/1787
	// Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

// TestSyncOptionsValidateTrue verifies when 'argocd.argoproj.io/sync-options: Validate=false' is
// not present, then validation is performed and we fail during the apply
func TestSyncOptionsValidateTrue(t *testing.T) {
	// k3s does not validate at all, so this test does not work
	if os.Getenv("ARGOCD_E2E_K3S") == "true" {
		t.SkipNow()
	}
	Given(t).
		Path("sync-options-validate-false").
		When().
		IgnoreErrors().
		Create().
		PatchFile("invalid-cm.yaml", `[{"op": "remove", "path": "/metadata/annotations"}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed))
}
