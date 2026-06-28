package e2e

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/gitops-engine/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/pkg/sync/common"

	"github.com/argoproj/argo-cd/v3/common"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/clusterauth"
)

// TestShardReconcilesAssignedApps tests to see if a shard only reconciles apps deploying to the clusters
// it is managing
func TestShardsReconcileAssignedApps(t *testing.T) {
	// Skip if running in a remote cluster due to not supporting infrastructure at the moment
	if remote := IsRemote(); remote {
		t.Skip("At the moment this test only works when not in cluster")
	}

	ctx := Given(t)

	shard0ClusterName := createClusterSecretWithShard(ctx, 0, ctx.DeploymentNamespace())
	shard1ClusterName := createClusterSecretWithShard(ctx, 1, ctx.DeploymentNamespace())

	env := map[string]string{
		common.EnvControllerShard:    "0",
		common.EnvControllerReplicas: "2",
	}
	err := RestartProcess(ApplicationControllerProcName, env)
	require.NoError(t, err)

	ctx.
		Path("shards-sync-assigned-apps/app-1").
		Name("shard-test-app-shard-1").
		DestName(shard0ClusterName).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(ResourceHealthIs("ConfigMap", "app-1-cm", health.HealthStatusHealthy))

	ctx2 := GivenWithSameState(ctx).
		Path("shards-sync-assigned-apps/app-2").
		Timeout(5).
		Name("shard-test-app-shard-2").
		DestName(shard1ClusterName).
		When().
		IgnoreErrors().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationRunning)).
		Expect(ResourceHealthIs("ConfigMap", "app-2-cm", health.HealthStatusMissing))

	env[common.EnvControllerShard] = "1"
	err = RestartProcess(ApplicationControllerProcName, env)
	require.NoError(t, err)

	ctx2.
		When().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(HealthIs(health.HealthStatusHealthy)).
		Expect(ResourceHealthIs("ConfigMap", "app-2-cm", health.HealthStatusHealthy))
}

// createClusterSecretWithShard creates a cluster secret and assigns it to a shard
func createClusterSecretWithShard(ctx *Context, shard int, ns string) string {
	// Create a ServiceAccount, role, and role binding to be used for a bearer token
	serviceAccountName := DnsFriendly("argocd-e2e", "-shard-"+strconv.Itoa(shard)+"-sa-"+ctx.ShortID())
	err := clusterauth.CreateServiceAccount(KubeClientset, serviceAccountName, ns)
	require.NoError(ctx.T(), err)

	clusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: DnsFriendly("allow-all-shard"+strconv.Itoa(shard), "-"+ctx.ShortID()),
			Labels: map[string]string{
				TestingLabel: "true",
			},
		},
		Rules: []rbacv1.PolicyRule{{
			Verbs:     []string{"*"},
			Resources: []string{"*"},
			APIGroups: []string{"*"},
		}},
	}
	_, err = KubeClientset.RbacV1().ClusterRoles().Create(ctx.T().Context(), &clusterRole, metav1.CreateOptions{})
	require.NoError(ctx.T(), err)

	clusterRoleBinding := rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: DnsFriendly("allow-all-binding-shard"+strconv.Itoa(shard), "-"+ctx.ShortID()),
			Labels: map[string]string{
				TestingLabel: "true",
			},
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      serviceAccountName,
			Namespace: ns,
		}},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
	}
	_, err = KubeClientset.RbacV1().ClusterRoleBindings().Create(ctx.T().Context(), &clusterRoleBinding, metav1.CreateOptions{})
	require.NoError(ctx.T(), err)

	var token string
	// Trying to patch a ServiceAccount could fail so try again up to 20 seconds
	// See ./test/e2e/deployment_test.go:334 for exact error
	waitErr := wait.PollUntilContextTimeout(ctx.T().Context(), 1*time.Second, 20*time.Second, true, func(context.Context) (done bool, err error) {
		token, err = clusterauth.GetServiceAccountBearerToken(KubeClientset, ns, serviceAccountName, time.Second*60)
		return (err == nil && token != ""), nil
	})
	require.NoError(ctx.T(), waitErr)
	require.NotEmpty(ctx.T(), token)

	_, apiURL, err := extractKubeConfigValues()
	require.NoError(ctx.T(), err)

	clusterName := "test-sharding-shard" + strconv.Itoa(shard) + "-" + ctx.ShortID()
	// Query paramater for the server URL to be unique, this will be ignored by the Kubernetes API
	queryParam := "?shard=" + strconv.Itoa(shard)

	clusterSecretConfigJSON := ClusterConfig{
		BearerToken: token,
		TLSClientConfig: TLSClientConfig{
			Insecure: true,
		},
	}

	jsonStringBytes, err := json.Marshal(clusterSecretConfigJSON)
	require.NoError(ctx.T(), err)

	secret := buildArgoCDClusterSecret(clusterName, ArgoCDNamespace, clusterName, apiURL+queryParam,
		string(jsonStringBytes), "", "")
	secret.Data["shard"] = []byte(strconv.Itoa(shard))

	_, err = KubeClientset.CoreV1().Secrets(secret.Namespace).Create(ctx.T().Context(), &secret, metav1.CreateOptions{})
	require.NoError(ctx.T(), err)

	return clusterName
}
