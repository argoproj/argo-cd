//go:build race

package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/common"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

func TestClusterInformer_ConcurrentAccess(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster1.example.com"),
			"name":   []byte("cluster1"),
			"config": []byte(`{"bearerToken":"token1"}`),
		},
	}

	clientset := fake.NewSimpleClientset(secret1)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cluster, err := informer.GetClusterByURL("https://cluster1.example.com")
			assert.NoError(t, err)
			assert.NotNil(t, cluster)
			// Modifying returned cluster should not affect others due to DeepCopy
			cluster.Name = "modified"
		}()
	}
	wg.Wait()

	cluster, err := informer.GetClusterByURL("https://cluster1.example.com")
	require.NoError(t, err)
	assert.Equal(t, "cluster1", cluster.Name)
}

func TestClusterInformer_TransformErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	badSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://bad.example.com"),
			"name":   []byte("bad-cluster"),
			"config": []byte(`{invalid json}`),
		},
	}

	clientset := fake.NewSimpleClientset(badSecret)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	// GetClusterByURL should return not found since transform failed
	_, err = informer.GetClusterByURL("https://bad.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	// ListClusters should return an error because the cache contains a secret and not a cluster
	_, err = informer.ListClusters()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster cache contains unexpected type")
}

func TestClusterInformer_TransformErrors_MixedSecrets(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	// One good secret and one bad secret
	goodSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "good-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://good.example.com"),
			"name":   []byte("good-cluster"),
			"config": []byte(`{"bearerToken":"token"}`),
		},
	}

	badSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bad-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://bad.example.com"),
			"name":   []byte("bad-cluster"),
			"config": []byte(`{invalid json}`),
		},
	}

	clientset := fake.NewSimpleClientset(goodSecret, badSecret)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	// GetClusterByURL should still work for the good cluster
	cluster, err := informer.GetClusterByURL("https://good.example.com")
	require.NoError(t, err)
	assert.Equal(t, "good-cluster", cluster.Name)

	// But ListClusters should fail because there's a bad secret in the cache
	_, err = informer.ListClusters()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cluster cache contains unexpected type")
}

func TestClusterInformer_DynamicUpdates(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster1.example.com"),
			"name":   []byte("cluster1"),
			"config": []byte(`{"bearerToken":"token1"}`),
		},
	}

	clientset := fake.NewSimpleClientset(secret1)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	clusters, err := informer.ListClusters()
	require.NoError(t, err)
	assert.Len(t, clusters, 1)

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster2",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster2.example.com"),
			"name":   []byte("cluster2"),
			"config": []byte(`{"bearerToken":"token2"}`),
		},
	}

	_, err = clientset.CoreV1().Secrets("argocd").Create(t.Context(), secret2, metav1.CreateOptions{})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	cluster2, err := informer.GetClusterByURL("https://cluster2.example.com")
	require.NoError(t, err)
	assert.Equal(t, "cluster2", cluster2.Name)

	clusters, err = informer.ListClusters()
	require.NoError(t, err)
	assert.Len(t, clusters, 2)
}

func TestClusterInformer_URLNormalization(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster.example.com/"),
			"name":   []byte("cluster1"),
		},
	}

	clientset := fake.NewSimpleClientset(secret)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	testCases := []string{
		"https://cluster.example.com",
		"https://cluster.example.com/",
	}

	for _, url := range testCases {
		cluster, err := informer.GetClusterByURL(url)
		require.NoError(t, err, "Failed for URL: %s", url)
		assert.Equal(t, "cluster1", cluster.Name)
		assert.Equal(t, "https://cluster.example.com", cluster.Server)
	}
}

func TestClusterInformer_GetClusterServersByName(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secrets := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prod-cluster-1",
				Namespace: "argocd",
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
				},
			},
			Data: map[string][]byte{
				"server":  []byte("https://prod1.example.com"),
				"name":    []byte("production"),
				"project": []byte("team-a"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "prod-cluster-2",
				Namespace: "argocd",
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
				},
			},
			Data: map[string][]byte{
				"server":  []byte("https://prod2.example.com"),
				"name":    []byte("production"),
				"project": []byte("team-b"),
			},
		},
	}

	clientset := fake.NewSimpleClientset(secrets...)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	servers, err := informer.GetClusterServersByName("production")
	require.NoError(t, err)
	assert.Len(t, servers, 2)
	assert.Contains(t, servers, "https://prod1.example.com")
	assert.Contains(t, servers, "https://prod2.example.com")
}

