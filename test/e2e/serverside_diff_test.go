package e2e

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	applicationpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/application"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	utilio "github.com/argoproj/argo-cd/v3/util/io"
)

// TestServerSideDiffMasksSecretData is a regression test for a CVE where the
// ServerSideDiff endpoint returned plaintext Kubernetes Secret values from etcd.
func TestServerSideDiffMasksSecretData(t *testing.T) {
	closer, client, err := ArgoCDClientset.NewApplicationClient()
	require.NoError(t, err)
	defer utilio.Close(closer)

	Given(t).
		Path("secrets").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			ns := app.Spec.Destination.Namespace

			// Establish a second SSA field manager owning the secret's data field.
			// Without a second manager, argocd-controller is the sole owner and the
			// SSA dry-run garbage-collects the data fields (since the target manifest
			// omits them). A second manager retains ownership, so the real values
			// survive in the dry-run response — the exact condition required for the
			// CVE to be exploitable.
			secretPatch := fmt.Sprintf(
				`{"apiVersion":"v1","kind":"Secret","metadata":{"name":"test-secret","namespace":%q},"data":{"username":%q}}`,
				ns,
				base64.StdEncoding.EncodeToString([]byte("test-username")),
			)
			_, err := KubeClientset.CoreV1().Secrets(ns).Patch(
				t.Context(),
				"test-secret",
				types.ApplyPatchType,
				[]byte(secretPatch),
				metav1.PatchOptions{FieldManager: "test-manager"},
			)
			require.NoError(t, err)

			// Annotate the app with IncludeMutationWebhook=true — the condition that
			// bypasses removeWebhookMutation() and exposed real etcd values in the response.
			_, err = RunCli("app", "patch", app.Name,
				"--patch", `{"metadata":{"annotations":{"argocd.argoproj.io/compare-options":"IncludeMutationWebhook=true"}}}`,
				"--type", "merge",
			)
			require.NoError(t, err)

			// Fetch the masked live state as ArgoCD sees it.
			// This is the same data an attacker would read from managed-resources
			// before crafting the ServerSideDiff request.
			resources, err := client.ManagedResources(t.Context(), &applicationpkg.ResourcesQuery{
				ApplicationName: &app.Name,
			})
			require.NoError(t, err)

			var secretLiveState string
			for _, r := range resources.Items {
				if r.Kind == "Secret" && r.Name == "test-secret" {
					secretLiveState = r.LiveState
					break
				}
			}
			require.NotEmpty(t, secretLiveState, "test-secret not found in managed resources")

			// Build a minimal target manifest with no data field — exactly what the
			// exploit sends to force the SSA dry-run to return data owned by the
			// second field manager (i.e., real etcd values).
			target, err := json.Marshal(map[string]any{
				"apiVersion": "v1",
				"kind":       "Secret",
				"metadata":   map[string]any{"name": "test-secret", "namespace": ns},
				"type":       "Opaque",
			})
			require.NoError(t, err)

			resp, err := client.ServerSideDiff(t.Context(), &applicationpkg.ApplicationServerSideDiffQuery{
				AppName: &app.Name,
				Project: &app.Spec.Project,
				LiveResources: []*ResourceDiff{{
					Kind:      "Secret",
					Namespace: ns,
					Name:      "test-secret",
					LiveState: secretLiveState,
					Modified:  true,
				}},
				TargetManifests: []string{string(target)},
			})
			require.NoError(t, err)
			require.NotEmpty(t, resp.Items, "expected at least one diff item in response")

			for _, item := range resp.Items {
				if item.Kind != "Secret" {
					continue
				}
				assertServerSideDiffSecretMasked(t, item.TargetState, "targetState")
				assertServerSideDiffSecretMasked(t, item.LiveState, "liveState")
			}
		})
}

// assertServerSideDiffSecretMasked verifies that every value in the data field of the
// given secret JSON manifest consists only of '+' characters (ArgoCD's masking format).
func assertServerSideDiffSecretMasked(t *testing.T, manifest, field string) {
	t.Helper()
	if manifest == "" || manifest == "null" {
		return
	}
	obj := &unstructured.Unstructured{}
	require.NoError(t, obj.UnmarshalJSON([]byte(manifest)), "failed to parse %s as JSON", field)

	data, hasData, err := unstructured.NestedStringMap(obj.Object, "data")
	require.NoError(t, err)
	if !hasData || len(data) == 0 {
		return
	}
	for k, v := range data {
		assert.Regexp(t, `^\++$`, v,
			"%s: secret key %q must be masked with '+' characters, got %q", field, k, v)
	}
}
