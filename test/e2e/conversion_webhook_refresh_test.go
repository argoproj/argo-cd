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

	// CRITICAL: Set up cleanup to fix the broken webhook BEFORE automatic CRD deletion
	// This prevents the test from hanging during cleanup
	defer func() {
		t.Log("🧹 Cleanup: Removing broken webhook from CRD to allow clean deletion")
		// Apply the working CRD (without webhook) to unblock deletion
		_, _ = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
		time.Sleep(2 * time.Second)
	}()

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
			assert.NotEmpty(t, app.Status.Conditions,
				"App should have conditions explaining the degraded state")

			// Check for the expected condition - we expect exactly one ComparisonError with our specific message
			var comparisonErrorCondition *ApplicationCondition
			for i, condition := range app.Status.Conditions {
				t.Logf("Condition: Type=%s, Message=%s", condition.Type, condition.Message)
				if condition.Type == ApplicationConditionComparisonError {
					comparisonErrorCondition = &app.Status.Conditions[i]
				}
			}

			require.NotNil(t, comparisonErrorCondition,
				"App should have a ComparisonError condition")

			// Check the exact message format we generate in appcontroller.go
			expectedMessagePrefix := "Application contains resources affected by conversion webhook failures"
			assert.True(t, strings.HasPrefix(comparisonErrorCondition.Message, expectedMessagePrefix),
				"Condition message should start with '%s', got: %s", expectedMessagePrefix, comparisonErrorCondition.Message)

			// The message should also include the affected GVK
			assert.Contains(t, comparisonErrorCondition.Message, "conversion.example.com",
				"Condition message should include the affected GVK group")
			t.Log("✅ Found expected ComparisonError condition with conversion webhook failure message")

			// Note: Individual resource statuses may still show their last known state (Synced/OutOfSync)
			// because the conversion webhook failure prevents accurate comparison.
			// The important indicators are the app-level Degraded health, Unknown sync status,
			// and the condition message explaining the problem.
			t.Log("✅ App correctly shows Degraded/Unknown status with conversion webhook error condition")
		})
}
