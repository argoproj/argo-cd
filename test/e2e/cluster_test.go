package e2e

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/health"
	synccommon "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/sync/common"

	"github.com/argoproj/argo-cd/v3/common"
	clusterpkg "github.com/argoproj/argo-cd/v3/pkg/apiclient/cluster"
	. "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture"
	accountFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/account"
	"github.com/argoproj/argo-cd/v3/test/e2e/fixture/app"
	clusterFixture "github.com/argoproj/argo-cd/v3/test/e2e/fixture/cluster"
	"github.com/argoproj/argo-cd/v3/util/errors"
)

func TestClusterList(t *testing.T) {
	fixture.SkipIfAlreadyRun(t)
	defer fixture.RecordTestRun(t)

	last := ""
	expectedRegexStr := fmt.Sprintf("^SERVER +NAME +VERSION +STATUS +MESSAGE +PROJECT\nhttps://kubernetes\\.default\\.svc +in-cluster +%v +Successful *$",
		regexp.QuoteMeta(fixture.GetVersions(t).ServerVersion.String()))
	expectedRegexp := regexp.MustCompile(expectedRegexStr)

	ctx := clusterFixture.Given(t)
	ctx.Project(fixture.ProjectName)

	// We need an application targeting the cluster, otherwise the test will
	// fail if run isolated.
	app.GivenWithSameState(ctx).
		Path(guestbookPath).
		When().
		CreateApp()

	tries := 25
	matches := false
	for i := 0; i <= tries; i++ {
		clusterFixture.GivenWithSameState(ctx).
			When().
			List().
			Then().
			AndCLIOutput(func(output string, _ error) {
				last = output
			})
		matches = expectedRegexp.MatchString(last)
		if matches {
			break
		} else if i < tries {
			// We retry with a simple backoff
			time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
		}
	}
	assert.Regexp(t, expectedRegexp, last)
}

func TestClusterAdd(t *testing.T) {
	ctx := clusterFixture.Given(t)
	ctx.Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		List().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Contains(t, fixture.NormalizeOutput(output), fmt.Sprintf(`https://kubernetes.default.svc %s %v Successful %s`, ctx.GetName(), fixture.GetVersions(t).ServerVersion.String(), fixture.ProjectName))
		})
}

func TestClusterAddPermissionDenied(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{}, "org-admin")

	clusterFixture.
		GivenWithSameState(ctx).
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		IgnoreErrors().
		Create().
		Then().
		AndCLIOutput(func(_ string, err error) {
			assert.ErrorContains(t, err, "PermissionDenied desc = permission denied")
		})
}

func TestClusterAddAllowed(t *testing.T) {
	accountCtx := accountFixture.Given(t)
	accountCtx.Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "clusters",
				Action:   "create",
				Scope:    fixture.ProjectName + "/*",
			},
			{
				Resource: "clusters",
				Action:   "get",
				Scope:    fixture.ProjectName + "/*",
			},
		}, "org-admin")

	ctx := clusterFixture.GivenWithSameState(accountCtx)
	ctx.Project(fixture.ProjectName).
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		List().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Contains(t, fixture.NormalizeOutput(output), fmt.Sprintf(`https://kubernetes.default.svc %s %v Successful %s`, ctx.GetName(), fixture.GetVersions(t).ServerVersion.String(), fixture.ProjectName))
		})
}

func TestClusterListDenied(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "clusters",
				Action:   "create",
				Scope:    fixture.ProjectName + "/*",
			},
		}, "org-admin")

	clusterFixture.
		GivenWithSameState(ctx).
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		List().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Equal(t, "SERVER  NAME  VERSION  STATUS  MESSAGE  PROJECT", output)
		})
}

func TestClusterSet(t *testing.T) {
	ctx := clusterFixture.Given(t)
	ctx.Project(fixture.ProjectName).
		Namespaces([]string{"namespace-edit-1", "namespace-edit-2"}).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		SetNamespaces().
		GetByName().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Contains(t, output, "namespace-edit-1")
			assert.Contains(t, output, "namespace-edit-2")
		})
}

