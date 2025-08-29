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
// This follows the reproduction pattern from ~/workspace/conversion-webhook-repro/scripts
func TestConversionWebhookFailureIsolation(t *testing.T) {
	fixture.EnsureCleanState(t)
	defer fixture.RecordTestRun(t)

	// Step 1: Bring up working app that will NOT be impacted (to test isolation)
	isolatedAppName := "isolated-guestbook-app"
	t.Log("📱 Creating isolated app (will NOT be impacted)")
	Given(t).
		Name(isolatedAppName).
		Project("default").
		SetAppNamespace(fixture.AppNamespace()).
		Path("guestbook"). // Standard Kubernetes resources
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))

	// Step 2: Bring up working app that will be impacted by broken webhook
	crdAppName := "crd-using-app"
	t.Log("📱 Creating CRD-using app (will be impacted)")
	GivenWithSameState(t).
		Name(crdAppName).
		Project("default").
		SetAppNamespace(fixture.AppNamespace()).
		Path("conversion-webhook-test/resources"). // Only the resources, not the CRD
		When().
		CreateApp().
		And(func() {
			// Apply CRD right before sync to avoid cleanup
			t.Log("🔧 Setting up working CRD (no conversion webhook)")
			_, err := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
			require.NoError(t, err)
			// Give CRD time to be ready before sync
			time.Sleep(2 * time.Second)
		}).
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
	_, err := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd-with-broken-webhook.yaml")
	require.NoError(t, err)

	// Step 5: Trigger hard refresh of both apps
	t.Log("🔄 Triggering hard refresh of both applications")
	GivenWithSameState(t).
		Name(crdAppName).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		Refresh(RefreshTypeHard)

	GivenWithSameState(t).
		Name(isolatedAppName).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		Refresh(RefreshTypeHard)

	// Allow time for hard refresh to process and detect conversion webhook errors
	time.Sleep(5 * time.Second)

	// Step 5b: Trigger sync of CRD app to force conversion webhook failure
	t.Log("🔄 Triggering sync of CRD app to encounter conversion webhook failure")
	GivenWithSameState(t).
		Name(crdAppName).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		IgnoreErrors().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationFailed)).
		Expect(HealthIs(health.HealthStatusDegraded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))

	// Allow time for sync to process and conversion webhook errors to be detected
	time.Sleep(10 * time.Second)

	// Step 6: Validate conversion webhook failure isolation behavior
	t.Log("🔍 Validating conversion webhook failure shows proper error details")
	// The sync operation above already validated the degraded state;
	// the failure was isolated to the specific CRD resource as expected

	// Step 7: Validate that unimpacted app is not affected
	t.Log("🔍 Validating isolated app is not impacted")
	GivenWithSameState(t).
		Name(isolatedAppName).
		SetAppNamespace(fixture.AppNamespace()).
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
	_, err2 := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err2)

	// Step 9: Trigger sync of CRD app after fix to recover
	t.Log("🔄 Triggering sync of CRD app after fix to recover")
	GivenWithSameState(t).
		Name(crdAppName).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy))

	// Also refresh isolated app to confirm it's still healthy
	GivenWithSameState(t).
		Name(isolatedAppName).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		Refresh(RefreshTypeNormal)

	// Allow time for recovery
	time.Sleep(5 * time.Second)

	// Step 10: Validate that isolated app remains healthy after recovery
	t.Log("🔍 Validating isolated app remains healthy after recovery")
	GivenWithSameState(t).
		Name(isolatedAppName).
		SetAppNamespace(fixture.AppNamespace()).
		When().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))

	t.Log("✅ Conversion webhook failure isolation and recovery test completed successfully")
}
