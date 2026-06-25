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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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
	argoCDCmdCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cmd-params-cm",
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
	kubeClient := fake.NewClientset(argoCDCM, argoCDCmdCM, argoCDSecret)
	appClient := fakeapps.NewSimpleClientset(app)
	cacheSrc := func() (*appstate.Cache, error) {
		return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
	}
	clusters, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
	require.NoError(t, err)
	for i := range clusters {
		// This changes, nil it to avoid testing it.
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
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cm",
			Namespace: "argocd",
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "argocd",
			},
		},
		Data: map[string]string{},
	}
	argoCDCmdCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-cmd-params-cm",
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

	t.Run("argocd-cmd-params-cm is missing", func(t *testing.T) {
		ctx := t.Context()
		kubeClient := fake.NewClientset(argoCDCM, argoCDSecret)
		appClient := fakeapps.NewSimpleClientset(app)
		cacheSrc := func() (*appstate.Cache, error) {
			return appstate.NewCache(cacheutil.NewCache(cacheutil.NewInMemoryCache(time.Minute)), time.Minute), nil
		}
		_, err := loadClusters(ctx, kubeClient, appClient, 3, "", "argocd", false, cacheSrc, 0, "", "", "")
		require.NoError(t, err)
		require.True(t, strings.Contains(logOutput.String(), "Using filter function:  legacy"))
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
		require.True(t, strings.Contains(logOutput.String(), "Using filter function:  round-robin"))
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
		require.True(t, strings.Contains(logOutput.String(), "Using filter function:  legacy"))
	})
}