func TestClusterGet(t *testing.T) {
	fixture.SkipIfAlreadyRun(t)
	fixture.EnsureCleanState(t)
	defer fixture.RecordTestRun(t)
	output := errors.NewHandler(t).FailOnErr(fixture.RunCli("cluster", "get", "https://kubernetes.default.svc")).(string)

	assert.Contains(t, output, "name: in-cluster")
	assert.Contains(t, output, "server: https://kubernetes.default.svc")
	assert.Contains(t, output, fmt.Sprintf(`serverVersion: %v`, fixture.GetVersions(t).ServerVersion.String()))
	assert.Contains(t, output, `config:
  tlsClientConfig:
    insecure: false`)

	assert.Contains(t, output, `status: Successful`)
}

func TestClusterNameInRestAPI(t *testing.T) {
	fixture.EnsureCleanState(t)

	var cluster Cluster
	err := fixture.DoHttpJsonRequest("GET", "/api/v1/clusters/in-cluster?id.type=name", &cluster)
	require.NoError(t, err)

	assert.Equal(t, "in-cluster", cluster.Name)
	assert.Contains(t, cluster.Server, "https://kubernetes.default.svc")

	err = fixture.DoHttpJsonRequest("PUT",
		"/api/v1/clusters/in-cluster?id.type=name&updatedFields=labels", &cluster, []byte(`{"labels":{"test": "val"}}`)...)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"test": "val"}, cluster.Labels)
}

func TestClusterURLInRestAPI(t *testing.T) {
	fixture.EnsureCleanState(t)

	clusterURL := url.QueryEscape(KubernetesInternalAPIServerAddr)

	var cluster Cluster
	err := fixture.DoHttpJsonRequest("GET", "/api/v1/clusters/"+clusterURL, &cluster)
	require.NoError(t, err)

	assert.Equal(t, "in-cluster", cluster.Name)
	assert.Contains(t, cluster.Server, "https://kubernetes.default.svc")

	err = fixture.DoHttpJsonRequest("PUT",
		fmt.Sprintf("/api/v1/clusters/%s?&updatedFields=labels", clusterURL), &cluster, []byte(`{"labels":{"test": "val"}}`)...)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"test": "val"}, cluster.Labels)
}

func TestClusterSkipReconcileAnnotation(t *testing.T) {
	fixture.EnsureCleanState(t)

	clusterURL := url.QueryEscape(KubernetesInternalAPIServerAddr)

	var cluster Cluster
	err := fixture.DoHttpJsonRequest("PUT",
		fmt.Sprintf("/api/v1/clusters/%s?updatedFields=annotations", clusterURL),
		&cluster,
		fmt.Appendf(nil, `{"annotations":{%q:"true"}}`, "argocd.argoproj.io/skip-reconcile")...)
	require.NoError(t, err)
	assert.Equal(t, "true", cluster.Annotations["argocd.argoproj.io/skip-reconcile"])

	var cluster2 Cluster
	err = fixture.DoHttpJsonRequest("GET", "/api/v1/clusters/"+clusterURL, &cluster2)
	require.NoError(t, err)
	assert.Equal(t, "in-cluster", cluster2.Name)
	assert.Equal(t, "true", cluster2.Annotations["argocd.argoproj.io/skip-reconcile"])

	err = fixture.DoHttpJsonRequest("PUT",
		fmt.Sprintf("/api/v1/clusters/%s?updatedFields=annotations", clusterURL),
		&cluster,
		[]byte(`{"annotations":{}}`)...)
	require.NoError(t, err)
}

