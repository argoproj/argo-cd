package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/util/argo"
	"github.com/argoproj/argo-cd/v2/util/clusterauth"

	"github.com/argoproj/gitops-engine/pkg/health"
	. "github.com/argoproj/gitops-engine/pkg/sync/common"

	. "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v2/test/e2e/fixture/app"
)

// when we have a config map generator, AND the ignore annotation, it is ignored in the app's sync status
func TestDeployment(t *testing.T) {
	Given(t).
		Path("deployment").
		When().
		CreateApp().
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

func TestDeploymentWithAnnotationTrackingMode(t *testing.T) {
	ctx := Given(t)

	SetTrackingMethod(string(argo.TrackingMethodAnnotation))
	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			out, err := RunCli("app", "manifests", ctx.AppName())
			require.NoError(t, err)
			assert.Contains(t, out, fmt.Sprintf(`annotations:
    argocd.argoproj.io/tracking-id: %s:apps/Deployment:%s/nginx-deployment
`, ctx.AppName(), DeploymentNamespace()))
		})
}

func TestDeploymentWithLabelTrackingMode(t *testing.T) {
	ctx := Given(t)
	SetTrackingMethod(string(argo.TrackingMethodLabel))
	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			out, err := RunCli("app", "manifests", ctx.AppName())
			require.NoError(t, err)
			assert.Contains(t, out, fmt.Sprintf(`labels:
    app: nginx
    app.kubernetes.io/instance: %s
`, ctx.AppName()))
		})
}

func TestDeploymentWithoutTrackingMode(t *testing.T) {
	ctx := Given(t)
	ctx.
		Path("deployment").
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		When().
		Then().
		And(func(app *Application) {
			out, err := RunCli("app", "manifests", ctx.AppName())
			require.NoError(t, err)
			assert.Contains(t, out, fmt.Sprintf(`labels:
    app: nginx
    app.kubernetes.io/instance: %s
`, ctx.AppName()))
		})
}

// This test verifies that Argo CD can:
// A) Deploy to a cluster where the URL of the cluster contains a query parameter: e.g. https://(kubernetes-url):443/?context=some-val
// and
// B) Multiple users can deploy to the same K8s cluster, using above mechanism (but with different Argo CD Cluster Secrets, and different ServiceAccounts)
func TestDeployToKubernetesAPIURLWithQueryParameter(t *testing.T) {
	// We test with both a cluster-scoped, and a non-cluster scoped, Argo CD Cluster Secret.
	clusterScopedParam := []bool{false, true}
	for _, clusterScoped := range clusterScopedParam {
		EnsureCleanState(t)

		// Simulate two users, each with their own Argo CD cluster secret that can only deploy to their Namespace
		users := []string{E2ETestPrefix + "user1", E2ETestPrefix + "user2"}

		for _, username := range users {
			createNamespaceScopedUser(t, username, clusterScoped)

			GivenWithSameState(t).
				Name("e2e-test-app-"+username).
				Path("deployment").
				When().
				CreateWithNoNameSpace("--dest-namespace", username).
				Sync().
				Then().
				Expect(OperationPhaseIs(OperationSucceeded)).
				Expect(SyncStatusIs(SyncStatusCodeSynced)).
				Expect(HealthIs(health.HealthStatusHealthy))
		}
	}
}

