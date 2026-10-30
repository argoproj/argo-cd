package e2e

import (
	"fmt"
	"testing"

	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

func TestDeletingAppWithExistingAndQualifiedFinalizers(t *testing.T) {
	for _, tc := range []struct {
		name      string
		finalizer string
	}{
		{"qualified", "argocd.argoproj.io/resources-finalizer"},
		{"existing", "resources-finalizer.argocd.argoproj.io"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := Given(t)
			ctx.Path("config-map").
				When().
				CreateApp().
				Sync().
				Then().
				Expect(SyncStatusIs(SyncStatusCodeSynced)).
				When().
				PatchApp(fmt.Sprintf(`[{"op":"add","path":"/metadata/finalizers","value":[%q]}]`, tc.finalizer)).
				And(func() {
					require.NoError(t, AppClientset.ArgoprojV1alpha1().Applications(ctx.AppNamespace()).Delete(t.Context(), ctx.AppName(), metav1.DeleteOptions{}))
				}).
				Then().
				Expect(DoesNotExist())
			_, err := KubeClientset.CoreV1().ConfigMaps(ctx.DeploymentNamespace()).Get(t.Context(), "my-map", metav1.GetOptions{})
			assert.True(t, apierrors.IsNotFound(err), "ConfigMap remains after Application deletion: %v", err)
		})
	}
}

// when a app gets stuck in sync, and we try to delete it, it won't delete, instead we must then terminate it
// and deletion will then just happen
func TestDeletingAppStuckInSync(t *testing.T) {
	Given(t).
		And(func() {
			require.NoError(t, SetResourceOverrides(map[string]ResourceOverride{
				"ConfigMap": {
					HealthLua: `return { status = obj.annotations and obj.annotations['health'] or 'Progressing' }`,
				},
			}))
		}).
		Async(true).
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

func TestDeletingAppByLabel(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp("--label=foo=bar").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		IgnoreErrors().
		DeleteBySelector("foo=baz").
		Then().
		// delete is unsuccessful since no selector match
		AndCLIOutput(
			func(_ string, err error) {
				assert.ErrorContains(t, err, "no apps match selector foo=baz")
			},
		).
		When().
		DeleteBySelector("foo=bar").
		Then().
		// delete is successful
		Expect(DoesNotExist())
}

func TestDeletingAppByLabelWait(t *testing.T) {
	Given(t).
		Path(guestbookPath).
		When().
		CreateApp("--label=foo=bar").
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCode(SyncStatusCodeSynced))).
		When().
		DeleteBySelectorWithWait("foo=bar").
		Then().
		// delete is successful
		Expect(DoesNotExistNow())
}
