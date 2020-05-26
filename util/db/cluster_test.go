package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	fakeClusterSecretName = "fake-secret"
	fakeNamespace         = "fake-ns"
)

func Test_serverToSecretName(t *testing.T) {
	name, err := serverToSecretName("http://foo")
	assert.NoError(t, err)
	assert.Equal(t, "cluster-foo-752281925", name)
}

// TestClusterInformer verifies the informer will get updated with a new cluster
func TestClusterInformer(t *testing.T) {
	cm := fakeSecret()
	kubeclientset := fake.NewSimpleClientset(cm)
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, "default")
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		db.WatchClusters(ctx, handleAddEvent, handleModEvent, handleDeleteEvent)
	}()

	for i := 1; i <= 20; i++ {
		kubeclientset.CoreV1().Secrets(fakeNamespace).Create(cm)
		time.Sleep(200 * time.Millisecond)
	}
}

func handleAddEvent(cluster *appv1.Cluster) {
	fmt.Print(cluster.Name)
}
func handleModEvent(oldCluster *appv1.Cluster, newCluster *appv1.Cluster) {
	fmt.Print(oldCluster.Name)
}
func handleDeleteEvent(clusterServer string) {
	fmt.Print(clusterServer)
}

func fakeSecret() *apiv1.Secret {
	secret := apiv1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fakeClusterSecretName,
			Namespace: fakeNamespace,
			Labels: map[string]string{
				"argocd.argoproj.io/secret-type": "cluster",
			},
			Annotations: map[string]string{
				"managed-by": "argocd.argoproj.io",
			},
		},
		Data: map[string][]byte{
			"name":   []byte("fakeClusterSecretName"),
			"server": []byte("test"),
		},
	}
	return &secret
}
