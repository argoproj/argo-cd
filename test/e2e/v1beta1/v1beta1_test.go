package v1beta1

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1beta1"
	appclientset "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v3/util/rand"
)

// v1beta1 CEL Validation Tests
// These tests verify that CEL validations on v1beta1.Application work correctly.
// v1beta1 has stricter validation rules than v1alpha1 (or rather v1alpha1 has no validation).
//
// These tests only require a Kubernetes cluster with the ArgoCD CRD installed.
// They do NOT require the full ArgoCD E2E environment (no API server needed).
// They use DryRun so no actual resources are created.

// getV1beta1TestClientset creates a clientset using the same kubeconfig approach as the e2e fixture.
// This is intentionally separate from the main fixture to avoid triggering the fixture's init()
// which tries to connect to the ArgoCD API server.
func getV1beta1TestClientset(t *testing.T) appclientset.Interface {
	t.Helper()

	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	clientConfig := clientcmd.NewInteractiveDeferredLoadingClientConfig(loadingRules, &clientcmd.ConfigOverrides{}, os.Stdin)

	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		t.Fatalf("Failed to get kubeconfig: %v", err)
	}

	clientset, err := appclientset.NewForConfig(restConfig)
	require.NoError(t, err, "Failed to create clientset")

	return clientset
}

// getV1beta1TestNamespace returns the namespace using the same env var as the e2e fixture
func getV1beta1TestNamespace() string {
	if ns := os.Getenv("ARGOCD_E2E_NAMESPACE"); ns != "" {
		return ns
	}
	return "argocd-e2e"
}

// randomString generates a lowercase random string for test names (K8s naming compliant)
func randomString(n int) string {
	s, _ := rand.StringFromCharset(n, "abcdefghijklmnopqrstuvwxyz0123456789")
	return s
}

func newV1beta1App(name, namespace string) *v1beta1.Application {
	return &v1beta1.Application{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "argoproj.io/v1beta1",
			Kind:       "Application",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1beta1.ApplicationSpec{
			Project: "default",
			Sources: v1beta1.ApplicationSources{
				{
					RepoURL:        "https://github.com/argoproj/argocd-example-apps",
					Path:           "guestbook",
					TargetRevision: "HEAD",
				},
			},
			Destination: v1alpha1.ApplicationDestination{
				Server:    v1alpha1.KubernetesInternalAPIServerAddr,
				Namespace: "default",
			},
		},
	}
}

// TestV1beta1SourcesRequired verifies that v1beta1 requires sources to be set
func TestV1beta1SourcesRequired(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-sources-required-"+randomString(5), namespace)
	app.Spec.Sources = nil

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sources is required")
}

// TestV1beta1SourcesEmptyArray verifies that v1beta1 rejects empty sources array
func TestV1beta1SourcesEmptyArray(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-sources-empty-"+randomString(5), namespace)
	app.Spec.Sources = v1beta1.ApplicationSources{}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sources is required")
}