func TestClusterInformer_RaceCondition(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	var secrets []*corev1.Secret
	for i := 0; i < 10; i++ {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("cluster-%d", i),
				Namespace: "argocd",
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
				},
			},
			Data: map[string][]byte{
				"server": []byte(fmt.Sprintf("https://cluster%d.example.com", i)),
				"name":   []byte(fmt.Sprintf("cluster-%d", i)),
				"config": []byte(`{"bearerToken":"token"}`),
			},
		}
		secrets = append(secrets, secret)
	}

	clientset := fake.NewSimpleClientset()
	for _, secret := range secrets {
		_, err := clientset.CoreV1().Secrets("argocd").Create(t.Context(), secret, metav1.CreateOptions{})
		require.NoError(t, err)
	}

	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	var wg sync.WaitGroup
	var readErrors, updateErrors atomic.Int64

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				clusterID := j % 10
				url := fmt.Sprintf("https://cluster%d.example.com", clusterID)

				cluster, err := informer.GetClusterByURL(url)
				if err != nil {
					readErrors.Add(1)
					continue
				}

				/*
					expectedName := fmt.Sprintf("cluster-%d", clusterID)
					if cluster.Name != expectedName {
						t.Errorf("Data corruption: expected name %s, got %s", expectedName, cluster.Name)
					}
				*/

				// Name may be original "cluster-X" or updated "updated-X-Y" depending on timing.
				// Just verify we get a non-empty name (no corruption/partial reads).
				if cluster.Name == "" {
					t.Errorf("Got empty cluster name for URL %s", url)
				}

				// Modifying the returned cluster should not affect the cache (DeepCopy isolation)
				cluster.Name = fmt.Sprintf("modified-%d-%d", id, j)
				cluster.Server = "https://modified.example.com"
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				secret := secrets[id%10].DeepCopy()
				secret.Data["name"] = []byte(fmt.Sprintf("updated-%d-%d", id, j))

				_, err := clientset.CoreV1().Secrets("argocd").Update(t.Context(), secret, metav1.UpdateOptions{})
				if err != nil {
					updateErrors.Add(1)
				}
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				clusters, err := informer.ListClusters()
				if err != nil {
					readErrors.Add(1)
					continue
				}

				for _, cluster := range clusters {
					if cluster == nil {
						t.Error("Got nil cluster from list")
					}
					cluster.Name = "modified-from-list"
				}
				time.Sleep(5 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	finalClusters, err := informer.ListClusters()
	require.NoError(t, err)
	assert.Len(t, finalClusters, 10)

	t.Logf("Read errors: %d, Update errors: %d", readErrors.Load(), updateErrors.Load())
}

func TestClusterInformer_DeepCopyIsolation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server":     []byte("https://test.example.com"),
			"name":       []byte("test-cluster"),
			"config":     []byte(`{"bearerToken":"token"}`),
			"namespaces": []byte("ns1,ns2,ns3"),
		},
	}

	clientset := fake.NewSimpleClientset(secret)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	cluster1, err := informer.GetClusterByURL("https://test.example.com")
	require.NoError(t, err)

	cluster2, err := informer.GetClusterByURL("https://test.example.com")
	require.NoError(t, err)

	assert.NotSame(t, cluster1, cluster2)

	cluster1.Name = "modified"
	cluster1.Namespaces = []string{"modified-ns"}
	cluster1.Config.BearerToken = "modified-token"

	assert.Equal(t, "test-cluster", cluster2.Name)
	assert.Equal(t, []string{"ns1", "ns2", "ns3"}, cluster2.Namespaces)
	assert.Equal(t, "token", cluster2.Config.BearerToken)

	cluster3, err := informer.GetClusterByURL("https://test.example.com")
	require.NoError(t, err)
	assert.Equal(t, "test-cluster", cluster3.Name)
	assert.Equal(t, []string{"ns1", "ns2", "ns3"}, cluster3.Namespaces)
}

