package e2e

import (
	"context"
	"testing"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	rbac "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncWithImpersonateDisable(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "false").
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestSyncWithImpersonateDefaultNamespaceServiceAccountNoRBAC(t *testing.T) {
	Given(t).
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{}}
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

func TestSyncWithImpersonateDefaultNamespaceServiceAccountWithRBAC(t *testing.T) {
	roleName := "default-sa-role"
	//assert.NoError(t, err)
	Given(t).
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{}}
		}).
		And(func() {
			err := createTestRole(roleName, fixture.DeploymentNamespace(), []rbac.PolicyRule{
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
			assert.NoError(t, err)
			err = createTestRoleBinding(roleName, "default", fixture.DeploymentNamespace())
			assert.NoError(t, err)
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
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
			destinationServiceAccounts := []ApplicationDestinationServiceAccount{
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: serviceAccountName,
				},
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: "false-serviceAccount",
				},
			}
			err := createTestServiceAccount(serviceAccountName, fixture.DeploymentNamespace())
			assert.NoError(t, err)
			err = createTestAppProject(projectName, fixture.TestNamespace(), destinationServiceAccounts)
			assert.NoError(t, err)
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
			assert.NoError(t, err)

			err = createTestRoleBinding(roleName, serviceAccountName, fixture.DeploymentNamespace())
			assert.NoError(t, err)

		}).
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{}}
			app.Spec.Project = projectName
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeSynced))
}

func TestSyncWithImpersonateWithFalseServiceAccount(t *testing.T) {
	projectName := "false-test-project"
	serviceAccountName := "test-account"
	roleName := "test-account-sa-role"
	Given(t).
		SetTrackingMethod("annotation").
		Path("guestbook").
		When().
		SetParamInSettingConfigMap("application.sync.impersonation.enabled", "true").
		And(func() {
			destinationServiceAccounts := []ApplicationDestinationServiceAccount{
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: "false-serviceAccount",
				},
				{
					Server:                "*",
					Namespace:             fixture.DeploymentNamespace(),
					DefaultServiceAccount: serviceAccountName,
				},
			}
			err := createTestServiceAccount(serviceAccountName, fixture.DeploymentNamespace())
			assert.NoError(t, err)
			err = createTestAppProject(projectName, fixture.TestNamespace(), destinationServiceAccounts)
			assert.NoError(t, err)
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
			assert.NoError(t, err)

			err = createTestRoleBinding(roleName, serviceAccountName, fixture.DeploymentNamespace())
			assert.NoError(t, err)

		}).
		CreateFromFile(func(app *Application) {
			app.Spec.SyncPolicy = &SyncPolicy{Automated: &SyncPolicyAutomated{}}
			app.Spec.Project = projectName
		}).
		Then().
		Expect(SyncStatusIs(SyncStatusCodeOutOfSync))
}

// createTestAppProject creates a test AppProject resource.
func createTestAppProject(name, namespace string, destinationServiceAccounts []ApplicationDestinationServiceAccount) error {
	appProject := &AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: AppProjectSpec{
			SourceRepos:      []string{"*"},
			SourceNamespaces: []string{"*"},
			Destinations: []ApplicationDestination{
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
