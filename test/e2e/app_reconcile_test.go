package e2e

import (
	"fmt"
	"testing"
	"time"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLastTransitionTimeUnchangedError(t *testing.T) {
	// Ensure that, if the health status hasn't changed, the lastTransitionTime is not updated.

	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		When().
		And(func() {
			// Manually create an application with an outdated reconciledAt field
			manifest := fmt.Sprintf(`
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: guestbook
    targetRevision: HEAD
  destination:
    server: https://non-existent-cluster
    namespace: default
status:
  reconciledAt: "2023-01-01T00:00:00Z"
  health:
    status: Unknown
    lastTransitionTime: "2023-01-01T00:00:00Z"
`, ctx.AppName())
			_, err := fixture.RunWithStdin(manifest, "", "kubectl", "apply", "-n", fixture.ArgoCDNamespace, "-f", "-")
			require.NoError(t, err)
		}).
		Refresh(RefreshTypeNormal).
		Then().
		And(func(app *Application) {
			// Verify the health status is still Unknown
			assert.Equal(t, health.HealthStatusUnknown, app.Status.Health.Status)

			// Verify the lastTransitionTime has not been updated
			assert.Equal(t, "2023-01-01T00:00:00Z", app.Status.Health.LastTransitionTime.UTC().Format(time.RFC3339))
		})
}
