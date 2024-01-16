package e2e

import (
	"context"
	"os"
	"testing"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v2/util/errors"
)

// TestSyncOptionsValidateFalse verifies we can disable validation during kubectl apply, using the
// 'argocd.argoproj.io/sync-options: Validate=false' sync option
func TestSyncOptionsValidateFalse(t *testing.T) {
	Given(t).
		Path("sync-options-validate-false").
		When().
		CreateApp().
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
		CreateApp().
		PatchFile("invalid-cm.yaml", `[{"op": "remove", "path": "/metadata/annotations"}]`).
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed))
}

func TestSyncWithStatusIgnored(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		And(func() {
			fixture.SetResourceOverrides(map[string]ResourceOverride{
				"/": {
					IgnoreDifferences: OverrideIgnoreDiff{JSONPointers: []string{"/status"}},
				},
			})
		}).
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{SelfHeal: true}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should remain synced if git change detected
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "add", "path": "/status", "value": { "observedGeneration": 1 }}]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should remain synced if k8s change detected
		When().
		And(func() {
			errors.FailOnErr(fixture.KubeClientset.AppsV1().Deployments(fixture.DeploymentNamespace()).Patch(context.Background(),
				"guestbook-ui", types.JSONPatchType, []byte(`[{ "op": "replace", "path": "/status/observedGeneration", "value": 2 }]`), v1.PatchOptions{}))
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestSyncWithSkipHook(t *testing.T) {
	fixture.SkipOnEnv(t, "OPENSHIFT")
	Given(t).
		Path(guestbookPath).
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{SelfHeal: true}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should remain synced when app has skipped annotation even if git change detected
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "add", "path": "/metadata/annotations", "value": { "argocd.argoproj.io/hook": "Skip" }}]`).
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "replace", "path": "/spec/replicas", "value": 1 }]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		// app should not remain synced if skipped annotation removed
		When().
		PatchFile("guestbook-ui-deployment.yaml", `[{ "op": "remove", "path": "/metadata/annotations" }]`).
		Refresh(RefreshTypeNormal).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}
