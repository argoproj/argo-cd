package utils

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

const (
	fakeNamespace = "fake-ns"
)

// From Argo CD util/db/cluster_test.go
func Test_secretToCluster(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster"),
			"config": []byte("{\"username\":\"foo\"}"),
		},
	}
	cluster, err := secretToCluster(secret)
	require.NoError(t, err)
	assert.Equal(t, argoappv1.Cluster{
		Name:   "test",
		Server: "http://mycluster",
		Config: argoappv1.ClusterConfig{
			Username: "foo",
		},
	}, *cluster)
}

// From Argo CD util/db/cluster_test.go
func Test_secretToCluster_NoConfig(t *testing.T) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
		},
		Data: map[string][]byte{
			"name":   []byte("test"),
			"server": []byte("http://mycluster"),
		},
	}
	cluster, err := secretToCluster(secret)
	require.NoError(t, err)
	assert.Equal(t, argoappv1.Cluster{
		Name:   "test",
		Server: "http://mycluster",
	}, *cluster)
}

func createClusterSecret(secretName string, clusterName string, clusterServer string) *corev1.Secret {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				ArgoCDSecretTypeLabel: ArgoCDSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"name":   []byte(clusterName),
			"server": []byte(clusterServer),
			"config": []byte("{\"username\":\"foo\",\"password\":\"foo\"}"),
		},
	}

	return secret
}

// From util/argo/argo_test.go
// (ported to use kubeclientset)
func TestValidateDestination(t *testing.T) {
	t.Run("Validate destination with server url", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Server:    "https://127.0.0.1:6443",
			Namespace: "default",
		}

		appCond := ValidateDestination(context.Background(), &dest, nil, fakeNamespace)
		require.NoError(t, appCond)
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Validate destination with server name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		secret := createClusterSecret("my-secret", "minikube", "https://127.0.0.1:6443")
		objects := []runtime.Object{}
		objects = append(objects, secret)
		kubeclientset := fake.NewSimpleClientset(objects...)

		appCond := ValidateDestination(context.Background(), &dest, kubeclientset, fakeNamespace)
		require.NoError(t, appCond)
		assert.Equal(t, "https://127.0.0.1:6443", dest.Server)
		assert.True(t, dest.IsServerInferred())
	})

	t.Run("Error when having both server url and name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Server:    "https://127.0.0.1:6443",
			Name:      "minikube",
			Namespace: "default",
		}

		err := ValidateDestination(context.Background(), &dest, nil, fakeNamespace)
		assert.Equal(t, "application destination can't have both name and server defined: minikube https://127.0.0.1:6443", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("List clusters fails", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}
		kubeclientset := fake.NewSimpleClientset()

		kubeclientset.PrependReactor("list", "*", func(action kubetesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, fmt.Errorf("an error occurred")
		})

		err := ValidateDestination(context.Background(), &dest, kubeclientset, fakeNamespace)
		assert.Equal(t, "unable to find destination server: an error occurred", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Destination cluster does not exist", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "minikube",
		}

		secret := createClusterSecret("dind", "dind", "https://127.0.0.1:6443")
		objects := []runtime.Object{}
		objects = append(objects, secret)
		kubeclientset := fake.NewSimpleClientset(objects...)

		err := ValidateDestination(context.Background(), &dest, kubeclientset, fakeNamespace)
		assert.Equal(t, "unable to find destination server: there are no clusters with this name: minikube", err.Error())
		assert.False(t, dest.IsServerInferred())
	})

	t.Run("Validate too many clusters with the same name", func(t *testing.T) {
		dest := argoappv1.ApplicationDestination{
			Name: "dind",
		}

		secret := createClusterSecret("dind", "dind", "https://127.0.0.1:2443")
		secret2 := createClusterSecret("dind2", "dind", "https://127.0.0.1:8443")

		objects := []runtime.Object{}
		objects = append(objects, secret, secret2)
		kubeclientset := fake.NewSimpleClientset(objects...)

		err := ValidateDestination(context.Background(), &dest, kubeclientset, fakeNamespace)
		assert.Equal(t, "unable to find destination server: there are 2 clusters with the same name: [https://127.0.0.1:2443 https://127.0.0.1:8443]", err.Error())
		assert.False(t, dest.IsServerInferred())
	})
}
