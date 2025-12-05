package settings

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/common"
)

func TestClusterCache_TransformAndIndex(t *testing.T) {
	namespace := "argocd"

	// Create test cluster secrets
	clusterSecret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: namespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster1.example.com"),
			"name":   []byte("cluster-one"),
			"config": []byte(`{"tlsClientConfig":{"insecure":false}}`),
		},
	}

	clusterSecret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-2",
			Namespace: namespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster2.example.com"),
			"name":   []byte("cluster-two"),
			"config": []byte(`{"tlsClientConfig":{"insecure":true}}`),
		},
	}

	// Create fake clientset with pre-populated secrets
	clientset := fake.NewClientset(clusterSecret1, clusterSecret2)

	// Create cluster cache
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	clusterCache, err := NewClusterInformer(ctx, clientset, namespace)
	require.NoError(t, err)

	go clusterCache.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), clusterCache.HasSynced) {
		t.Fatal("Timed out waiting for informer cache to sync")
	}

	// Debug: List all clusters to see what's in the cache
	allClusters, err := clusterCache.ListClusters()
	require.NoError(t, err)
	t.Logf("Cache has %d clusters after sync", len(allClusters))
	for _, c := range allClusters {
		t.Logf("  - %s (%s)", c.Name, c.Server)
	}

	t.Run("GetClusterByURL returns pre-converted clusters", func(t *testing.T) {
		// Get cluster by URL - should return Cluster object, not Secret
		cluster, err := clusterCache.GetClusterByURL("https://cluster1.example.com")
		require.NoError(t, err)
		assert.Equal(t, "https://cluster1.example.com", cluster.Server)
		assert.Equal(t, "cluster-one", cluster.Name)
		assert.False(t, cluster.Config.TLSClientConfig.Insecure)

		// Verify it's a real Cluster object with parsed config
		assert.NotNil(t, cluster.Config)
	})

	t.Run("GetClusterServersByName returns server URLs", func(t *testing.T) {
		servers, err := clusterCache.GetClusterServersByName("cluster-two")
		require.NoError(t, err)
		require.Len(t, servers, 1)
		assert.Equal(t, "https://cluster2.example.com", servers[0])
	})

	t.Run("ListClusters returns all clusters", func(t *testing.T) {
		clusters, err := clusterCache.ListClusters()
		require.NoError(t, err)
		require.Len(t, clusters, 2)

		// Verify all are Cluster objects with parsed config
		for _, cluster := range clusters {
			assert.NotEmpty(t, cluster.Server)
			assert.NotEmpty(t, cluster.Name)
			// Config should be parsed, not raw JSON
			assert.NotNil(t, cluster.Config)
		}
	})

	t.Run("Returns copy to prevent modification", func(t *testing.T) {
		cluster1, err := clusterCache.GetClusterByURL("https://cluster1.example.com")
		require.NoError(t, err)

		// Modify the returned cluster
		cluster1.Name = "modified-name"

		// Get again - should still have original name
		cluster2, err := clusterCache.GetClusterByURL("https://cluster1.example.com")
		require.NoError(t, err)
		assert.Equal(t, "cluster-one", cluster2.Name)
		assert.NotEqual(t, cluster1.Name, cluster2.Name)
	})

	t.Run("NotFound error for missing cluster", func(t *testing.T) {
		_, err := clusterCache.GetClusterByURL("https://nonexistent.example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestClusterCache_DynamicUpdates(t *testing.T) {
	clientset := fake.NewClientset()
	namespace := "argocd"

	// Start with empty cache
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	clusterCache, err := NewClusterInformer(ctx, clientset, namespace)
	require.NoError(t, err)

	go clusterCache.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), clusterCache.HasSynced) {
		t.Fatal("Timed out waiting for informer cache to sync")
	}

	t.Run("Cache updates when new secret is added", func(t *testing.T) {
		// Initially empty
		clusters, err := clusterCache.ListClusters()
		require.NoError(t, err)
		assert.Len(t, clusters, 0)

		// Add a cluster secret
		clusterSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "new-cluster",
				Namespace: namespace,
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
				},
			},
			Data: map[string][]byte{
				"server": []byte("https://new.example.com"),
				"name":   []byte("new-cluster"),
			},
		}

		_, err = clientset.CoreV1().Secrets(namespace).Create(t.Context(), clusterSecret, metav1.CreateOptions{})
		require.NoError(t, err)

		time.Sleep(100 * time.Millisecond)

		// Should now see the cluster
		cluster, err := clusterCache.GetClusterByURL("https://new.example.com")
		require.NoError(t, err)
		assert.Equal(t, "new-cluster", cluster.Name)
	})
}

func TestClusterCache_ServerURLNormalization(t *testing.T) {
	namespace := "argocd"

	// Create cluster secret with trailing slash
	clusterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-1",
			Namespace: namespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster1.example.com/"),
			"name":   []byte("cluster-one"),
		},
	}

	// Create fake clientset with pre-populated secret
	clientset := fake.NewClientset(clusterSecret)

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	clusterCache, err := NewClusterInformer(ctx, clientset, namespace)
	require.NoError(t, err)

	go clusterCache.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), clusterCache.HasSynced) {
		t.Fatal("Timed out waiting for informer cache to sync")
	}

	t.Run("Can lookup with or without trailing slash", func(t *testing.T) {
		// Without trailing slash
		cluster1, err := clusterCache.GetClusterByURL("https://cluster1.example.com")
		require.NoError(t, err)
		assert.Equal(t, "cluster-one", cluster1.Name)

		// With trailing slash
		cluster2, err := clusterCache.GetClusterByURL("https://cluster1.example.com/")
		require.NoError(t, err)
		assert.Equal(t, "cluster-one", cluster2.Name)
	})
}

// BenchmarkClusterCache_vs_DirectConversion compares performance of using
// the cluster cache vs. direct SecretToCluster conversion
func BenchmarkClusterCache_vs_DirectConversion(b *testing.B) {
	namespace := "argocd"

	// Create a cluster secret
	clusterSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-cluster",
			Namespace: namespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://bench.example.com"),
			"name":   []byte("bench-cluster"),
			"config": []byte(`{"tlsClientConfig":{"insecure":false,"certData":"base64data","keyData":"base64data"}}`),
		},
	}
	// Create fake clientset with pre-populated secret
	clientset := fake.NewClientset(clusterSecret)

	ctx := b.Context()
	clusterCache, err := NewClusterInformer(ctx, clientset, namespace)
	require.NoError(b, err)

	go clusterCache.Run(ctx.Done())
	
	if !cache.WaitForCacheSync(ctx.Done(), clusterCache.HasSynced) {
		b.Fatal("Timed out waiting for informer cache to sync")
	}

	b.Run("ClusterInformer (pre-converted)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := clusterCache.GetClusterByURL("https://bench.example.com")
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Direct SecretToCluster (repeated conversion)", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := secretToCluster(clusterSecret)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
