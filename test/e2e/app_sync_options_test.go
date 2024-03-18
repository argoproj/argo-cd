package e2e

import (
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

// Given application is set with --sync-option CreateNamespace=true and --sync-option ServerSideApply=true
//
//		application --dest-namespace exists
//
//	Then, --dest-namespace is created with server side apply
//		  	application is synced and healthy with resource
//		  	application resources created with server side apply in the newly created namespace.
func TestNamespaceCreationWithSSA(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	namespace := "guestbook-ui-with-ssa"
	defer func() {
		if !t.Skipped() {
			_, err := Run("", "kubectl", "delete", "namespace", namespace)
			assert.NoError(t, err)
		}
	}()

	ctx := Given(t)
	ctx.
		SetAppNamespace(AppNamespace()).
		Timeout(30).
		Path("guestbook").
		When().
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{
				SyncOptions: SyncOptions{"CreateNamespace=true", "ServerSideApply=true"},
			}
		}).
		Then().
		Expect(NoNamespace(namespace)).
		When().
		AppSet("--dest-namespace", namespace).
		Sync().
		Then().
		Expect(Success("")).
		Expect(Namespace(namespace, func(app *Application, ns *v1.Namespace) {
			assert.NotContains(t, ns.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
		})).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", namespace, SyncStatusCodeSynced)).
		Expect(ResourceHealthWithNamespaceIs("Service", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Service", "guestbook-ui", namespace, SyncStatusCodeSynced))
}
