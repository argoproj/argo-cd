package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	. "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller/sharding"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	. "github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	"github.com/argoproj/argo-cd/v3/util/clusterauth"
	"github.com/argoproj/argo-cd/v3/util/db"
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
// TestShardsReconcileClusterFirstSeenOnUpdate covers a cluster that reaches the
// sharding cache through an update event instead of an add. A cluster secret
// that cannot be parsed when it is created is dropped by the add handler, and
// the update that fixes it is dropped too, because its old object cannot be
// parsed either. The next update, here a label change, is the first event the
// sharding cache sees for that cluster. It must be assigned a shard, or the
// replica that owns it never reconciles its applications and every other
// cluster keeps the index it had without it.
func TestShardsReconcileClusterFirstSeenOnUpdate(t *testing.T) {
	if remote := IsRemote(); remote {
		t.Skip("At the moment this test only works when not in cluster")
	}

	ctx := Given(t)

	// The shard the cluster belongs to must differ from 0, because a cluster
	// without an assigned shard falls back to shard 0.
	const shard = 1

	token := createClusterBearerToken(ctx, "first-seen-on-update", ctx.DeploymentNamespace())
	_, apiURL, err := extractKubeConfigValues()
	require.NoError(t, err)
	clusterName := "test-sharding-first-seen-on-update-" + ctx.ShortID()
	server := apiURL + "?first-seen-on-update=" + ctx.ShortID()
	validConfig, err := json.Marshal(ClusterConfig{BearerToken: token, Insecure: true})
	require.NoError(t, err)

	// Create the secret with a config that does not parse, so that the add
	// event is dropped. Recreate it until round-robin puts the cluster on the
	// shard under test: the order depends on the secret UID.
	secrets := KubeClientset.CoreV1().Secrets(ArgoCDNamespace)
	var secret *corev1.Secret
	for range 10 {
		s := buildArgoCDClusterSecret(clusterName, ArgoCDNamespace, clusterName, server, "{not json", "", "")
		secret, err = secrets.Create(t.Context(), &s, metav1.CreateOptions{})
		require.NoError(t, err)
		if expectedClusterShard(t, server, secret.UID) == shard {
			break
		}
		require.NoError(t, secrets.Delete(t.Context(), secret.Name, metav1.DeleteOptions{}))
		secret = nil
	}
	require.NotNil(t, secret, "could not place the cluster on shard %d", shard)
	t.Cleanup(func() {
		_ = secrets.Delete(context.Background(), clusterName, metav1.DeleteOptions{})
	})

	// Start the controller only now, so that it sees the secret while it does
	// not parse, whether through its initial list or through the add event.
	env := map[string]string{
		common.EnvControllerShard:             strconv.Itoa(shard),
		common.EnvControllerReplicas:          "2",
		common.EnvControllerShardingAlgorithm: common.RoundRobinShardingAlgorithm,
	}
	require.NoError(t, RestartProcess(ApplicationControllerProcName, env))
	waitForControllerReady(t)

	// Fix the config. This update is dropped as well, because its old object
	// does not parse.
	patch, err := json.Marshal(map[string]any{"data": map[string][]byte{"config": validConfig}})
	require.NoError(t, err)
	_, err = secrets.Patch(t.Context(), clusterName, apitypes.MergePatchType, patch, metav1.PatchOptions{})
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Change nothing the distribution depends on. This is the first event for
	// the cluster that reaches the sharding cache, as the informer's periodic
	// resync of the secret would be.
	patch, err = json.Marshal(map[string]any{"metadata": map[string]any{"labels": map[string]string{"e2e-resync": "true"}}})
	require.NoError(t, err)
	_, err = secrets.Patch(t.Context(), clusterName, apitypes.MergePatchType, patch, metav1.PatchOptions{})
	require.NoError(t, err)

	ctx.
		Path("shards-sync-assigned-apps/app-1").
		Name("shard-test-app-first-seen-on-update").
		DestName(clusterName).
		When().
		CreateApp().
		Sync().
		Then().
		Expect(OperationPhaseIs(OperationSucceeded)).
		Expect(SyncStatusIs(SyncStatusCodeSynced)).
		Expect(ResourceHealthIs("ConfigMap", "app-1-cm", health.HealthStatusHealthy))
}

// waitForControllerReady waits until the restarted application controller
// serves its health endpoint, then gives its cluster secret informer, which
// starts asynchronously, time to list the existing secrets.
func waitForControllerReady(t *testing.T) {
	t.Helper()
	err := wait.PollUntilContextTimeout(t.Context(), time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:8082/healthz", http.NoBody)
		if err != nil {
			return false, err
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return false, nil
		}
		_ = resp.Body.Close()
		return resp.StatusCode == http.StatusOK, nil
	})
	require.NoError(t, err)
	time.Sleep(5 * time.Second)
}

// expectedClusterShard returns the round-robin shard a freshly started
// controller with two replicas would assign to server, if the cluster secret
// for it had the given UID.
func expectedClusterShard(t *testing.T, server string, uid apitypes.UID) int {
	t.Helper()
	list, err := KubeClientset.CoreV1().Secrets(ArgoCDNamespace).List(t.Context(), metav1.ListOptions{
		LabelSelector: common.LabelKeySecretType + "=" + common.LabelValueSecretTypeCluster,
	})
	require.NoError(t, err)
	clusters := []Cluster{{Server: KubernetesInternalAPIServerAddr}}
	for i := range list.Items {
		c, err := db.SecretToCluster(&list.Items[i])
		if err != nil || c.Server == server {
			continue
		}
		if c.Server == KubernetesInternalAPIServerAddr {
			clusters[0] = *c
			continue
		}
		clusters = append(clusters, *c)
	}
	clusters = append(clusters, Cluster{ID: string(uid), Server: server})
	distribution := sharding.NewClusterSharding(nil, 0, 2, common.RoundRobinShardingAlgorithm)
	distribution.Init(&ClusterList{Items: clusters}, &ApplicationList{})
	return distribution.GetDistribution()[server]
}

// createClusterBearerToken creates a cluster-admin ServiceAccount in ns and
// returns a bearer token for it, to be used in a cluster secret.
func createClusterBearerToken(ctx *Context, suffix string, ns string) string {
	// Create a ServiceAccount, role, and role binding to be used for a bearer token
	serviceAccountName := DnsFriendly("argocd-e2e", "-"+suffix+"-sa-"+ctx.ShortID())
	err := clusterauth.CreateServiceAccount(KubeClientset, serviceAccountName, ns)
	require.NoError(ctx.T(), err)

	clusterRole := rbacv1.ClusterRole{
		Name: DnsFriendly("allow-all-"+suffix, "-"+ctx.ShortID()),
		Labels: map[string]string{
			TestingLabel: "true",
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
		Name: DnsFriendly("allow-all-binding-"+suffix, "-"+ctx.ShortID()),
		Labels: map[string]string{
			TestingLabel: "true",
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
	return token
}

func createClusterSecretWithShard(ctx *Context, shard int, ns string) string {
	token := createClusterBearerToken(ctx, "shard-"+strconv.Itoa(shard), ns)

	_, apiURL, err := extractKubeConfigValues()
	require.NoError(ctx.T(), err)

	clusterName := "test-sharding-shard" + strconv.Itoa(shard) + "-" + ctx.ShortID()
	// Query paramater for the server URL to be unique, this will be ignored by the Kubernetes API
	queryParam := "?shard=" + strconv.Itoa(shard)

	clusterSecretConfigJSON := ClusterConfig{
		BearerToken: token,
		Insecure:    true,
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
