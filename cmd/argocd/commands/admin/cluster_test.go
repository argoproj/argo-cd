package admin

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	fakeapps "github.com/argoproj/argo-cd/v3/pkg/client/clientset/versioned/fake"
	cacheutil "github.com/argoproj/argo-cd/v3/util/cache"
	"github.com/argoproj/argo-cd/v3/util/cache/appstate"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"
)

func Test_loadClusters(t *testing.T) {
	argoCDCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	argoCDSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-secret",
			Namespace: "argocd",
		},
		Data: map[string][]byte{
			"server.secretkey": []byte("test"),
		},
	}
	app := &v1alpha1.Application{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "argocd",
		},
		Spec: v1alpha1.ApplicationSpec{
			Project: "default",
			Destination: v1alpha1.ApplicationDestination{
				Server:    "https://kubernetes.default.svc",
				Namespace: "test",
			},
		},
	}
	ctx := t.Context()
	kubeClient := fake.NewClientset(argoCDCM, argoCDSecret)
	appClient := fakeapps.NewSimpleClientset(app)
	cacheSrc := func() (*appstate.Cache, error) {
		return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
	}
	clusters, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
	require.NoError(t, err)
	for i := range clusters {
		// This changes, nil it to avoid testing it.
		//nolint:staticcheck
		clusters[i].ConnectionState.ModifiedAt = nil
	}

	expected := []ClusterWithInfo{{
		Cluster: v1alpha1.Cluster{
			ID:     "",
			Server: "https://kubernetes.default.svc",
			Name:   "in-cluster",
			ConnectionState: v1alpha1.ConnectionState{
				Status: "Successful",
			},
			ServerVersion: ".",
			Shard:         ptr.To(int64(0)),
		},
		Namespaces: []string{"test"},
	}}
	assert.Equal(t, expected, clusters)
}