func TestClusterInformer_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		secrets  []runtime.Object
		testFunc func(t *testing.T, informer *ClusterInformer)
	}{
		{
			name:    "Empty namespace - no clusters",
			secrets: []runtime.Object{},
			testFunc: func(t *testing.T, informer *ClusterInformer) {
				clusters, err := informer.ListClusters()
				require.NoError(t, err)
				assert.Empty(t, clusters)

				_, err = informer.GetClusterByURL("https://nonexistent.example.com")
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not found")
			},
		},
		{
			name: "Cluster with empty name",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-no-name",
						Namespace: "argocd",
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
						},
					},
					Data: map[string][]byte{
						"server": []byte("https://noname.example.com"),
						"name":   []byte(""),
						"config": []byte(`{}`),
					},
				},
			},
			testFunc: func(t *testing.T, informer *ClusterInformer) {
				cluster, err := informer.GetClusterByURL("https://noname.example.com")
				require.NoError(t, err)
				assert.Equal(t, "", cluster.Name)

				servers, err := informer.GetClusterServersByName("")
				require.NoError(t, err)
				assert.Empty(t, servers)
			},
		},
		{
			name: "Cluster with special characters in URL",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "special-cluster",
						Namespace: "argocd",
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
						},
					},
					Data: map[string][]byte{
						"server": []byte("https://cluster.example.com:8443/path"),
						"name":   []byte("special"),
						"config": []byte(`{}`),
					},
				},
			},
			testFunc: func(t *testing.T, informer *ClusterInformer) {
				cluster, err := informer.GetClusterByURL("https://cluster.example.com:8443/path/")
				require.NoError(t, err)
				assert.Equal(t, "special", cluster.Name)
				assert.Equal(t, "https://cluster.example.com:8443/path", cluster.Server)
			},
		},
		{
			name: "Multiple clusters with same URL",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "argocd",
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
						},
					},
					Data: map[string][]byte{
						"server": []byte("https://duplicate.example.com"),
						"name":   []byte("first"),
						"config": []byte(`{}`),
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster2",
						Namespace: "argocd",
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
						},
					},
					Data: map[string][]byte{
						"server": []byte("https://duplicate.example.com/"),
						"name":   []byte("second"),
						"config": []byte(`{}`),
					},
				},
			},
			testFunc: func(t *testing.T, informer *ClusterInformer) {
				cluster, err := informer.GetClusterByURL("https://duplicate.example.com")
				require.NoError(t, err)
				assert.NotNil(t, cluster)
			},
		},
		{
			name: "Cluster with very long namespace list",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "many-namespaces",
						Namespace: "argocd",
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
						},
					},
					Data: map[string][]byte{
						"server": []byte("https://many-ns.example.com"),
						"name":   []byte("many-ns"),
						"namespaces": func() []byte {
							ns := ""
							for i := 0; i < 100; i++ {
								if i > 0 {
									ns += ","
								}
								ns += fmt.Sprintf("namespace-%d", i)
							}
							return []byte(ns)
						}(),
						"config": []byte(`{}`),
					},
				},
			},
			testFunc: func(t *testing.T, informer *ClusterInformer) {
				cluster, err := informer.GetClusterByURL("https://many-ns.example.com")
				require.NoError(t, err)
				assert.Len(t, cluster.Namespaces, 100)
				assert.Equal(t, "namespace-0", cluster.Namespaces[0])
				assert.Equal(t, "namespace-99", cluster.Namespaces[99])
			},
		},
		{
			name: "Cluster with annotations and labels",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "annotated-cluster",
						Namespace: "argocd",
						Labels: map[string]string{
							common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
							"custom-label":            "custom-value",
							"team":                    "platform",
						},
						Annotations: map[string]string{
							"description":                      "Production cluster",
							"owner":                            "platform-team",
							common.AnnotationKeyManagedBy:      "argocd", // system annotation - should be filtered
							appv1.AnnotationKeyRefresh:         time.Now().Format(time.RFC3339),
							corev1.LastAppliedConfigAnnotation: "should-be-filtered",
						},
					},
					Data: map[string][]byte{
						"server": []byte("https://annotated.example.com"),
						"name":   []byte("annotated"),
						"config": []byte(`{}`),
						"shard":  []byte("5"),
					},
				},
			},
			testFunc: func(t *testing.T, informer *ClusterInformer) {
				cluster, err := informer.GetClusterByURL("https://annotated.example.com")
				require.NoError(t, err)

				assert.Equal(t, "custom-value", cluster.Labels["custom-label"])
				assert.Equal(t, "platform", cluster.Labels["team"])
				_, hasSystemLabel := cluster.Labels[common.LabelKeySecretType]
				assert.False(t, hasSystemLabel)

				assert.Equal(t, "Production cluster", cluster.Annotations["description"])
				assert.Equal(t, "platform-team", cluster.Annotations["owner"])
				// System annotations should be filtered out
				_, hasManagedBy := cluster.Annotations[common.AnnotationKeyManagedBy]
				assert.False(t, hasManagedBy, "managed-by is a system annotation and should be filtered")
				_, hasLastApplied := cluster.Annotations[corev1.LastAppliedConfigAnnotation]
				assert.False(t, hasLastApplied, "LastAppliedConfigAnnotation should be filtered")

				assert.NotNil(t, cluster.RefreshRequestedAt)
				assert.NotNil(t, cluster.Shard)
				assert.Equal(t, int64(5), *cluster.Shard)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()

			clientset := fake.NewSimpleClientset(tt.secrets...)
			informer, err := NewClusterInformer(clientset, "argocd")
			require.NoError(t, err)

			go informer.Run(ctx.Done())
			cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

			tt.testFunc(t, informer)
		})
	}
}