func TestClusterDeleteDenied(t *testing.T) {
	ctx := accountFixture.Given(t)
	ctx.Name("test").
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "clusters",
				Action:   "create",
				Scope:    fixture.ProjectName + "/*",
			},
			{
				Resource: "clusters",
				Action:   "get",
				Scope:    fixture.ProjectName + "/*",
			},
		}, "org-admin")

	// Attempt to remove cluster creds by name
	clusterFixture.
		GivenWithSameState(ctx).
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		DeleteByName().
		Then().
		AndCLIOutput(func(_ string, err error) {
			assert.ErrorContains(t, err, "PermissionDenied desc = permission denied")
		})

	// Attempt to remove cluster creds by server
	clusterFixture.
		GivenWithSameState(ctx).
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		Create().
		DeleteByServer().
		Then().
		AndCLIOutput(func(_ string, err error) {
			assert.ErrorContains(t, err, "PermissionDenied desc = permission denied")
		})
}

func TestClusterDelete(t *testing.T) {
	ctx := clusterFixture.Given(t)
	accountFixture.GivenWithSameState(ctx).
		When().
		Create().
		Login().
		SetPermissions([]fixture.ACL{
			{
				Resource: "clusters",
				Action:   "create",
				Scope:    fixture.ProjectName + "/*",
			},
			{
				Resource: "clusters",
				Action:   "get",
				Scope:    fixture.ProjectName + "/*",
			},
			{
				Resource: "clusters",
				Action:   "delete",
				Scope:    fixture.ProjectName + "/*",
			},
		}, "org-admin")

	clstAction := ctx.
		Project(fixture.ProjectName).
		Upsert(true).
		Server(KubernetesInternalAPIServerAddr).
		When().
		CreateWithRBAC()
	clstAction.
		Then().
		Expect().
		AndCLIOutput(func(_ string, err error) {
			assert.NoError(t, err)
		})

	// Check that RBAC is created
	_, err := fixture.Run("", "kubectl", "get", "serviceaccount", "argocd-manager", "-n", "kube-system")
	require.NoError(t, err, "Expected no error from not finding serviceaccount argocd-manager")

	_, err = fixture.Run("", "kubectl", "get", "clusterrole", "argocd-manager-role")
	require.NoError(t, err, "Expected no error from not finding clusterrole argocd-manager-role")

	_, err = fixture.Run("", "kubectl", "get", "clusterrolebinding", "argocd-manager-role-binding")
	require.NoError(t, err, "Expected no error from not finding clusterrolebinding argocd-manager-role-binding")

	clstAction.DeleteByName().
		Then().
		AndCLIOutput(func(output string, _ error) {
			assert.Equal(t, fmt.Sprintf("Cluster '%s' removed", ctx.GetName()), output)
		})

	// Check that RBAC is removed after delete
	output, err := fixture.Run("", "kubectl", "get", "serviceaccount", "argocd-manager", "-n", "kube-system")
	require.Error(t, err, "Expected error from not finding serviceaccount argocd-manager but got:\n%s", output)

	output, err = fixture.Run("", "kubectl", "get", "clusterrole", "argocd-manager-role")
	require.Error(t, err, "Expected error from not finding clusterrole argocd-manager-role but got:\n%s", output)

	output, err = fixture.Run("", "kubectl", "get", "clusterrolebinding", "argocd-manager-role-binding")
	assert.Error(t, err, "Expected error from not finding clusterrolebinding argocd-manager-role-binding but got:\n%s", output)
}

