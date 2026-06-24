package admin

import (
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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_loadClustersSkipsApplicationWithRemovedCluster(t *testing.T) {
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
