package e2e

import (
	"fmt"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/project"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
)

const (
	WaitDuration    = time.Second
	TimeoutDuration = time.Second * 3
)

func TestSyncWithFeatureDisabled(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		WithImpersonationDisabled().
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
		}).
		Then().
		// With the impersonation feature disabled, Application sync should continue to use
		// the control plane service account for the sync operation and the sync should succeed.
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeSynced), WaitDuration, TimeoutDuration).
		Expect(OperationMessageContains("successfully synced"))
}

func TestSyncWithNoDestinationServiceAccountsInProject(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
		}).
		WithImpersonationEnabled("", nil).
		Then().
		// With the impersonation feature enabled, Application sync must fail
		// when there are no destination service accounts configured in AppProject
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeOutOfSync), WaitDuration, TimeoutDuration).
		Expect(OperationMessageContains("failed to find a matching service account to impersonate"))
}

func TestSyncWithImpersonateWithSyncServiceAccount(t *testing.T) {
	projectName := "sync-test-project"
	serviceAccountName := "test-account"

	projectCtx := project.Given(t)
	appCtx := Given(t)

	projectCtx.
		Name(projectName).
		SourceNamespaces([]string{"*"}).
		SourceRepositories([]string{"*"}).
		Destination("*,*").
		DestinationServiceAccounts(
			[]string{
				fmt.Sprintf("%s,%s,%s", "*", fixture.DeploymentNamespace(), serviceAccountName),
				fmt.Sprintf("%s,%s,%s", v1alpha1.KubernetesInternalAPIServerAddr, fixture.DeploymentNamespace(), "missing-serviceAccount"),
			}).
		When().
		Create().
		Then().
		Expect()

	appCtx.
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		WithImpersonationEnabled(serviceAccountName, []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps", ""},
				Resources: []string{"deployments"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"*"},
			},
		}).
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
			app.Spec.Project = projectName
		}).
		Then().
		// With the impersonation feature enabled, Application sync should succeed
		// as there is a valid match found in the available destination service accounts configured in AppProject
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeSynced), WaitDuration, TimeoutDuration).
		Expect(OperationMessageContains("successfully synced"))
}

func TestSyncWithMissingServiceAccount(t *testing.T) {
	projectName := "false-test-project"
	serviceAccountName := "test-account"

	projectCtx := project.Given(t)
	appCtx := Given(t)

	projectCtx.
		Name(projectName).
		SourceNamespaces([]string{"*"}).
		SourceRepositories([]string{"*"}).
		Destination("*,*").
		DestinationServiceAccounts(
			[]string{
				fmt.Sprintf("%s,%s,%s", v1alpha1.KubernetesInternalAPIServerAddr, fixture.DeploymentNamespace(), "missing-serviceAccount"),
				fmt.Sprintf("%s,%s,%s", "*", fixture.DeploymentNamespace(), serviceAccountName),
			}).
		When().
		Create().
		Then().
		Expect()

	appCtx.
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		WithImpersonationEnabled(serviceAccountName, []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps", ""},
				Resources: []string{"deployments"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"*"},
			},
		}).
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
			app.Spec.Project = projectName
		}).
		Then().
		// With the impersonation feature enabled, Application sync must fail
		// when there is a valid match found in the available destination service accounts configured in AppProject,
		// but the matching service account is missing.
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeOutOfSync), WaitDuration, TimeoutDuration).
		Expect(OperationMessageContains("one or more objects failed to apply"))
}

func TestSyncWithValidSAButDisallowedDestination(t *testing.T) {
	projectName := "negation-test-project"
	serviceAccountName := "test-account"

	projectCtx := project.Given(t)
	appCtx := Given(t)

	projectCtx.
		Name(projectName).
		SourceNamespaces([]string{"*"}).
		SourceRepositories([]string{"*"}).
		Destination("*,*").
		DestinationServiceAccounts(
			[]string{
				fmt.Sprintf("%s,%s,%s", "*", fixture.DeploymentNamespace(), serviceAccountName),
			}).
		When().
		Create().
		Then().
		Expect()

	appCtx.
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		WithImpersonationEnabled(serviceAccountName, []rbacv1.PolicyRule{
			{
				APIGroups: []string{"apps", ""},
				Resources: []string{"deployments"},
				Verbs:     []string{"*"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{"services"},
				Verbs:     []string{"*"},
			},
		}).
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
			app.Spec.Project = projectName
		}).
		Then().
		Expect(SyncStatusIs(v1alpha1.SyncStatusCodeSynced)).
		When().
		And(func() {
			// Patch destination to disallow target destination namespace
			patch := []byte(fmt.Sprintf(`{"spec": {"destinations": [{"namespace": %q}]}}`, "!"+fixture.DeploymentNamespace()))

			_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.TestNamespace()).Patch(t.Context(), projectName, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)
		}).
		Refresh(v1alpha1.RefreshTypeNormal).
		Then().
		// With the impersonation feature enabled, Application sync must fail
		// as there is a valid match found in the available destination service accounts configured in AppProject
		// but the destination namespace is now disallowed.
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeUnknown), WaitDuration, TimeoutDuration)
}