func TestClusterDefaultCABundle(t *testing.T) {
	if fixture.IsRemote() {
		t.Skip("the test cluster CA is not available to a remote Argo CD workload")
	}
	kubeConfig := rest.CopyConfig(fixture.KubeConfig)
	require.NoError(t, rest.LoadTLSFiles(kubeConfig))
	if kubeConfig.Insecure || len(kubeConfig.CAData) == 0 {
		t.Skip("the kubeconfig of the test cluster does not verify its API server certificate")
	}

	ctx := clusterFixture.Given(t)
	server := kubeConfig.Host

	setClusterCABundle(t, string(kubeConfig.CAData))
	registerCluster(t, ctx, server, createClusterAdminToken(t, ctx.DeploymentNamespace()))

	appCtx := app.GivenWithSameState(ctx)
	appCtx.Path(guestbookPath).
		DestServer(server).
		When().
		CreateApp()
	refreshUntilClusterTrusted(t, appCtx, true)
	appCtx.When().
		Sync().
		Then().
		Expect(app.OperationPhaseIs(synccommon.OperationSucceeded)).
		Expect(app.SyncStatusIs(SyncStatusCodeSynced)).
		Expect(app.HealthIs(health.HealthStatusHealthy))

	deleteClusterCABundle(t)
	refreshUntilClusterTrusted(t, appCtx, false)

	setClusterCABundle(t, string(kubeConfig.CAData))
	refreshUntilClusterTrusted(t, appCtx, true)
}

func registerCluster(t *testing.T, ctx *clusterFixture.Context, server, bearerToken string) {
	t.Helper()
	_, clusterClient, err := fixture.ArgoCDClientset.NewClusterClient()
	require.NoError(t, err)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := clusterClient.Create(t.Context(), &clusterpkg.ClusterCreateRequest{
			Cluster: &Cluster{
				Server: server,
				Name:   ctx.GetName(),
				Config: ClusterConfig{BearerToken: bearerToken},
			},
			Upsert: true,
		})
		assert.NoError(c, err)
	}, time.Minute, time.Second)
}

func refreshUntilClusterTrusted(t *testing.T, appCtx *app.Context, trusted bool) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		_, err := fixture.RunCli("app", "get", appCtx.AppQualifiedName(), "--refresh")
		require.NoError(c, err)
		application, err := fixture.AppClientset.ArgoprojV1alpha1().Applications(appCtx.AppNamespace()).Get(t.Context(), appCtx.AppName(), metav1.GetOptions{})
		require.NoError(c, err)
		untrusted := slices.ContainsFunc(application.Status.Conditions, func(condition ApplicationCondition) bool {
			return condition.Type == ApplicationConditionComparisonError && strings.Contains(condition.Message, "tls: failed to verify certificate")
		})
		assert.NotEqual(c, trusted, untrusted, "conditions: %v", application.Status.Conditions)
	}, time.Minute, time.Second)
}

func createClusterAdminToken(t *testing.T, namespace string) string {
	t.Helper()
	sa, err := fixture.KubeClientset.CoreV1().ServiceAccounts(namespace).Create(t.Context(), &corev1.ServiceAccount{
		Name: "cluster-ca-bundle",
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	_, err = fixture.KubeClientset.RbacV1().ClusterRoleBindings().Create(t.Context(), &rbacv1.ClusterRoleBinding{
		Name:     namespace + "-cluster-ca-bundle",
		Labels:   map[string]string{fixture.TestingLabel: "true"},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "cluster-admin"},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: namespace}},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	token, err := fixture.KubeClientset.CoreV1().ServiceAccounts(namespace).CreateToken(t.Context(), sa.Name, &authenticationv1.TokenRequest{
		Spec: authenticationv1.TokenRequestSpec{ExpirationSeconds: ptr.To(int64(time.Hour.Seconds()))},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	return token.Status.Token
}

func setClusterCABundle(t *testing.T, caBundle string) {
	t.Helper()
	_, err := fixture.KubeClientset.CoreV1().ConfigMaps(fixture.TestNamespace()).Create(t.Context(), &corev1.ConfigMap{
		Name:   common.ArgoCDClusterCAConfigMapName,
		Labels: map[string]string{"app.kubernetes.io/part-of": "argocd"},
		Data:   map[string]string{common.ArgoCDClusterCAConfigMapKey: caBundle},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
}

func deleteClusterCABundle(t *testing.T) {
	t.Helper()
	require.NoError(t, fixture.KubeClientset.CoreV1().ConfigMaps(fixture.TestNamespace()).Delete(t.Context(), common.ArgoCDClusterCAConfigMapName, metav1.DeleteOptions{}))
}
