package e2e

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v2/util/argo"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

// when we have a config map generator, AND the ignore annotation, it is ignored in the app's sync status
func TestDeployment(t *testing.T) {
	Given(t).
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		PatchFile("deployment.yaml", `[
    {
        "op": "replace",
        "path": "/spec/template/spec/containers/0/image",
        "value": "nginx:1.17.4-alpine"
    }
]`).
		Sync()
}

func TestDeploymentWithAnnotationTrackingMode(t *testing.T) {
	ctx := Given(t)

	SetTrackingMethod(string(argo.TrackingMethodAnnotation))
	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			out, err := RunCli("app", "manifests", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, out, fmt.Sprintf(`annotations:
    argocd.argoproj.io/tracking-id: %s:apps/Deployment:%s/nginx-deployment
`, Name(), DeploymentNamespace()))
		})
}

func TestDeploymentWithLabelTrackingMode(t *testing.T) {
	ctx := Given(t)
	SetTrackingMethod(string(argo.TrackingMethodLabel))
	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			out, err := RunCli("app", "manifests", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, out, fmt.Sprintf(`labels:
    app: nginx
    app.kubernetes.io/instance: %s
`, Name()))
		})
}

func TestDeploymentWithoutTrackingMode(t *testing.T) {
	Given(t).
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			out, err := RunCli("app", "manifests", app.Name)
			assert.NoError(t, err)
			assert.Contains(t, out, fmt.Sprintf(`labels:
    app: nginx
    app.kubernetes.io/instance: %s
`, Name()))
		})
}
