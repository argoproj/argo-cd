package controllers

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientcmdv1 "k8s.io/client-go/tools/clientcmd/api/v1"
	clusterinventory "sigs.k8s.io/cluster-inventory-api/apis/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestClusterProfileReconciler(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clusterinventory.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))

	t.Run("Reconcile", func(t *testing.T) {
		t.Run("should create a secret when a new ClusterProfile is created", func(t *testing.T) {
			// Create a temporary file with the provider config
			file, err := os.CreateTemp(t.TempDir(), "providers.json")
			require.NoError(t, err)
			defer os.Remove(file.Name())
			_, err = file.WriteString(`{
				"providers": [
					{
						"name": "secretreader",
						"execConfig": {
							"apiVersion": "client.authentication.k8s.io/v1",
							"command": "./bin/secretreader-plugin"
						}
					}
				]
			}`)
			require.NoError(t, err)

			clusterProfile := &clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Status: clusterinventory.ClusterProfileStatus{
					AccessProviders: []clusterinventory.AccessProvider{
						{
							Name: "secretreader",
							Cluster: clientcmdv1.Cluster{
								Server: "https://test-cluster.example.com",
							},
						},
					},
				},
			}
			r := &ClusterProfileReconciler{
				Client:                      fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterProfile).Build(),
				Log:                         logr.Discard(),
				Scheme:                      scheme,
				Namespace:                   "argocd",
				ClusterProfileProvidersFile: file.Name(),
			}
			require.NoError(t, r.loadClusterProfileProviders())
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}

			_, err = r.Reconcile(context.Background(), req)

			require.NoError(t, err)
			var secret corev1.Secret
			err = r.Get(context.Background(), types.NamespacedName{Name: "cluster-test-cluster", Namespace: "argocd"}, &secret)
			require.NoError(t, err)
			assert.Equal(t, "cluster-test-cluster", secret.Name)
			assert.Equal(t, "argocd", secret.Namespace)
			assert.Equal(t, "cluster", secret.Labels["argocd.argoproj.io/secret-type"])
			assert.Equal(t, "default-test-cluster", secret.Labels["argocd.argoproj.io/cluster-profile-origin"])
			assert.Equal(t, "test-cluster", secret.StringData["name"])
			assert.Equal(t, "https://test-cluster.example.com", secret.StringData["server"])

			var configMap map[string]any
			require.NoError(t, json.Unmarshal([]byte(secret.StringData["config"]), &configMap))
			execProviderConfig := configMap["execProviderConfig"].(map[string]any)
			assert.Equal(t, "client.authentication.k8s.io/v1", execProviderConfig["apiVersion"])
			assert.Equal(t, "./bin/secretreader-plugin", execProviderConfig["command"])
		})

		t.Run("should delete the secret when the ClusterProfile is deleted", func(t *testing.T) {
			now := metav1.NewTime(time.Now())
			clusterProfile := &clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{clusterProfileFinalizer},
				},
			}
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-test-cluster",
					Namespace: "argocd",
				},
			}
			r := &ClusterProfileReconciler{
				Client:    fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterProfile, secret).Build(),
				Log:       logr.Discard(),
				Scheme:    scheme,
				Namespace: "argocd",
			}
			req := reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "test-cluster",
					Namespace: "default",
				},
			}

			_, err := r.Reconcile(context.Background(), req)

			require.NoError(t, err)
			var deletedSecret corev1.Secret
			err = r.Get(context.Background(), types.NamespacedName{Name: "cluster-test-cluster", Namespace: "argocd"}, &deletedSecret)
			assert.Error(t, err)
		})
	})
}
