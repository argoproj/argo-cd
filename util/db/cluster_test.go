package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	fakeNamespace = "fake-ns"
)

func Test_serverToSecretName(t *testing.T) {
	name, err := serverToSecretName("http://foo")
	assert.NoError(t, err)
	assert.Equal(t, "cluster-foo-752281925", name)
}

func TestUpdateCluster(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset(&v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mycluster",
			Namespace: fakeNamespace,
			Labels: map[string]string{
				common.LabelKeySecretType: common.LabelValueSecretTypeCluster,
			},
		},
		Data: map[string][]byte{
			"server": []byte("http://mycluster"),
			"config": []byte("{}"),
		},
	})
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	requestedAt := metav1.Now()
	_, err := db.UpdateCluster(context.Background(), &v1alpha1.Cluster{
		Name:               "test",
		Server:             "http://mycluster",
		RefreshRequestedAt: &requestedAt,
	})
	if !assert.NoError(t, err) {
		return
	}

	secret, err := kubeclientset.CoreV1().Secrets(fakeNamespace).Get("mycluster", metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, secret.Annotations[common.AnnotationKeyRefresh], requestedAt.Format(time.RFC3339))
}

func TestWatchClusters_CreateRemoveCluster(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)

			_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
				Server: "https://minikube",
				Name:   "minikube",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, new.Server, "https://minikube")
			assert.Equal(t, new.Name, "minikube")

			assert.NoError(t, db.DeleteCluster(context.Background(), "https://minikube"))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, new)
			assert.Equal(t, old.Server, "https://minikube")
		},
	})
}

func TestWatchClusters_LocalClusterModifications(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	runWatchTest(t, db, []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster){
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Nil(t, old)
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)

			_, err := db.CreateCluster(context.Background(), &v1alpha1.Cluster{
				Server: common.KubernetesInternalAPIServerAddr,
				Name:   "some name",
			})
			assert.NoError(t, err)
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.NotNil(t, old)
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)
			assert.Equal(t, new.Name, "some name")

			assert.NoError(t, db.DeleteCluster(context.Background(), common.KubernetesInternalAPIServerAddr))
		},
		func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
			assert.Equal(t, new.Server, common.KubernetesInternalAPIServerAddr)
			assert.Equal(t, new.Name, "in-cluster")
		},
	})
}

func runWatchTest(t *testing.T, db ArgoDB, actions []func(old *v1alpha1.Cluster, new *v1alpha1.Cluster)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	timeout := time.Second * 5

	allDone := make(chan bool, 1)

	doNext := func(old *v1alpha1.Cluster, new *v1alpha1.Cluster) {
		if len(actions) == 0 {
			assert.Fail(t, "Unexpected event")
		}
		next := actions[0]
		next(old, new)
		if t.Failed() {
			allDone <- true
		}
		if len(actions) == 1 {
			allDone <- true
		} else {
			actions = actions[1:]
		}
	}

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			doNext(nil, cluster)
		}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
			doNext(oldCluster, newCluster)
		}, func(clusterServer string) {
			doNext(&v1alpha1.Cluster{Server: clusterServer}, nil)
		}))
	}()

	select {
	case <-allDone:
	case <-time.After(timeout):
		assert.Fail(t, "Failed due to timeout")
	}

}
