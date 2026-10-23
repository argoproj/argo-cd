package admin

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/cache/appstate"

	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_loadClustersSkipsApplicationWithRemovedCluster(t *testing.T) {
	argoCDCM := &corev1.ConfigMap{
		Name:      "argocd-cm",
		Namespace: "argocd",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string]string{},
	}
	argoCDCmdCM := &corev1.ConfigMap{
		Name:      "argocd-cmd-params-cm",
		Namespace: "argocd",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      "argocd-secret",
		Namespace: "argocd",
		Data: map[string][]byte{
			"server.secretkey": []byte("test"),
		},
	}
	app := &v1alpha1.Application{
		Name:      "test",
		Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "test",
			},
		},
	}
	// Applications can outlive their destination cluster and must not prevent stats for healthy clusters from loading.
	staleApp := app.DeepCopy()
	staleApp.Name = "stale"
	staleApp.Spec.Destination.Server = "https://removed-cluster.example.com"
	staleApp.Spec.Destination.Namespace = "stale"
	ctx := t.Context()
	kubeClient := fake.NewClientset(argoCDCM, argoCDCmdCM, argoCDSecret)
	appClient := fakeapps.NewSimpleClientset(app, staleApp)
	oldHooks := log.StandardLogger().ReplaceHooks(log.LevelHooks{})
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(oldHooks) })
	logHook := logtest.NewGlobal()
	cacheSrc := func() (*appstate.Cache, error) {
		return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
	}
	clusters, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
	require.NoError(t, err)
	foundWarning := false
	for _, entry := range logHook.AllEntries() {
		if strings.Contains(entry.Message, `Skipping application "argocd/stale"`) {
			foundWarning = true
			break
		}
	}
	assert.True(t, foundWarning, "expected a warning about the application with a removed destination cluster")
	for i := range clusters {
		// These change, nil them to avoid testing them.
		clusters[i].ConfigHash = nil
		clusters[i].Info.ConnectionState.ModifiedAt = nil
	}
	expected := []ClusterWithInfo{{
		Cluster: v1alpha1.Cluster{
			ID:     "",
			Server: "https://kubernetes.default.svc",
			Name:   "in-cluster",
			Info: v1alpha1.ClusterInfo{
				ConnectionState: v1alpha1.ConnectionState{
					Status: "Successful",
				},
				ServerVersion: "0.0.0",
			},
			Shard: new(int64(0)),
		},
		Namespaces: []string{"test"},
	}}
	assert.Equal(t, expected, clusters)
}

func Test_loadClusters_ShardingAlgorithm(t *testing.T) {
	var logOutput bytes.Buffer
	originalOutput := log.StandardLogger().Out
	log.SetOutput(&logOutput)
	defer log.SetOutput(originalOutput)

	originalLevel := log.GetLevel()
	log.SetLevel(log.DebugLevel)
	defer log.SetLevel(originalLevel)

	argoCDCM := &corev1.ConfigMap{
		Name:      "argocd-cm",
		Namespace: "argocd",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string]string{},
	}
	argoCDCmdCM := &corev1.ConfigMap{
		Name:      "argocd-cmd-params-cm",
		Namespace: "argocd",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      "argocd-secret",
		Namespace: "argocd",
		Data: map[string][]byte{
			"server.secretkey": []byte("test"),
		},
	}
	app := &v1alpha1.Application{
		Name:      "test",
		Namespace: "argocd",
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "test",
			},
		},
	}

	t.Run("argocd-cmd-params-cm is missing", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := fake.NewClientset(argoCDCM, argoCDSecret)
		appClient := fakeapps.NewSimpleClientset(app)
		cacheSrc := func() (*appstate.Cache, error) {
			return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
		}
		_, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
		require.NoError(t, err)
		require.Contains(t, logOutput.String(), "Using filter function:  legacy")
	})

	t.Run("argocd-cmd-params-cm has non-default value", func(t *testing.T) {
		cmdParamsCopy := argoCDCmdCM.DeepCopy()
		cmdParamsCopy.Data["controller.sharding.algorithm"] = "round-robin"

		ctx := t.Context()
		kubeClient := fake.NewClientset(argoCDCM, cmdParamsCopy, argoCDSecret)
		appClient := fakeapps.NewSimpleClientset(app)
		cacheSrc := func() (*appstate.Cache, error) {
			return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
		}
		_, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
		require.NoError(t, err)
		require.Contains(t, logOutput.String(), "Using filter function:  round-robin")
	})

	t.Run("argocd-cmd-params-cm does not contain controller.sharding.algorithm key", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := fake.NewClientset(argoCDCM, argoCDCmdCM, argoCDSecret)
		appClient := fakeapps.NewSimpleClientset(app)
		cacheSrc := func() (*appstate.Cache, error) {
			return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
		}
		_, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
		require.NoError(t, err)
		require.Contains(t, logOutput.String(), "Using filter function:  legacy")
	})
}

