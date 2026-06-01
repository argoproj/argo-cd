package e2e

import (
	"testing"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

// ensure that cluster scoped objects, like a cluster role, as a hook, can be successfully deployed
func TestClusterRoleBindingHook(t *testing.T) {
	Given(t).
		Path("cluster-role-hook").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}