func TestClusterInformer_SecretDeletion(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	secret1 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster1",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster1.example.com"),
			"name":   []byte("cluster1"),
		},
	}

	secret2 := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster2",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("https://cluster2.example.com"),
			"name":   []byte("cluster2"),
		},
	}

	clientset := fake.NewSimpleClientset(secret1, secret2)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	clusters, err := informer.ListClusters()
	require.NoError(t, err)
	assert.Len(t, clusters, 2)

	err = clientset.CoreV1().Secrets("argocd").Delete(t.Context(), "cluster1", metav1.DeleteOptions{})
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)

	_, err = informer.GetClusterByURL("https://cluster1.example.com")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")

	cluster2, err := informer.GetClusterByURL("https://cluster2.example.com")
	require.NoError(t, err)
	assert.Equal(t, "cluster2", cluster2.Name)

	clusters, err = informer.ListClusters()
	require.NoError(t, err)
	assert.Len(t, clusters, 1)
}

func TestClusterInformer_ComplexConfig(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	complexConfig := appv1.ClusterConfig{
		Username:    "admin",
		Password:    "password123",
		BearerToken: "bearer-token",
		TLSClientConfig: appv1.TLSClientConfig{
			Insecure:   true,
			ServerName: "cluster.internal",
			CertData:   []byte("cert-data"),
			KeyData:    []byte("key-data"),
			CAData:     []byte("ca-data"),
		},
		AWSAuthConfig: &appv1.AWSAuthConfig{
			ClusterName: "eks-cluster",
			RoleARN:     "arn:aws:iam::123456789:role/eks-role",
			Profile:     "default",
		},
		ExecProviderConfig: &appv1.ExecProviderConfig{
			Command:     "kubectl",
			Args:        []string{"version"},
			Env:         map[string]string{"KUBECONFIG": "/tmp/config"},
			APIVersion:  "client.authentication.k8s.io/v1beta1",
			InstallHint: "Install kubectl",
		},
	}

	configJSON, err := json.Marshal(complexConfig)
	require.NoError(t, err)

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "complex-cluster",
			Namespace: "argocd",
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server":           []byte("https://complex.example.com"),
			"name":             []byte("complex"),
			"config":           configJSON,
			"clusterResources": []byte("true"),
			"project":          []byte("default"),
		},
	}

	clientset := fake.NewSimpleClientset(secret)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(t, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	cluster, err := informer.GetClusterByURL("https://complex.example.com")
	require.NoError(t, err)

	assert.Equal(t, "admin", cluster.Config.Username)
	assert.Equal(t, "password123", cluster.Config.Password)
	assert.Equal(t, "bearer-token", cluster.Config.BearerToken)
	assert.True(t, cluster.Config.TLSClientConfig.Insecure)
	assert.Equal(t, "cluster.internal", cluster.Config.TLSClientConfig.ServerName)

	assert.NotNil(t, cluster.Config.AWSAuthConfig)
	assert.Equal(t, "eks-cluster", cluster.Config.AWSAuthConfig.ClusterName)
	assert.Equal(t, "arn:aws:iam::123456789:role/eks-role", cluster.Config.AWSAuthConfig.RoleARN)

	assert.NotNil(t, cluster.Config.ExecProviderConfig)
	assert.Equal(t, "kubectl", cluster.Config.ExecProviderConfig.Command)
	assert.Equal(t, []string{"version"}, cluster.Config.ExecProviderConfig.Args)

	assert.True(t, cluster.ClusterResources)
	assert.Equal(t, "default", cluster.Project)
}

func BenchmarkClusterInformer_GetClusterByURL(b *testing.B) {
	ctx, cancel := context.WithCancel(b.Context())
	defer cancel()

	var secrets []runtime.Object
	for i := 0; i < 1000; i++ {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("cluster-%d", i),
				Namespace: "argocd",
				Labels: map[string]string{
					common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
				},
			},
			Data: map[string][]byte{
				"server": []byte(fmt.Sprintf("https://cluster%d.example.com", i)),
				"name":   []byte(fmt.Sprintf("cluster-%d", i)),
				"config": []byte(`{"bearerToken":"token"}`),
			},
		}
		secrets = append(secrets, secret)
	}

	clientset := fake.NewSimpleClientset(secrets...)
	informer, err := NewClusterInformer(clientset, "argocd")
	require.NoError(b, err)

	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			url := fmt.Sprintf("https://cluster%d.example.com", i%1000)
			cluster, err := informer.GetClusterByURL(url)
			if err != nil {
				b.Fatal(err)
			}
			if cluster == nil {
				b.Fatal("cluster should not be nil")
			}
			i++
		}
	})
}
