package e2e

import (
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// liveSecretWithCredentials returns the live Secret with its labels and
// annotations intact (so Argo CD tracking is untouched) but with data holding
// values that were "generated in the cluster", i.e. not the rendered ones.
func liveSecretWithCredentials(t *testing.T, namespace string) string {
	t.Helper()
	live, err := fixture.KubeClientset.CoreV1().Secrets(namespace).Get(t.Context(), "generated-credentials", metav1.GetOptions{})
	require.NoError(t, err)

	seed := live.DeepCopy()
	seed.APIVersion = "v1"
	seed.Kind = "Secret"
	seed.ResourceVersion = ""
	seed.UID = ""
	seed.ManagedFields = nil
	delete(seed.Annotations, corev1.LastAppliedConfigAnnotation)
	seed.StringData = nil
	seed.Data = map[string][]byte{
		"ACCESS_KEY": []byte("live-access"),
		"SECRET_KEY": []byte("live-secret"),
		"ENDPOINT":   []byte("live-endpoint"),
	}

	manifest, err := json.Marshal(seed)
	require.NoError(t, err)
	return string(manifest)
}

// respectIgnoreDifferencesKeepsSecretKeys syncs the app once, then rewrites the
// live Secret the way a sync from a controller before #27136 left it: the
// ignored keys recorded as managed by Argo CD, either in the last-applied
// annotation (client-side apply) or under the argocd-controller field manager
// (server-side apply). A second sync must keep the live values of the ignored
// keys, apply the rendered value of the non-ignored key, and stay Synced.
func respectIgnoreDifferencesKeepsSecretKeys(t *testing.T, serverSideApply bool) {
	t.Helper()
	syncOptions := `["RespectIgnoreDifferences=true"]`
	seedArgs := []string{"apply", "--validate=false", "-f", "-"}
	if serverSideApply {
		syncOptions = `["RespectIgnoreDifferences=true", "ServerSideApply=true"]`
		seedArgs = []string{"apply", "--server-side=true", "--field-manager=" + common.ArgoCDSSAManager, "--force-conflicts", "--validate=false", "-f", "-"}
	}

	ctx := Given(t)
	ctx.Path("secret-stringdata").
		When().
		CreateApp().
		PatchApp(`[{
			"op": "add",
			"path": "/spec/ignoreDifferences",
			"value": [{
				"kind": "Secret",
				"name": "generated-credentials",
				"jqPathExpressions": [".data[\"ACCESS_KEY\"], .data[\"SECRET_KEY\"], .stringData[\"ACCESS_KEY\"], .stringData[\"SECRET_KEY\"]"]
			}]
		}, {
			"op": "add",
			"path": "/spec/syncPolicy",
			"value": { "syncOptions": ` + syncOptions + ` }
		}]`).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		And(func() {
			seed := liveSecretWithCredentials(t, ctx.DeploymentNamespace())
			args := append([]string{"-n", ctx.DeploymentNamespace()}, seedArgs...)
			_, err := fixture.RunWithStdin(seed, "", "kubectl", args...)
			require.NoError(t, err)
		}).
		Refresh(RefreshTypeNormal).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(_ *Application) {
			secret, err := fixture.KubeClientset.CoreV1().Secrets(ctx.DeploymentNamespace()).Get(t.Context(), "generated-credentials", metav1.GetOptions{})
			require.NoError(t, err)
			assert.Equal(t, "live-access", string(secret.Data["ACCESS_KEY"]), "ignored key must keep its live value")
			assert.Equal(t, "live-secret", string(secret.Data["SECRET_KEY"]), "ignored key must keep its live value")
			assert.Equal(t, "rendered-endpoint", string(secret.Data["ENDPOINT"]), "non-ignored key must come from git")
		})
}

func TestRespectIgnoreDifferencesKeepsSecretKeysClientSideApply(t *testing.T) {
	respectIgnoreDifferencesKeepsSecretKeys(t, false)
}

func TestRespectIgnoreDifferencesKeepsSecretKeysServerSideApply(t *testing.T) {
	respectIgnoreDifferencesKeepsSecretKeys(t, true)
}
