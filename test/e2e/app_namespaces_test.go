package e2e

import (
	"context"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	. "github.com/argoproj/argo-cd/v2/util/argo"
	. "github.com/argoproj/argo-cd/v2/util/errors"
)

func TestAppCreationInOtherNamespace(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetAppNamespace(AppNamespace()).
		When().
		CreateApp().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		And(func(app *Application) {
			assert.Equal(t, ctx.AppName(), app.Name)
			assert.Equal(t, AppNamespace(), app.Namespace)
			assert.Equal(t, RepoURL(RepoURLTypeFile), app.Spec.GetSource().RepoURL)
			assert.Equal(t, guestbookPath, app.Spec.GetSource().Path)
			assert.Equal(t, DeploymentNamespace(), app.Spec.Destination.Namespace)
			assert.Equal(t, KubernetesInternalAPIServerAddr, app.Spec.Destination.Server)
		}).
		Expect(NamespacedEvent(ctx.AppNamespace(), EventReasonResourceCreated, "create")).
		And(func(_ *Application) {
			// app should be listed
			output, err := RunCli("app", "list")
			assert.NoError(t, err)
			assert.Contains(t, output, ctx.AppName())
		}).
		When().
		// ensure that create is idempotent
		CreateApp().
		Then().
		Given().
		Revision("master").
		When().
		// ensure that update replaces spec and merge labels and annotations
		And(func() {
			FailOnErr(AppClientset.ArgoprojV1alpha1().Applications(AppNamespace()).Patch(context.Background(),
				ctx.AppName(), types.MergePatchType, []byte(`{"metadata": {"labels": { "test": "label" }, "annotations": { "test": "annotation" }}}`), metav1.PatchOptions{}))
		}).
		CreateApp("--upsert").
		Then().
		And(func(app *Application) {
			assert.Equal(t, "label", app.Labels["test"])
			assert.Equal(t, "annotation", app.Annotations["test"])
			assert.Equal(t, "master", app.Spec.GetSource().TargetRevision)
		})
}

func TestForbiddenNamespace(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path(guestbookPath).
		SetAppNamespace("forbidden").
		When().
		IgnoreErrors().
		CreateApp().
		Then().
		Expect(DoesNotExist())
}

func TestDeletingNamespacedAppStuckInSync(t *testing.T) {
	ctx := Given(t)
	ctx.And(func() {
		SetResourceOverrides(map[string]ResourceOverride{
			"ConfigMap": {
				HealthLua: `return { status = obj.annotations and obj.annotations['health'] or 'Progressing' }`,
			},
		})
	}).
		Async(true).
		SetAppNamespace(AppNamespace()).
		Path("hook-custom-health").
		When().
		CreateApp().
		Sync().
		Then().
		// stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync)).
		When().
		Delete(true).
		Then().
		// delete is ignored, still stuck in running state
		Expect(OperationPhaseIs(OperationRunning)).
		When().
		TerminateOp().
		Then().
		// delete is successful
		Expect(DoesNotExist())
}

// Given application is set with --sync-option CreateNamespace=true and --sync-option ServerSideApply=true
//
//		application --dest-namespace exists
//
//	Then, --dest-namespace is created with server side apply
//		  	application is synced and healthy with resource
//		  	application resources created with server side apply in the newly created namespace.
func TestNamespacedNamespaceAutoCreationWithServerSideApply(t *testing.T) {
	SkipOnEnv(t, "OPENSHIFT")
	namespace := "guestbook-ui-with-server-side-apply"
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
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Deployment", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Deployment", "guestbook-ui", namespace, SyncStatusCodeSynced)).
		Expect(OperationPhaseIs(OperationSucceeded)).Expect(ResourceHealthWithNamespaceIs("Service", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceHealthWithNamespaceIs("Service", "guestbook-ui", namespace, health.HealthStatusHealthy)).
		Expect(ResourceSyncStatusWithNamespaceIs("Service", "guestbook-ui", namespace, SyncStatusCodeSynced))
}
