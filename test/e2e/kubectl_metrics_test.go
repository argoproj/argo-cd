package e2e

import (
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func TestKubectlMetrics(t *testing.T) {
	// Sync an app so that there are metrics to scrape.
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, fixture.Name(), app.Name)
			assert.Equal(t, fixture.RepoURL(fixture.RepoURLTypeFile), app.Spec.GetSource().RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.GetSource().Path)
			assert.Equal(t, fixture.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(argo.EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := fixture.RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, fixture.Name())
		}).
		When().
		// ensure that create is idempotent
		CreateApp().
		Then().
		Given().
		Revision("master").
		When().
		// ensure that update replaces spec and merge labels and annotations
		And(func() {
			errors.NewHandler(t).FailOnErr(fixture.AppClientset.ArgoprojV1alpha1().Applications(fixture.TestNamespace()).Patch(t.Context(),
				ctx.GetName(), types.MergePatchType, []byte(`{"metadata": {"labels": { "test": "label" }, "annotations": { "test": "annotation" }}}`), metav1.PatchOptions{}))
		}).
		CreateApp("--upsert").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "label", app.Labels["test"])
			assert.Equal(t, "annotation", app.Annotations["test"])
			assert.Equal(t, "master", app.Spec.GetSource().TargetRevision)
		})

	// Make a request to the app controller metrics endpoint and ensure the response contains kubectl metrics.
	resp, err := http.Get("http://127.0.0.1:8082/metrics")
	require.NoError(t, err)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Contains(t, string(body), "argocd_kubectl_request_duration_seconds", "metrics should have contained argocd_kubectl_request_duration_seconds")
	assert.Contains(t, string(body), "argocd_kubectl_request_size_bytes", "metrics should have contained argocd_kubectl_request_size_bytes")
	assert.Contains(t, string(body), "argocd_kubectl_response_size_bytes", "metrics should have contained argocd_kubectl_response_size_bytes")
	assert.Contains(t, string(body), "argocd_kubectl_rate_limiter_duration_seconds", "metrics should have contained argocd_kubectl_rate_limiter_duration_seconds")
	assert.Contains(t, string(body), "argocd_kubectl_requests_total", "metrics should have contained argocd_kubectl_requests_total")

	/*
	  The following metrics are not being tested:
	  - argocd_kubectl_client_cert_rotation_age_seconds: The test doesn't use a client certificate, so this metric doesn't get populated.
	  - argocd_kubectl_dns_resolution_duration_seconds: It's unclear why this metric isn't populated. Possibly because DNS resolution is short-circuited by the test environment.
	  - argocd_kubectl_exec_plugin_call_total: The test doesn't use an exec plugin, so this metric doesn't get populated. TODO: add a test using an exec plugin to populate this metric.
	  - argocd_kubectl_request_retries_total: The test is unlikely to encounter a need to retry requests, so this metric is likely unpopulated.
	  - argocd_kubectl_transport_cache_entries: The transport cache is only used under certain conditions, which this test doesn't encounter.
	  - argocd_kubectl_transport_create_calls_total: The transport cache is only used under certain conditions, which this test doesn't encounter.
	*/

	// Repeat the test for port 8083, i.e. the API server.
	resp, err = http.Get("http://127.0.0.1:8083/metrics")
	require.NoError(t, err)
	defer func() {
		err = resp.Body.Close()
		require.NoError(t, err)
	}()
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	assert.Contains(t, string(body), "argocd_kubectl_request_duration_seconds", "metrics should have contained argocd_kubectl_request_duration_seconds")
	assert.Contains(t, string(body), "argocd_kubectl_request_size_bytes", "metrics should have contained argocd_kubectl_request_size_bytes")
	assert.Contains(t, string(body), "argocd_kubectl_response_size_bytes", "metrics should have contained argocd_kubectl_response_size_bytes")
	assert.Contains(t, string(body), "argocd_kubectl_rate_limiter_duration_seconds", "metrics should have contained argocd_kubectl_rate_limiter_duration_seconds")
	assert.Contains(t, string(body), "argocd_kubectl_requests_total", "metrics should have contained argocd_kubectl_requests_total")
}