// TestV1beta1SourcesMissingRepoURL verifies that all sources must have repoURL
func TestV1beta1SourcesMissingRepoURL(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-sources-missing-repourl-"+randomString(5), namespace)
	app.Spec.Sources = v1beta1.ApplicationSources{
		{
			Path:           "guestbook",
			TargetRevision: "HEAD",
		},
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all sources must have a repoURL")
}

// TestV1beta1SourcesChartAndPathConflict verifies that sources can't have both chart and path
func TestV1beta1SourcesChartAndPathConflict(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-sources-chart-path-conflict-"+randomString(5), namespace)
	app.Spec.Sources = v1beta1.ApplicationSources{
		{
			RepoURL:        "https://charts.helm.sh/stable",
			Chart:          "nginx",
			Path:           "some-path",
			TargetRevision: "1.0.0",
		},
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "sources cannot have both chart and path defined")
}

// TestV1beta1RefSourceWithPath verifies that ref sources can't have path
func TestV1beta1RefSourceWithPath(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-ref-source-with-path-"+randomString(5), namespace)
	app.Spec.Sources = v1beta1.ApplicationSources{
		{
			RepoURL:        "https://github.com/argoproj/argocd-example-apps",
			Path:           "guestbook",
			TargetRevision: "HEAD",
		},
		{
			RepoURL:        "https://github.com/argoproj/argocd-example-apps",
			Ref:            "values",
			Path:           "should-not-be-here",
			TargetRevision: "HEAD",
		},
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ref sources cannot have path or chart defined")
}

// TestV1beta1ProjectRequired verifies that project must be set
func TestV1beta1ProjectRequired(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-project-required-"+randomString(5), namespace)
	app.Spec.Project = ""

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "project is required")
}

// TestV1beta1DestinationServerOrName verifies that destination must have server or name
func TestV1beta1DestinationServerOrName(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-dest-required-"+randomString(5), namespace)
	app.Spec.Destination = v1alpha1.ApplicationDestination{
		Namespace: "default",
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "destination must have either server or name set")
}

// TestV1beta1DestinationBothServerAndName verifies that destination can't have both server and name
func TestV1beta1DestinationBothServerAndName(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-dest-both-"+randomString(5), namespace)
	app.Spec.Destination = v1alpha1.ApplicationDestination{
		Server:    v1alpha1.KubernetesInternalAPIServerAddr,
		Name:      "in-cluster",
		Namespace: "default",
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "can't have both name and server defined")
}

// TestV1beta1BackoffFactorLessThanOne verifies that backoff factor must be >= 1
func TestV1beta1BackoffFactorLessThanOne(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-backoff-factor-"+randomString(5), namespace)
	factorPtr := int64(0)
	app.Spec.SyncPolicy = &v1beta1.SyncPolicy{
		Retry: &v1alpha1.RetryStrategy{
			Limit: 3,
			Backoff: &v1alpha1.Backoff{
				Factor: &factorPtr,
			},
		},
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "retry backoff factor must be >= 1")
}

// TestV1beta1RevisionHistoryLimitNegative verifies that revisionHistoryLimit must be >= 0
func TestV1beta1RevisionHistoryLimitNegative(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-rev-history-negative-"+randomString(5), namespace)
	negativeLimit := int64(-5)
	app.Spec.RevisionHistoryLimit = &negativeLimit

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "revisionHistoryLimit must be >= 0")
}

