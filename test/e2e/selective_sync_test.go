package e2e

import (
	"fmt"
	"strings"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/util/rand"
)

// when you selectively sync, only selected resources should be synced, but the app will be out of sync
func TestSelectiveSync(t *testing.T) {
	Given(t).
		Path("guestbook").
		SelectedResource(":Service:guestbook-ui").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("Service", "guestbook-ui", "", health.HealthStatusHealthy)).
		Expect(ResourceHealthIs("Deployment", "guestbook-ui", "", health.HealthStatusMissing))
}

// when running selective sync, hooks do not run
// hooks don't run even if all resources are selected
func TestSelectiveSyncDoesNotRunHooks(t *testing.T) {
	Given(t).
		Path("hook").
		SelectedResource(":Pod:pod").
		When().
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(ResourceHealthIs("Pod", "pod", "", health.HealthStatusHealthy)).
		Expect(ResourceResultNumbering(1))
}

func TestSelectiveSyncWithoutNamespace(t *testing.T) {
	selectedResourceNamespace := getNewNamespace(t)
	Given(t).
		Prune(true).
		Path("guestbook-with-namespace").
		And(func() {
			fixture.CreateNamespace(selectedResourceNamespace)
		}).
		SelectedResource("apps:Deployment:guestbook-ui").
		When().
		PatchFile("guestbook-ui-deployment-ns.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/metadata/namespace", "value": "%s"}]`, selectedResourceNamespace)).
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("Deployment", "guestbook-ui", "", health.HealthStatusHealthy))
}

//In selectedResource to sync, namespace is provided
func TestSelectiveSyncWithNamespace(t *testing.T) {
	selectedResourceNamespace := getNewNamespace(t)
	Given(t).
		Prune(true).
		Path("guestbook-with-namespace").
		And(func() {
			fixture.CreateNamespace(selectedResourceNamespace)
		}).
		SelectedResource(fmt.Sprintf("apps:Deployment:%s/guestbook-ui", selectedResourceNamespace)).
		When().
		PatchFile("guestbook-ui-deployment-ns.yaml", fmt.Sprintf(`[{"op": "replace", "path": "/metadata/namespace", "value": "%s"}]`, selectedResourceNamespace)).
		Create().
		Sync().
		Then().
		Expect(Success("")).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		Expect(ResourceHealthIs("Deployment", "guestbook-ui", selectedResourceNamespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthIs("Deployment", "guestbook-ui", fixture.DeploymentNamespace(), health.HealthStatusMissing))
}

func getNewNamespace(t *testing.T) string {
	postFix := "-" + strings.ToLower(rand.RandString(5))
	name := fixture.DnsFriendly(t.Name(), "")
	return fixture.DnsFriendly(fmt.Sprintf("argocd-e2e-%s", name), postFix)
}
