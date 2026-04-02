package e2e

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestConversionWebhookFailureIsolation tests that a broken conversion webhook
// only affects applications using that CRD, not other applications on the same
// cluster. This validates the per-GVK error isolation implemented in the cluster
// cache sync and OpenAPI v3 lazy parser.
//
// Adapted from github.com/argoproj/argo-cd/pull/23425 and the reproduction
// scenario at github.com/jcogilvie/conversion-webhook-repro.
func TestConversionWebhookFailureIsolation(t *testing.T) {
	// Step 1: Create an isolated app (standard k8s resources, unrelated to CRD)
	ctx := Given(t).
		Path("guestbook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given()

	// Step 2: Deploy working CRD (no conversion webhook)
	_, err := fixture.Run("", "kubectl", "apply", "-f",
		fixture.TmpDir()+"/testdata.git"+"/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	// Ensure cleanup removes the CRD even if the test fails mid-way
	defer func() {
		// Restore working CRD first (in case broken webhook blocks deletion)
		_, _ = fixture.Run("", "kubectl", "apply", "-f",
			fixture.TmpDir()+"/testdata.git"+"/conversion-webhook-test/crds/crd.yaml")
		time.Sleep(2 * time.Second)
		_, _ = fixture.Run("", "kubectl", "delete", "crd",
			"examples.conversion.example.com", "--ignore-not-found")
	}()

	// Give CRD time to be ready and ArgoCD cache to discover new API
	time.Sleep(10 * time.Second)

	// Step 3: Create app that uses the CRD
	crdCtx := GivenWithSameState(ctx).
		Name("crd-using-app").
		Path("conversion-webhook-test/resources").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Given()

	time.Sleep(3 * time.Second)

	// Step 4: Break the webhook (update CRD to point to non-existent webhook service)
	_, err = fixture.Run("", "kubectl", "apply", "-f",
		fixture.TmpDir()+"/testdata.git"+"/conversion-webhook-test/crds/crd-with-broken-webhook.yaml")
	require.NoError(t, err)

	// Step 5: Hard refresh both apps to force cache re-sync
	GivenWithSameState(crdCtx).
		When().
		Refresh(RefreshTypeHard)

	GivenWithSameState(ctx).
		When().
		Refresh(RefreshTypeHard)

	time.Sleep(10 * time.Second)

	// Step 6: Verify isolated app is NOT affected
	GivenWithSameState(ctx).
		When().
		Then().
		And(func(app *Application) {
			assert.Equal(t, health.HealthStatusHealthy, app.Status.Health.Status,
				"Isolated app should remain healthy despite broken CRD webhook")
			assert.Equal(t, SyncStatusCodeSynced, app.Status.Sync.Status,
				"Isolated app should remain synced despite broken CRD webhook")
		})

	// Step 7: Fix the webhook (restore working CRD)
	_, err = fixture.Run("", "kubectl", "apply", "-f",
		fixture.TmpDir()+"/testdata.git"+"/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	time.Sleep(10 * time.Second)

	// Step 8: Hard refresh and sync the CRD app to verify recovery
	GivenWithSameState(crdCtx).
		When().
		Refresh(RefreshTypeHard)

	time.Sleep(5 * time.Second)

	GivenWithSameState(crdCtx).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy))

	// Step 9: Verify isolated app is still healthy after recovery
	GivenWithSameState(ctx).
		When().
		Refresh(RefreshTypeNormal)

	time.Sleep(3 * time.Second)

	GivenWithSameState(ctx).
		When().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
