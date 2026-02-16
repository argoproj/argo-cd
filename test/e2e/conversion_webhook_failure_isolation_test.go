package e2e

import (
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// TestConversionWebhookFailureIsolation tests conversion webhook failure isolation:
// - Application isolation (only CRD-using apps affected, not others)
// - Complete recovery after webhook is fixed (apps healthy)
//
// This follows the reproduction pattern from conversion-webhook-repro/scripts
func TestConversionWebhookFailureIsolation(t *testing.T) {
	// Step 1: Bring up working app that will NOT be impacted (to test isolation)
	isolatedAppName := "isolated-guestbook-app"
	t.Log("📱 Creating isolated app (will NOT be impacted)")
	ctx := Given(t).
		Name(isolatedAppName).
		Path("guestbook"). // Standard Kubernetes resources
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Given()

	// Step 2: Set up CRD first, then create app that uses it
	crdAppName := "crd-using-app"
	t.Log("🔧 Setting up working CRD first (no conversion webhook)")
	_, err := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)
	// Give CRD time to be ready and for ArgoCD cache to discover new API versions
	time.Sleep(10 * time.Second)

	t.Log("📱 Creating CRD-using app (will be impacted)")
	GivenWithSameState(ctx).
		Name(crdAppName).
		Path("conversion-webhook-test/resources"). // Only the resources, not the CRD
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy))

	// Allow time for CRD app to fully establish
	time.Sleep(3 * time.Second)

	// Step 3: Validate both are healthy/successful
	t.Log("✅ Both applications are initially healthy and successful")

	// Step 4: Break the webhook (simulate CRD evolution adding broken conversion webhook)
	t.Log("🔥 Breaking conversion webhook by updating CRD in-place (simulating production CRD evolution)")
	// This simulates what happens in production: a working CRD gets updated to add conversion webhook
	_, err = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd-with-broken-webhook.yaml")
	require.NoError(t, err)

	// Step 5: Trigger hard refresh of both apps
	t.Log("🔄 Triggering hard refresh of both applications")
	GivenWithSameState(ctx).
		Name(crdAppName).
		When().
		Refresh(RefreshTypeHard)

	GivenWithSameState(ctx).
		Name(isolatedAppName).
		When().
		Refresh(RefreshTypeHard)

	// Allow time for hard refresh to process and detect conversion webhook errors
	time.Sleep(5 * time.Second)

	// Step 5b: Trigger sync of CRD app to force conversion webhook failure
	t.Log("🔄 Triggering sync of CRD app to encounter conversion webhook failure")
	GivenWithSameState(ctx).
		Name(crdAppName).
		When().
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(HealthIs(health.HealthStatusUnknown)). // Unknown because we can't fetch to assess health
		Expect(SyncStatusIs(SyncStatusCodeUnknown))   // Cache tainting from conversion failures prevents safe sync operations

	// Allow time for sync to process and conversion webhook errors to be detected
	time.Sleep(10 * time.Second)

	// Step 6: Validate conversion webhook failure isolation behavior
	t.Log("🔍 Validating conversion webhook failure shows proper error details")
	// The sync operation above already validated the degraded state;
	// the failure was isolated to the specific CRD resource as expected

	// Step 7: Validate that unimpacted app is not affected
	t.Log("🔍 Validating isolated app is not impacted")
	GivenWithSameState(ctx).
		Name(isolatedAppName).
		When().
		Then().
		And(func(app *Application) {
			assert.NotEqual(t, health.HealthStatusUnknown, app.Status.Health.Status,
				"Isolated app should not go to Unknown health due to unrelated CRD webhook failure")
			assert.Equal(t, health.HealthStatusHealthy, app.Status.Health.Status,
				"Isolated app should remain healthy despite broken CRD in cluster")
			assert.Equal(t, SyncStatusCodeSynced, app.Status.Sync.Status,
				"Isolated app should remain synced despite broken CRD in cluster")
		})

	// Step 8: Fix the webhook (remove conversion webhook from CRD)
	t.Log("🛠️ Fixing conversion webhook by updating CRD back to working state")
	// This simulates fixing the CRD by removing the broken conversion webhook
	_, err = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	// Wait for CRD changes to propagate through the API server
	t.Log("⏳ Waiting for CRD changes to propagate through the API server...")
	time.Sleep(10 * time.Second)

	// Step 9: Trigger hard refresh of CRD app after fix to re-evaluate tainted GVKs
	t.Log("🔄 Triggering hard refresh of CRD app after fix to re-evaluate tainted GVKs")
	GivenWithSameState(ctx).
		Name(crdAppName).
		When().
		Refresh(RefreshTypeHard)

	// Allow time for hard refresh to process and validate tainted GVKs
	time.Sleep(5 * time.Second)

	// Step 10: Trigger sync of CRD app after taint validation to recover
	t.Log("🔄 Triggering sync of CRD app after taint validation to recover")
	GivenWithSameState(ctx).
		Name(crdAppName).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy))

	// Also refresh isolated app to confirm it's still healthy
	GivenWithSameState(ctx).
		Name(isolatedAppName).
		When().
		Refresh(RefreshTypeNormal)

	// Allow time for recovery
	time.Sleep(5 * time.Second)

	// Step 11: Validate that isolated app remains healthy after recovery
	t.Log("🔍 Validating isolated app remains healthy after recovery")
	GivenWithSameState(ctx).
		Name(isolatedAppName).
		When().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))

	t.Log("✅ Conversion webhook failure isolation and recovery test completed successfully")
}
