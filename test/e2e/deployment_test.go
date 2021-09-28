package e2e

import (
	"fmt"
	"testing"

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
		Create().
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
	SetTrackingMethod(string(argo.TrackingMethodAnnotation))

	Given(t).
		Path("deployment").
		When().
		Create().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			_, _ = RunCli("app", "sync", app.Name, "--label", fmt.Sprintf("app.kubernetes.io/instance=%s", app.Name))
		})
}
