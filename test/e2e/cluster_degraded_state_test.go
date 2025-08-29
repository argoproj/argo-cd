package e2e

import (
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/require"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	clusterFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/cluster"
)

// TestClusterDegradedState tests that when a cluster has failed resources,
// it shows as degraded in the API and contains the failedResourceGVKs
func TestClusterDegradedState(t *testing.T) {
	// Create a test cluster - use Given() to set up clean state
	clusterFixture.
		Given(t).
		Name("test-degraded").
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		CreateWithRBAC()

	// Wait for the cluster to be registered and synced, and for cache invalidation to fully take effect
	time.Sleep(5 * time.Second)

	// First verify the cluster is in a healthy state
	clusterFixture.VerifyHealthyClusterState(t, KubernetesInternalAPIServerAddr)

	// Set up real conversion webhook failure by first applying working CRD
	t.Log("🔧 Setting up working CRD (no conversion webhook)")
	_, err := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	// Give CRD time to be ready
	time.Sleep(2 * time.Second)

	// Create an ArgoCD application that manages the conversion webhook resources
	// This ensures ArgoCD has ownership and will attempt to list the GVK
	t.Log("📱 Creating ArgoCD application to manage conversion webhook resources")
	GivenWithSameState(t).
		Path("conversion-webhook-test/resources").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy))

	// Wait for application to be fully synced and resources cached
	time.Sleep(3 * time.Second)

	// Now break the webhook by updating CRD in-place (simulating production CRD evolution)
	t.Log("🔥 Breaking conversion webhook by updating CRD in-place")
	_, err = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd-with-broken-webhook.yaml")
	require.NoError(t, err)

	// Trigger cluster cache invalidation to force detection of conversion webhook failures
	t.Log("🔄 Invalidating cluster cache to trigger conversion webhook failure detection")
	clusterURL := fixture.URLEncodeServerAddress(KubernetesInternalAPIServerAddr)
	var invalidateResult Cluster
	err = fixture.DoHttpJsonRequest("POST", "/api/v1/clusters/"+clusterURL+"/invalidate-cache", &invalidateResult)
	require.NoError(t, err)

	// Follow up with application hard refresh to force cache sync and trigger conversion webhook failure detection
	t.Log("🔄 Hard refreshing application to force re-examination of conversion webhook GVKs")
	GivenWithSameState(t).
		When().
		Refresh(RefreshTypeHard)

	// Wait for cache invalidation, refresh, and conversion webhook failure detection
	// Also wait for cluster info updater to run and populate failedResourceGVKs
	time.Sleep(10 * time.Second)

	// Verify that the cluster is now in a degraded state and the field appears in the API
	clusterFixture.VerifyFailedResourcesInResponse(t, KubernetesInternalAPIServerAddr)

	t.Log("✅ Conversion webhook failure detected and cluster degraded state validated via API")

	// Clean up the real conversion webhook failure resources
	t.Log("🛠️ Cleaning up conversion webhook failure resources")
	_, _ = fixture.Run("", "kubectl", "delete", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/resources/example-resource-v2.yaml", "--ignore-not-found")
	_, _ = fixture.Run("", "kubectl", "delete", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd-with-broken-webhook.yaml", "--ignore-not-found")
}

// TestClusterDegradedStateRecovery tests that when a cluster recovers from failures,
// it transitions back to healthy and clears the failedResourceGVKs
func TestClusterDegradedStateRecovery(t *testing.T) {
	// Create a test cluster
	clusterFixture.
		GivenWithSameState(t).
		Name("test-recovery").
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		CreateWithRBAC()

	// Wait for the cluster to be registered and synced
	time.Sleep(2 * time.Second)

	// Set up real conversion webhook failure by first applying working CRD
	t.Log("🔧 Setting up working CRD (no conversion webhook)")
	_, err := fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	// Give CRD time to be ready
	time.Sleep(2 * time.Second)

	// Create an ArgoCD application that manages the conversion webhook resources
	// This ensures ArgoCD has ownership and will attempt to list the GVK
	t.Log("📱 Creating ArgoCD application to manage conversion webhook resources")
	GivenWithSameState(t).
		Path("conversion-webhook-test/resources").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(HealthIs(health.HealthStatusHealthy))

	// Wait for application to be fully synced and resources cached
	time.Sleep(3 * time.Second)

	// Now break the webhook by updating CRD in-place (simulating production CRD evolution)
	t.Log("🔥 Breaking conversion webhook by updating CRD in-place")
	_, err = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd-with-broken-webhook.yaml")
	require.NoError(t, err)

	// Trigger cluster cache invalidation to force detection of conversion webhook failures
	t.Log("🔄 Invalidating cluster cache to trigger conversion webhook failure detection")
	clusterURL := fixture.URLEncodeServerAddress(KubernetesInternalAPIServerAddr)
	var invalidateResult Cluster
	err = fixture.DoHttpJsonRequest("POST", "/api/v1/clusters/"+clusterURL+"/invalidate-cache", &invalidateResult)
	require.NoError(t, err)

	// Follow up with application hard refresh to force cache sync and trigger conversion webhook failure detection
	t.Log("🔄 Hard refreshing application to force re-examination of conversion webhook GVKs")
	GivenWithSameState(t).
		When().
		Refresh(RefreshTypeHard)

	// Wait for cache invalidation, refresh, and conversion webhook failure detection
	time.Sleep(5 * time.Second)

	// Verify the cluster is in degraded state via API
	clusterFixture.VerifyFailedResourcesInResponse(t, KubernetesInternalAPIServerAddr)
	t.Log("✅ Verified cluster is in degraded state via API")

	// Now simulate recovery by fixing the webhook (restore working CRD)
	t.Log("🛠️ Fixing conversion webhook by updating CRD back to working state")
	_, err = fixture.Run("", "kubectl", "apply", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml")
	require.NoError(t, err)

	// Trigger cluster cache invalidation to force detection of recovery
	t.Log("🔄 Invalidating cluster cache to trigger recovery detection")
	err = fixture.DoHttpJsonRequest("POST", "/api/v1/clusters/"+clusterURL+"/invalidate-cache", &invalidateResult)
	require.NoError(t, err)

	// Follow up with application hard refresh to force cache sync and trigger recovery detection
	t.Log("🔄 Hard refreshing application to force re-examination of fixed GVKs")
	GivenWithSameState(t).
		When().
		Refresh(RefreshTypeHard)

	// Wait for cache invalidation, refresh, and recovery detection
	time.Sleep(5 * time.Second)

	// Verify the cluster recovers to healthy state via API
	// The failedResourceGVKs field should be cleared after recovery
	time.Sleep(3 * time.Second) // Give time for recovery to propagate
	clusterFixture.VerifyHealthyClusterState(t, KubernetesInternalAPIServerAddr)
	t.Log("✅ Verified cluster recovered to healthy state via API")

	// Final cleanup
	t.Log("🛠️ Final cleanup of test resources")
	_, _ = fixture.Run("", "kubectl", "delete", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/resources/example-resource-v2.yaml", "--ignore-not-found")
	_, _ = fixture.Run("", "kubectl", "delete", "-f", "/tmp/argo-e2e/testdata.git/conversion-webhook-test/crds/crd.yaml", "--ignore-not-found")
}
