package e2e

import (
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	. "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/stretchr/testify/assert"

	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/project"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

// when a app gets stuck in sync, and we try to delete it, it won't delete, instead we must then terminate it
// and deletion will then just happen
func TestDeletingAppStuckInSync(t *testing.T) {
	Given(t).
		And(func() {
			errors.CheckError(SetResourceOverrides(map[string]ResourceOverride{
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
		Expect(SyncStatusIs(SyncStatusCode(SyncStatusCodeSynced))).
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

func TestDeletingAppWithImpersonation(t *testing.T) {
	projectCtx := project.Given(t)
	appCtx := Given(t)

	projectCtx.
		Name("impersonation-test").
		SourceNamespaces([]string{"*"}).
		SourceRepositories([]string{"*"}).
		Destination("*,*").
		DestinationServiceAccounts([]string{fmt.Sprintf("%s,%s,%s", KubernetesInternalAPIServerAddr, "*", "default-sa")}).
		When().
		Create().
		Then().
		Expect()

	appCtx.
		Project("impersonation-test").
		Path(guestbookPath).
		When().
		CreateApp("--label=foo=bar").
		WithImpersonationEnabled("default-sa", []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"*"},
			},
		}).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		DeleteBySelectorWithWait("foo=bar").
		Then().
		Expect(DoesNotExist())
}

func TestDeletingAppWithImpersonationMissingDeletePermission(t *testing.T) {
	projectName := "impersonation-missing-delete-permission"
	serviceAccountName := "default-sa"

	projectCtx := project.Given(t)
	appCtx := Given(t)

	projectCtx.
		Name(projectName).
		SourceNamespaces([]string{"*"}).
		SourceRepositories([]string{"*"}).
		Destination("*,*").
		DestinationServiceAccounts([]string{fmt.Sprintf("%s,%s,%s", KubernetesInternalAPIServerAddr, "*", serviceAccountName)}).
		When().
		Create().
		Then().
		Expect()

	appCtx.
		Project(projectName).
		Path(guestbookPath).
		When().
		CreateApp("--label=foo=bar").
		WithImpersonationEnabled(serviceAccountName, []rbacv1.PolicyRule{
			{
				APIGroups: []string{"*"},
				Resources: []string{"*"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch"},
			},
		}).
		Sync().
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		When().
		Delete(false).
		Then().
		Expect(DoesNotExist()).
		ExpectConsistently(DoesNotExist(), WaitDuration, TimeoutDuration).
		ExpectConsistently(Pod(func(p corev1.Pod) bool { return strings.HasPrefix(p.Name, "guestbook-ui") }), WaitDuration, TimeoutDuration)
}
