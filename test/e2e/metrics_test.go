package e2e

import (
	"bufio"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	sessionpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/session"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/argo"
	"github.com/argoproj/argo-cd/v3/util/errors"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
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
			assert.Equal(t, ctx.GetName(), app.Name)
			assert.Equal(t, fixture.RepoURL(fixture.RepoURLTypeFile), app.Spec.GetSource().RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.GetSource().Path)
			assert.Equal(t, ctx.DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(Event(argo.EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := fixture.RunCli("app", "list")
			require.NoError(t, err)
			assert.Contains(t, output, ctx.GetName())
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

	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:8082/metrics", http.NoBody)
	require.NoError(t, err)

	// Make a request to the app controller metrics endpoint and ensure the response contains kubectl metrics.
	resp, err := http.DefaultClient.Do(req)
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

	req, err = http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:8083/metrics", http.NoBody)
	require.NoError(t, err)

	// Repeat the test for port 8083, i.e. the API server.
	resp, err = http.DefaultClient.Do(req)
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
	assert.Contains(t, string(body), "grpc_server_handled_total", "metrics should have contained grpc_server_handled_total for all the reflected methods")
	assert.Contains(t, string(body), "grpc_server_msg_received_total", "metrics should have contained grpc_server_msg_received_total for all the reflected methods")
}

func TestActiveUsersMetric(t *testing.T) {
	fixture.EnsureCleanState(t)

	baseline := scrapeActiveUsersCount(t)

	require.NoError(t, fixture.SetAccounts(map[string][]string{
		"active-user-1": {"login"},
		"active-user-2": {"login"},
	}))

	for _, username := range []string{"active-user-1", "active-user-2"} {
		_, err := fixture.RunCli("account", "update-password", "--account", username, "--current-password", fixture.AdminPassword, "--new-password", fixture.DefaultTestUserPassword)
		require.NoError(t, err)
	}

	for _, username := range []string{"active-user-1", "active-user-2"} {
		require.NoError(t, fixture.LoginAs(username))
		closer, client := fixture.ArgoCDClientset.NewSessionClientOrDie()
		_, err := client.GetUserInfo(t.Context(), &sessionpkg.GetUserInfoRequest{})
		utilio.Close(closer)
		require.NoError(t, err)
	}

	count := scrapeActiveUsersCount(t)
	assert.Equal(t, baseline+2, count)
}

func scrapeActiveUsersCount(t *testing.T) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://127.0.0.1:8083/metrics", http.NoBody)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, resp.Body.Close())
	}()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return parseActiveUsersMetric(t, string(body))
}

func parseActiveUsersMetric(t *testing.T, body string) int {
	t.Helper()
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "argocd_server_active_users_24h ") {
			fields := strings.Fields(line)
			require.Len(t, fields, 2)
			val, err := strconv.ParseFloat(fields[1], 64)
			require.NoError(t, err)
			return int(val)
		}
	}
	require.NoError(t, sc.Err())
	return 0
}