// This test verifies that Argo CD can:
// When multiple Argo CD cluster secrets used to deploy to the same cluster (using query parameters), that the ServiceAccount RBAC
// fully enforces user boundary.
// Our simulated user's ServiceAccounts should not be able to deploy into a namespace that is outside that SA's RBAC.
func TestArgoCDSupportsMultipleServiceAccountsWithDifferingRBACOnSameCluster(t *testing.T) {
	// We test with both a cluster-scoped, and a non-cluster scoped, Argo CD Cluster Secret.
	clusterScopedParam := []bool{ /*false,*/ true}

	for _, clusterScoped := range clusterScopedParam {
		EnsureCleanState(t)

		// Simulate two users, each with their own Argo CD cluster secret that can only deploy to their Namespace
		users := []string{E2ETestPrefix + "user1", E2ETestPrefix + "user2"}

		for _, username := range users {
			createNamespaceScopedUser(t, username, clusterScoped)
		}

		for idx, username := range users {
			// we should use user-a's serviceaccount to deploy to user-b's namespace, and vice versa
			// - If everything as working as expected, this should fail.
			otherUser := users[(idx+1)%len(users)]

			// e.g. Attempt to deploy to user1's namespace, with user2's cluster Secret. This should fail, as user2's cluster Secret does not have the requisite permissions.
			consequences := GivenWithSameState(t).
				Name("e2e-test-app-"+username).
				DestName(E2ETestPrefix+"cluster-"+otherUser).
				Path("deployment").
				When().
				CreateWithNoNameSpace("--dest-namespace", username).IgnoreErrors().
				Sync().Then()

			// The error message differs based on whether the Argo CD Cluster Secret is namespace-scoped or cluster-scoped, but the idea is the same:
			// - Even when deploying to the same cluster using 2 separate ServiceAccounts, the RBAC of those ServiceAccounts should continue to fully enforce RBAC boundaries.

			if !clusterScoped {
				consequences.Expect(Condition(ApplicationConditionComparisonError, "Namespace \""+username+"\" for Deployment \"nginx-deployment\" is not managed"))
			} else {
				consequences.Expect(OperationMessageContains("User \"system:serviceaccount:" + otherUser + ":" + otherUser + "-serviceaccount\" cannot create resource \"deployments\" in API group \"apps\" in the namespace \"" + username + "\""))
			}
		}
	}
}

// generateReadOnlyClusterRoleandBindingForServiceAccount creates a ClusterRole/Binding that allows a ServiceAccount in a given namespace to read all resources on a cluster.
// - This allows the ServiceAccount to be used within a cluster-scoped Argo CD Cluster Secret
func generateReadOnlyClusterRoleandBindingForServiceAccount(roleSuffix string, serviceAccountNS string) (rbacv1.ClusterRole, rbacv1.ClusterRoleBinding) {
	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: E2ETestPrefix + "read-all-" + roleSuffix,
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"get", "list", "watch"},
			Resources: []string{"*"},
			APIGroups: []string{"*"},
		}},
	}

	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: E2ETestPrefix + "read-all-" + roleSuffix,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Namespace: serviceAccountNS,
			Name:      roleSuffix + "-serviceaccount",
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}

	return clusterRole, clusterRoleBinding
}

// buildArgoCDClusterSecret build (but does not create) an Argo CD Cluster Secret object with the given values
func buildArgoCDClusterSecret(secretName, secretNamespace, clusterName, clusterServer, clusterConfigJSON, clusterResources, clusterNamespaces string) corev1.Secret {
	res := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: secretNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"name":   ([]byte)(clusterName),
			"server": ([]byte)(clusterServer),
			"config": ([]byte)(string(clusterConfigJSON)),
		},
	}

	if clusterResources != "" {
		res.Data["clusterResources"] = ([]byte)(clusterResources)
	}

	if clusterNamespaces != "" {
		res.Data["namespaces"] = ([]byte)(clusterNamespaces)
	}

	return res
}

