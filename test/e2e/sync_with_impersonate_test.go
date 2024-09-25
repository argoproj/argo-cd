package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

const (
	WaitDuration    = time.Second
	TimeoutDuration = time.Second * 3
)

func TestSyncWithFeatureDisabled(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "false").
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
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		CreateFromFile(func(app *v1alpha1.Application) {
			app.Spec.SyncPolicy = &v1alpha1.SyncPolicy{Automated: &v1alpha1.SyncPolicyAutomated{}}
		}).
		Then().
		// With the impersonation feature enabled, Application sync must fail
		// when there are no destination service accounts configured in AppProject
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeOutOfSync), WaitDuration, TimeoutDuration).
		Expect(OperationMessageContains("failed to find a matching service account to impersonate"))
}

func TestSyncWithImpersonateWithSyncServiceAccount(t *testing.T) {
	projectName := "sync-test-project"
	serviceAccountName := "test-account"
	roleName := "test-account-sa-role"
	Given(t).
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		And(func() {
			destinationServiceAccounts := []v1alpha1.ApplicationDestinationServiceAccount{
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: serviceAccountName,
				},
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: "missing-serviceAccount",
				},
			}
			err := createTestServiceAccount(serviceAccountName, fixture.DeploymentNamespace())
			require.NoError(t, err)
			err = createTestAppProject(projectName, fixture.TestNamespace(), destinationServiceAccounts)
			require.NoError(t, err)
			err = createTestRole(roleName, fixture.DeploymentNamespace(), []rbac.PolicyRule{
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
			})
			require.NoError(t, err)

			err = createTestRoleBinding(roleName, serviceAccountName, fixture.DeploymentNamespace())
			require.NoError(t, err)
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
	roleName := "test-account-sa-role"
	Given(t).
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		And(func() {
			destinationServiceAccounts := []v1alpha1.ApplicationDestinationServiceAccount{
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: "missing-serviceAccount",
				},
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: serviceAccountName,
				},
			}
			err := createTestServiceAccount(serviceAccountName, fixture.DeploymentNamespace())
			require.NoError(t, err)
			err = createTestAppProject(projectName, fixture.TestNamespace(), destinationServiceAccounts)
			require.NoError(t, err)
			err = createTestRole(roleName, fixture.DeploymentNamespace(), []rbac.PolicyRule{
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
			})
			require.NoError(t, err)

			err = createTestRoleBinding(roleName, serviceAccountName, fixture.DeploymentNamespace())
			require.NoError(t, err)
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
	roleName := "test-account-sa-role"
	Given(t).
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		And(func() {
			destinationServiceAccounts := []v1alpha1.ApplicationDestinationServiceAccount{
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: serviceAccountName,
				},
			}
			err := createTestServiceAccount(serviceAccountName, fixture.DeploymentNamespace())
			require.NoError(t, err)
			err = createTestAppProject(projectName, fixture.TestNamespace(), destinationServiceAccounts)
			require.NoError(t, err)
			err = createTestRole(roleName, fixture.DeploymentNamespace(), []rbac.PolicyRule{
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
			})
			require.NoError(t, err)
			err = createTestRoleBinding(roleName, serviceAccountName, fixture.DeploymentNamespace())
			require.NoError(t, err)
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
			patch := []byte(fmt.Sprintf(`{"spec": {"destinations": [{"namespace": "%s"}]}}`, "!"+fixture.DeploymentNamespace()))

			_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(fixture.TestNamespace()).Patch(context.Background(), projectName, types.MergePatchType, patch, metav1.PatchOptions{})
			require.NoError(t, err)
		}).
		Refresh(v1alpha1.RefreshTypeNormal).
		Then().
		// With the impersonation feature enabled, Application sync must fail
		// as there is a valid match found in the available destination service accounts configured in AppProject
		// but the destination namespace is now disallowed.
		ExpectConsistently(SyncStatusIs(v1alpha1.SyncStatusCodeUnknown), WaitDuration, TimeoutDuration)
}

// createTestAppProject creates a test AppProject resource.
func createTestAppProject(name, namespace string, destinationServiceAccounts []v1alpha1.ApplicationDestinationServiceAccount) error {
	appProject := &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos:      []string{"*"},
			SourceNamespaces: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
			ClusterResourceWhitelist: []metav1.GroupKind{
				{
					Group: "*",
					Kind:  "*",
				},
			},
			DestinationServiceAccounts: destinationServiceAccounts,
		},
	}

	_, err := fixture.AppClientset.ArgoprojV1alpha1().AppProjects(namespace).Create(context.Background(), appProject, metav1.CreateOptions{})
	return err
}

// createTestRole creates a test Role resource.
func createTestRole(roleName, namespace string, rules []rbac.PolicyRule) error {
	role := &rbac.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      roleName,
			Namespace: namespace,
		},
		Rules: rules,
	}

	_, err := fixture.KubeClientset.RbacV1().Roles(namespace).Create(context.Background(), role, metav1.CreateOptions{})
	return err
}

// createTestRoleBinding creates a test RoleBinding resource.
func createTestRoleBinding(roleName, serviceAccountName, namespace string) error {
	roleBinding := &rbac.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: roleName + "-binding",
		},
		Subjects: []rbac.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      serviceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbac.RoleRef{
			Kind:     "Role",
			Name:     roleName,
			APIGroup: "rbac.authorization.k8s.io",
		},
	}

	_, err := fixture.KubeClientset.RbacV1().RoleBindings(namespace).Create(context.Background(), roleBinding, metav1.CreateOptions{})
	return err
}

// createTestServiceAccount creates a test ServiceAccount resource.
func createTestServiceAccount(name, namespace string) error {
	serviceAccount := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}

	_, err := fixture.KubeClientset.CoreV1().ServiceAccounts(namespace).Create(context.Background(), serviceAccount, metav1.CreateOptions{})
	return err
}
