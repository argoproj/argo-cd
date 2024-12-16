package utils

import (
	"testing"

	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
			"config": []byte("{\"username\":\"foo\", \"disableCompression\":true}"),
		},
	}
	cluster, err := secretToCluster(secret)
	require.NoError(t, err)
	assert.Equal(t, argoappv1.Cluster{
		Name:   "test",
		Server: "http://mycluster",
		Config: argoappv1.ClusterConfig{
			Username:           "foo",
			DisableCompression: true,
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