func TestNewGenClusterConfigCommand_QPSAndBurstFlagsRegistered(t *testing.T) {
	cmd := NewGenClusterConfigCommand(clientcmd.NewDefaultPathOptions())

	qpsFlag := cmd.Flags().Lookup("k8s-client-qps")
	require.NotNil(t, qpsFlag, "--k8s-client-qps flag should be registered")
	assert.Equal(t, "0", qpsFlag.DefValue)
	assert.Contains(t, qpsFlag.Usage, "QPS")

	burstFlag := cmd.Flags().Lookup("k8s-client-burst")
	require.NotNil(t, burstFlag, "--k8s-client-burst flag should be registered")
	assert.Equal(t, "0", burstFlag.DefValue)
	assert.Contains(t, burstFlag.Usage, "Burst")
}

func newLoadClustersTestClients() (*fake.Clientset, *fakeapps.Clientset) {
	argoCDCM := &corev1.ConfigMap{
		Name:      "argocd-cm",
		Namespace: "argocd",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "argocd",
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		Name:      "argocd-secret",
		Namespace: "argocd",
		Data: map[string][]byte{
			"server.secretkey": []byte("test"),
		},
	}
	return fake.NewClientset(argoCDCM, argoCDSecret), fakeapps.NewSimpleClientset()
}

func newInMemoryCacheSrc() func() (*appstate.Cache, error) {
	cache := appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute)
	return func() (*appstate.Cache, error) {
		return cache, nil
	}
}

func Test_loadClustersOmitsClustersOfOtherShards(t *testing.T) {
	kubeClient, appClient := newLoadClustersTestClients()

	// The in-cluster is always assigned to shard 0, so filtering by shard 1 must not return any clusters,
	// rather than empty placeholder entries.
	clusters, err := loadClusters(t.Context(), kubeClient, appClient, 3, "", "argocd", false, newInMemoryCacheSrc(), 1, "", "", "")
	require.NoError(t, err)
	assert.Empty(t, clusters)

	clusters, err = loadClusters(t.Context(), kubeClient, appClient, 3, "", "argocd", false, newInMemoryCacheSrc(), 0, "", "", "")
	require.NoError(t, err)
	require.Len(t, clusters, 1)
	assert.Equal(t, "https://kubernetes.default.svc", clusters[0].Server)
}

func Test_loadClustersWarnsWhenClusterInfoIsMissing(t *testing.T) {
	kubeClient, appClient := newLoadClustersTestClients()
	oldHooks := log.StandardLogger().ReplaceHooks(log.LevelHooks{})
	t.Cleanup(func() { log.StandardLogger().ReplaceHooks(oldHooks) })
	logHook := logtest.NewGlobal()

	clusters, err := loadClusters(t.Context(), kubeClient, appClient, 1, "", "argocd", false, newInMemoryCacheSrc(), -1, "", "", "")
	require.NoError(t, err)
	require.Len(t, clusters, 1)

	foundWarning := false
	for _, entry := range logHook.AllEntries() {
		if strings.Contains(entry.Message, `Failed to get info of cluster "https://kubernetes.default.svc" from cache`) {
			foundWarning = true
			break
		}
	}
	assert.True(t, foundWarning, "expected a warning about the cluster info missing from the cache")
}

func Test_printStatsSummary(t *testing.T) {
	withResources := func(shard int, count int64) ClusterWithInfo {
		return ClusterWithInfo{
			Info:  v1alpha1.ClusterInfo{CacheInfo: v1alpha1.ClusterCacheInfo{ResourcesCount: count}},
			Shard: shard,
		}
	}

	t.Run("no resources", func(t *testing.T) {
		out := &bytes.Buffer{}
		printStatsSummary(out, []ClusterWithInfo{withResources(0, 0), withResources(1, 0)}, 2)
		assert.NotContains(t, out.String(), "NaN")
		assert.Equal(t, "SHARD  RESOURCES COUNT  % OF TOTAL  % OF AVG PER SHARD (0)\n"+
			"0      0                0%          0%\n"+
			"1      0                0%          0%\n", out.String())
	})

	t.Run("with resources", func(t *testing.T) {
		out := &bytes.Buffer{}
		printStatsSummary(out, []ClusterWithInfo{withResources(2, 300), withResources(0, 100)}, 3)
		assert.Equal(t, "SHARD  RESOURCES COUNT  % OF TOTAL  % OF AVG PER SHARD (133)\n"+
			"0      100              25%         75%\n"+
			"1      0                0%          0%\n"+
			"2      300              75%         225%\n", out.String())
	})

	t.Run("fewer resources than replicas", func(t *testing.T) {
		out := &bytes.Buffer{}
		printStatsSummary(out, []ClusterWithInfo{withResources(0, 1)}, 2)
		assert.Equal(t, "SHARD  RESOURCES COUNT  % OF TOTAL  % OF AVG PER SHARD (0)\n"+
			"0      1                100%        200%\n"+
			"1      0                0%          0%\n", out.String())
	})
}
