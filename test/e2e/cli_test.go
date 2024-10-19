package e2e

import (
	"context"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

func TestCliAppCommand(t *testing.T) {
	Given(t).
		Path("hook").
		When().
		CreateApp().
		And(func() {
			output, err := RunCli("app", "sync", Name(), "--timeout", "90")
			require.NoError(t, err)
			vars := map[string]interface{}{"Name": Name(), "Namespace": DeploymentNamespace()}
			assert.Contains(t, NormalizeOutput(output), Tmpl(`Pod {{.Namespace}} pod Synced Progressing pod/pod created`, vars))
			assert.Contains(t, NormalizeOutput(output), Tmpl(`Pod {{.Namespace}} hook Succeeded Sync pod/hook created`, vars))
		}).
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		And(func(_ *Application) {
			output, err := RunCli("app", "list")
			require.NoError(t, err)
			expected := Tmpl(
				`{{.Name}} https://kubernetes.default.svc {{.Namespace}} default Synced Healthy Manual <none>`,
				map[string]interface{}{"Name": Name(), "Namespace": DeploymentNamespace()})
			assert.Contains(t, NormalizeOutput(output), expected)
		})
}

func TestResourceOverrideActionWithParameters(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		And(func(app *Application) {
			// Setup: Create a ConfigMap with a custom resource override action
			SetResourceOverrides(map[string]ResourceOverride{
				"apps/Deployment": {
					Actions: `
testAction:
  lua:
    action: |
      function run(obj, args)
        if args.label ~= nil then
          obj.metadata.labels["test-label"] = args.label
        end
        return obj
      end
`,
				},
			})

			output, err := RunCli("admin", "settings", "resource-overrides", "run-action",
				"guestbook-ui", "testAction",
				"--param", "label=test-value")

			// Verify: Check the output and error
			assert.NoError(t, err)
			assert.Contains(t, output, `+ "test-label": "test-value"`)

			deployment, err := KubeClientset.AppsV1().Deployments(DeploymentNamespace()).Get(context.Background(), "guestbook-ui", metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, "test-value", deployment.ObjectMeta.Labels["test-label"])
		})
}
