package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
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
