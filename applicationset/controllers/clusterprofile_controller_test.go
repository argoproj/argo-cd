package controllers

import (
	"context"
	"encoding/json"
	"errors"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type failingDeleteClient struct {
	client.Client
}

func (c *failingDeleteClient) Delete(_ context.Context, _ client.Object, _ ...client.DeleteOption) error {
	return errors.New("unable to delete secret")
}

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
						"name": "auth-provider",
						"execConfig": {
							"apiVersion": "client.authentication.k8s.io/v1",
							"command": "./bin/auth-provider-plugin"
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
							Name: "auth-provider",
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
			assert.Equal(t, "./bin/auth-provider-plugin", execProviderConfig["command"])
		})

		t.Run("should update the secret when the ClusterProfile is updated", func(t *testing.T) {
			// Create a temporary file with the provider config
			file, err := os.CreateTemp(t.TempDir(), "providers.json")
			require.NoError(t, err)
			defer os.Remove(file.Name())
			_, err = file.WriteString(`{
				"providers": [
					{
						"name": "auth-provider",
						"execConfig": {
							"apiVersion": "client.authentication.k8s.io/v1",
							"command": "./bin/auth-provider-plugin"
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
							Name: "auth-provider",
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

			// Update the ClusterProfile
			updatedClusterProfile := &clusterinventory.ClusterProfile{}
			err = r.Get(context.Background(), req.NamespacedName, updatedClusterProfile)
			require.NoError(t, err)
			updatedClusterProfile.Status.AccessProviders[0].Cluster.Server = "https://updated-cluster.example.com"
			err = r.Update(context.Background(), updatedClusterProfile)
			require.NoError(t, err)

			_, err = r.Reconcile(context.Background(), req)
			require.NoError(t, err)

			var secret corev1.Secret
			err = r.Get(context.Background(), types.NamespacedName{Name: "cluster-test-cluster", Namespace: "argocd"}, &secret)
			require.NoError(t, err)
			assert.Equal(t, "https://updated-cluster.example.com", secret.StringData["server"])
		})

		t.Run("should not return an error if the ClusterProfile is not found", func(t *testing.T) {
			r := &ClusterProfileReconciler{
				Client:    fake.NewClientBuilder().WithScheme(scheme).Build(),
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

			res, err := r.Reconcile(context.Background(), req)

			require.NoError(t, err)
			assert.Equal(t, reconcile.Result{}, res)
		})

		t.Run("should add a finalizer if it is not present", func(t *testing.T) {
			// Create a temporary file with the provider config
			file, err := os.CreateTemp(t.TempDir(), "providers.json")
			require.NoError(t, err)
			defer os.Remove(file.Name())
			_, err = file.WriteString(`{
				"providers": [
					{
						"name": "auth-provider",
						"execConfig": {
							"apiVersion": "client.authentication.k8s.io/v1",
							"command": "./bin/auth-provider-plugin"
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
							Name: "auth-provider",
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
			var updatedClusterProfile clusterinventory.ClusterProfile
			err = r.Get(context.Background(), req.NamespacedName, &updatedClusterProfile)
			require.NoError(t, err)
			assert.Contains(t, updatedClusterProfile.Finalizers, clusterProfileFinalizer)
		})

		t.Run("should return an error if AccessProviders is empty", func(t *testing.T) {
			clusterProfile := &clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-cluster",
					Namespace: "default",
				},
				Status: clusterinventory.ClusterProfileStatus{
					AccessProviders: []clusterinventory.AccessProvider{},
				},
			}
			r := &ClusterProfileReconciler{
				Client:    fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterProfile).Build(),
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

			require.Error(t, err)
		})

		t.Run("should remove the finalizer if the secret does not exist on prune", func(t *testing.T) {
			now := metav1.NewTime(time.Now())
			clusterProfile := &clusterinventory.ClusterProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-cluster",
					Namespace:         "default",
					DeletionTimestamp: &now,
					Finalizers:        []string{clusterProfileFinalizer},
				},
			}
			r := &ClusterProfileReconciler{
				Client:    fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterProfile).Build(),
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

			// The reconcile should not return an error, and the garbage collector should delete the resource.
			require.NoError(t, err)
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

		t.Run("should not remove the finalizer if secret deletion fails", func(t *testing.T) {
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
				Client: &failingDeleteClient{
					Client: fake.NewClientBuilder().WithScheme(scheme).WithObjects(clusterProfile, secret).Build(),
				},
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

			// The reconcile should return an error because the secret deletion fails.
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unable to delete secret")
		})
	})
}

func TestLoadClusterProfileProviders(t *testing.T) {
	t.Run("should not return an error if the providers file is not specified", func(t *testing.T) {
		r := &ClusterProfileReconciler{
			Log: logr.Discard(),
		}
		err := r.loadClusterProfileProviders()
		assert.NoError(t, err)
	})

	t.Run("should return an error if the providers file does not exist", func(t *testing.T) {
		r := &ClusterProfileReconciler{
			Log:                         logr.Discard(),
			ClusterProfileProvidersFile: "non-existent-file",
		}
		err := r.loadClusterProfileProviders()
		assert.Error(t, err)
	})
}