// TestV1beta1IgnoreDifferencesMissingKind verifies that ignoreDifferences must have kind
func TestV1beta1IgnoreDifferencesMissingKind(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-ignore-diff-no-kind-"+randomString(5), namespace)
	app.Spec.IgnoreDifferences = v1beta1.IgnoreDifferences{
		{
			Group:        "apps",
			JSONPointers: []string{"/spec/replicas"},
		},
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ignoreDifferences entries must have a kind")
}

// TestV1beta1ValidApplication verifies that a valid v1beta1 Application is accepted
func TestV1beta1ValidApplication(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-valid-app-"+randomString(5), namespace)

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	assert.NoError(t, err)
}

// TestV1beta1ValidApplicationWithMultipleSources verifies that multiple sources are accepted
func TestV1beta1ValidApplicationWithMultipleSources(t *testing.T) {
	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	app := newV1beta1App("test-valid-multi-source-"+randomString(5), namespace)
	app.Spec.Sources = v1beta1.ApplicationSources{
		{
			RepoURL:        "https://github.com/argoproj/argocd-example-apps",
			Path:           "guestbook",
			TargetRevision: "HEAD",
		},
		{
			RepoURL:        "https://github.com/argoproj/argocd-example-apps",
			Ref:            "values",
			TargetRevision: "HEAD",
		},
	}

	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	assert.NoError(t, err)
}

// TestV1beta1ObservedGeneration verifies that observedGeneration is set in v1beta1 status
// after the controller reconciles the application.
// This test requires the full ArgoCD E2E environment to be running.
func TestV1beta1ObservedGeneration(t *testing.T) {
	/*
		if os.Getenv("ARGOCD_E2E_TEST") != "true" {
			t.Skip("Skipping: requires full ArgoCD E2E environment (set ARGOCD_E2E_TEST=true)")
		}

	*/

	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	appName := "test-observed-gen-" + randomString(5)
	app := newV1beta1App(appName, namespace)

	// Create the application (not DryRun - we need the controller to reconcile it)
	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Cleanup
	defer func() {
		_ = clientset.ArgoprojV1beta1().Applications(namespace).Delete(
			context.Background(),
			appName,
			metav1.DeleteOptions{},
		)
	}()

	// Wait for the controller to reconcile (status will be updated)
	// Poll until status.observedGeneration is set or timeout
	var reconciledApp *v1beta1.Application
	require.Eventually(t, func() bool {
		reconciledApp, err = clientset.ArgoprojV1beta1().Applications(namespace).Get(
			context.Background(),
			appName,
			metav1.GetOptions{},
		)
		if err != nil {
			return false
		}
		// Check if observedGeneration is set (non-zero indicates reconciliation)
		return reconciledApp.Status.ObservedGeneration > 0
	}, 60*time.Second, 1*time.Second, "Timed out waiting for observedGeneration to be set")

	// Verify observedGeneration matches the current generation
	// Note: We use reconciledApp.Generation, not createdApp.Generation, because
	// normalization/defaulting webhooks may have bumped the generation after creation
	assert.Equal(t, reconciledApp.Generation, reconciledApp.Status.ObservedGeneration,
		"observedGeneration should match metadata.generation after reconciliation")

	// Also verify via v1alpha1 API - observedGeneration should NOT be present
	// (v1alpha1.ApplicationStatus doesn't have the field)
	v1alpha1App, err := clientset.ArgoprojV1alpha1().Applications(namespace).Get(
		context.Background(),
		appName,
		metav1.GetOptions{},
	)
	require.NoError(t, err)

	// v1alpha1.ApplicationStatus doesn't have ObservedGeneration field,
	// so we can't check it directly. We just verify the status is populated.
	assert.NotEmpty(t, v1alpha1App.Status.Sync.Status, "v1alpha1 status should be populated")
}

// TestV1beta1ObservedGenerationUpdatesOnSpecChange verifies that observedGeneration
// is updated when the spec changes.
// This test requires the full ArgoCD E2E environment to be running.
func TestV1beta1ObservedGenerationUpdatesOnSpecChange(t *testing.T) {
	/*
		if os.Getenv("ARGOCD_E2E_TEST") != "true" {
			t.Skip("Skipping: requires full ArgoCD E2E environment (set ARGOCD_E2E_TEST=true)")
		}

	*/

	clientset := getV1beta1TestClientset(t)
	namespace := getV1beta1TestNamespace()

	appName := "test-observed-gen-update-" + randomString(5)
	app := newV1beta1App(appName, namespace)

	// Create the application
	_, err := clientset.ArgoprojV1beta1().Applications(namespace).Create(
		context.Background(),
		app,
		metav1.CreateOptions{},
	)
	require.NoError(t, err)

	// Cleanup
	defer func() {
		_ = clientset.ArgoprojV1beta1().Applications(namespace).Delete(
			context.Background(),
			appName,
			metav1.DeleteOptions{},
		)
	}()

	// Wait for initial reconciliation - observedGeneration should match current generation
	// Note: generation may increase after creation due to normalization/defaulting
	var reconciledApp *v1beta1.Application
	require.Eventually(t, func() bool {
		reconciledApp, err = clientset.ArgoprojV1beta1().Applications(namespace).Get(
			context.Background(),
			appName,
			metav1.GetOptions{},
		)
		if err != nil {
			return false
		}
		// Wait until observedGeneration catches up to generation
		return reconciledApp.Status.ObservedGeneration == reconciledApp.Generation
	}, 60*time.Second, 1*time.Second, "Timed out waiting for initial reconciliation")

	initialGeneration := reconciledApp.Generation

	// Update the spec to trigger a new generation
	reconciledApp.Spec.Sources[0].TargetRevision = "main"
	updatedApp, err := clientset.ArgoprojV1beta1().Applications(namespace).Update(
		context.Background(),
		reconciledApp,
		metav1.UpdateOptions{},
	)
	require.NoError(t, err)

	// Note: We don't rely on updatedApp.Generation because normalization webhooks
	// may bump it again. Instead, we verify that observedGeneration catches up
	// to whatever the current generation is.
	_ = updatedApp // Ensure spec change was accepted

	// Wait for observedGeneration to catch up to the current generation
	// The generation should be higher than initial after our spec update
	require.Eventually(t, func() bool {
		reconciledApp, err = clientset.ArgoprojV1beta1().Applications(namespace).Get(
			context.Background(),
			appName,
			metav1.GetOptions{},
		)
		if err != nil {
			return false
		}
		// observedGeneration should match current generation AND be higher than initial
		return reconciledApp.Status.ObservedGeneration == reconciledApp.Generation &&
			reconciledApp.Generation > initialGeneration
	}, 60*time.Second, 1*time.Second, "Timed out waiting for observedGeneration to update")

	assert.Equal(t, reconciledApp.Generation, reconciledApp.Status.ObservedGeneration,
		"observedGeneration should match current generation after spec change")
	assert.Greater(t, reconciledApp.Generation, initialGeneration,
		"generation should have increased after spec update")
}