// createNamespaceScopedUser
// - username = name of Namespace the simulated user is able to deploy to
// - clusterScopedSecrets = whether the Service Account is namespace-scoped or cluster-scoped.
func createNamespaceScopedUser(t *testing.T, username string, clusterScopedSecrets bool) {
	// Create a new Namespace for our simulated user
	ns := corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: username,
		},
	}
	_, err := KubeClientset.CoreV1().Namespaces().Create(context.Background(), &ns, metav1.CreateOptions{})
	require.NoError(t, err)

	// Create a ServiceAccount in that Namespace, which will be used for the Argo CD Cluster SEcret
	serviceAccountName := username + "-serviceaccount"
	err = clusterauth.CreateServiceAccount(KubeClientset, serviceAccountName, ns.Name)
	require.NoError(t, err)

	// Create a Role that allows the ServiceAccount to read/write all within the Namespace
	role := rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:      E2ETestPrefix + "allow-all",
			Namespace: ns.Name,
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"*"},
			Resources: []string{"*"},
			APIGroups: []string{"*"},
		}},
	}
	_, err = KubeClientset.RbacV1().Roles(role.Namespace).Create(context.Background(), &role, metav1.CreateOptions{})
	require.NoError(t, err)

	// Bind the Role with the ServiceAccount in the Namespace
	roleBinding := rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      E2ETestPrefix + "allow-all-binding",
			Namespace: ns.Name,
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: ns.Name,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "Role",
			Name:     role.Name,
		},
	}
	_, err = KubeClientset.RbacV1().RoleBindings(roleBinding.Namespace).Create(context.Background(), &roleBinding, metav1.CreateOptions{})
	require.NoError(t, err)

	// Retrieve the bearer token from the ServiceAccount
	token, err := clusterauth.GetServiceAccountBearerToken(KubeClientset, ns.Name, serviceAccountName, time.Second*60)
	require.NoError(t, err)
	assert.NotEmpty(t, token)

	// In order to test a cluster-scoped Argo CD Cluster Secret, we may optionally grant the ServiceAccount read-all permissions at cluster scope.
	if clusterScopedSecrets {
		clusterRole, clusterRoleBinding := generateReadOnlyClusterRoleandBindingForServiceAccount(username, username)

		_, err := KubeClientset.RbacV1().ClusterRoles().Create(context.Background(), &clusterRole, metav1.CreateOptions{})
		require.NoError(t, err)

		_, err = KubeClientset.RbacV1().ClusterRoleBindings().Create(context.Background(), &clusterRoleBinding, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	// Build the Argo CD Cluster Secret by using the service account token, and extracting needed values from kube config
	clusterSecretConfigJSON := ClusterConfig{
		BearerToken: token,
		TLSClientConfig: TLSClientConfig{
			Insecure: true,
		},
	}

	jsonStringBytes, err := json.Marshal(clusterSecretConfigJSON)
	require.NoError(t, err)

	_, apiURL, err := extractKubeConfigValues()
	require.NoError(t, err)

	clusterResourcesField := ""
	namespacesField := ""

	if !clusterScopedSecrets {
		clusterResourcesField = "false"
		namespacesField = ns.Name
	}

	// We create an Argo CD cluster Secret declaratively, using the K8s client, rather than via CLI, as the CLI doesn't currently
	// support Kubernetes API server URLs with query parameters.

	secret := buildArgoCDClusterSecret("test-"+username, ArgoCDNamespace, E2ETestPrefix+"cluster-"+username, apiURL+"?user="+username,
		string(jsonStringBytes), clusterResourcesField, namespacesField)

	// Finally, create the Cluster secret in the Argo CD E2E namespace
	_, err = KubeClientset.CoreV1().Secrets(secret.Namespace).Create(context.Background(), &secret, metav1.CreateOptions{})
	require.NoError(t, err)
}

// extractKubeConfigValues returns contents of the local environment's kubeconfig, using standard path resolution mechanism.
// Returns:
// - contents of kubeconfig
// - server name (within the kubeconfig)
// - error
func extractKubeConfigValues() (string, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()

	config, err := loadingRules.Load()
	if err != nil {
		return "", "", err
	}

	context, ok := config.Contexts[config.CurrentContext]
	if !ok || context == nil {
		return "", "", fmt.Errorf("no context")
	}

	cluster, ok := config.Clusters[context.Cluster]
	if !ok || cluster == nil {
		return "", "", fmt.Errorf("no cluster")
	}

	var kubeConfigDefault string

	paths := loadingRules.Precedence
	{
		// For all the kubeconfig paths, look for one that exists
		for _, path := range paths {
			_, err = os.Stat(path)
			if err == nil {
				// Success
				kubeConfigDefault = path
				break
			} // Otherwise, continue.
		}

		if kubeConfigDefault == "" {
			return "", "", fmt.Errorf("unable to retrieve kube config path")
		}
	}

	kubeConfigContents, err := os.ReadFile(kubeConfigDefault)
	if err != nil {
		return "", "", err
	}

	return string(kubeConfigContents), cluster.Server, nil
}
