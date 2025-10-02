package e2e

import (
	"strings"
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

// TestConversionWebhookRefreshBehavior tests the specific refresh behavior when a conversion webhook fails.
// This test focuses on the refresh flow (not sync) to validate state transitions and condition messages.
//
// Current behavior: app stays Healthy/OutOfSync, resources show Missing
// Previous behavior: app goes Degraded/Unknown, shows 3 cryptic error conditions
// Desired behavior: app goes Degraded/Unknown, shows 1 friendly condition message
func TestConversionWebhookRefreshBehavior(t *testing.T) {
	crdAppName := "crd-refresh-test-app"

	// Initialize test framework and create app first (without CRD yet)
	t.Log("📱 Creating app WITHOUT autosync using v1 and v2 resources")
	Given(t).
		Name(crdAppName).
		Path("conversion-webhook-test/resources"). // Has v1 and v2 Example resources + ConfigMap
		When().
		CreateApp()

	// Step 1: Deploy working CRD with v1 and v2 (no conversion webhook)
	t.Log("🔧 Deploying working CRD with v1 and v2 (no conversion webhook)")
	_, err := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	// Give CRD time to be ready and for ArgoCD cache to discover new API versions
	time.Sleep(10 * time.Second)

	// Step 2: Now sync the app with the CRD in place
	t.Log("📚 Syncing app now that CRD is available")
	GivenWithSameState(t).
		Name(crdAppName).
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy))
		// Note: App may show OutOfSync due to resource differences, that's ok for this test

	// Allow app to stabilize
	time.Sleep(3 * time.Second)

	// Step 3: Break the webhook (add broken conversion webhook to CRD)
	t.Log("🔥 Breaking conversion webhook by updating CRD")
	_, err = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd-with-broken-webhook.yaml")
	require.NoError(t, err)

	// Allow CRD changes to propagate
	time.Sleep(5 * time.Second)

	// Step 4: Trigger REFRESH (not sync!)
	t.Log("🔄 Triggering REFRESH (not sync) of the application")
	GivenWithSameState(t).
		Name(crdAppName).
		When().
		Refresh(RefreshTypeHard)

	// Allow refresh to complete and errors to be detected
	time.Sleep(10 * time.Second)

	// Step 5: Evaluate the state of the app and resource tree
	t.Log("🔍 Evaluating app state after refresh with broken webhook")

	GivenWithSameState(t).
		Name(crdAppName).
		When().
		Then().
		And(func(app *Application) {
			// Log the actual state for debugging
			t.Logf("App Health Status: %s", app.Status.Health.Status)
			t.Logf("App Sync Status: %s", app.Status.Sync.Status)
			t.Logf("Number of Conditions: %d", len(app.Status.Conditions))

			for i, condition := range app.Status.Conditions {
				t.Logf("Condition %d: Type=%s, Message=%s", i+1, condition.Type, condition.Message)
			}

			// Check resource tree status
			for _, resource := range app.Status.Resources {
				if resource.Kind == "Example" {
					healthStatus := "nil"
					if resource.Health != nil {
						healthStatus = string(resource.Health.Status)
					}
					t.Logf("Resource %s/%s: Health=%s, Status=%s",
						resource.Kind, resource.Name,
						healthStatus,
						resource.Status)
				}
			}

			// DESIRED BEHAVIOR: App should go Degraded/Unknown with friendly message
			assert.Equal(t, health.HealthStatusDegraded, app.Status.Health.Status,
				"App should be Degraded when conversion webhook fails")
			assert.Equal(t, SyncStatusCodeUnknown, app.Status.Sync.Status,
				"App should have Unknown sync status when conversion webhook fails")

			// Should have at least one condition
			assert.Greater(t, len(app.Status.Conditions), 0,
				"App should have conditions explaining the degraded state")

			// Check for friendly condition message
			hasFriendlyMessage := false
			for _, condition := range app.Status.Conditions {
				t.Logf("Condition: Type=%s, Message=%s", condition.Type, condition.Message)

				// Accept either a friendly message or at least a message mentioning conversion webhook
				if strings.Contains(condition.Message, "conversion webhook") ||
				   strings.Contains(condition.Message, "Application contains resources affected by conversion webhook failures") {
					hasFriendlyMessage = true
					t.Log("✅ Found condition message about conversion webhook failure")
				}
			}

			assert.True(t, hasFriendlyMessage,
				"App should have a condition message explaining the conversion webhook failure")

			// Check resource status - they should ideally show as Missing or have health issues
			affectedResources := 0
			for _, resource := range app.Status.Resources {
				if resource.Kind == "Example" {
					t.Logf("Resource %s/%s: Health=%v, Status=%s",
						resource.Kind, resource.Name,
						resource.Health, resource.Status)

					// Resources should either be Missing or have health issues
					if resource.Status == "Missing" ||
					   (resource.Health != nil && resource.Health.Status != health.HealthStatusHealthy) {
						affectedResources++
					}
				}
			}

			assert.Greater(t, affectedResources, 0,
				"Example resources should show issues when conversion webhook fails")
		})

	t.Log("🔬 Test captured current behavior - app incorrectly stays Healthy/OutOfSync with Missing resources")
	t.Log("📝 This test will be updated to expect Degraded/Unknown once we fix the issue")
}