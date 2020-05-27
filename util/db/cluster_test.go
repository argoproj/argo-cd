package db

import (
	"context"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/client-go/kubernetes/fake"

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

func TestWatchClusters(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	timeout := time.Second * 5

	t.Run("NoClustersRegistered", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		addedClusters := make(chan *v1alpha1.Cluster)

		go func() {
			assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
				addedClusters <- cluster
			}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
				assert.Fail(t, "no cluster modifications expected")
			}, func(clusterServer string) {
				assert.Fail(t, "no cluster removals expected")
			}))
		}()

		select {
		case addedCluster := <-addedClusters:
			assert.Equal(t, addedCluster.Server, common.KubernetesInternalAPIServerAddr)
		case <-time.After(timeout):
			assert.Fail(t, "no cluster event within timeout")
		}
	})

	t.Run("ClusterAdded", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		addedClusters := make(chan *v1alpha1.Cluster)

		go func() {
			assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
				addedClusters <- cluster
			}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
				assert.Fail(t, "no cluster modifications expected")
			}, func(clusterServer string) {
				assert.Fail(t, "no cluster removals expected")
			}))
		}()

		_, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
		if !assert.NoError(t, err) {
			return
		}

		var addClusterServers []string
		// expect two cluster added events
		for i := 0; i < 2; i++ {
			select {
			case addedCluster := <-addedClusters:
				addClusterServers = append(addClusterServers, addedCluster.Server)
			case <-time.After(timeout):
				assert.Fail(t, "no cluster event within timeout")
			}
		}

		assert.ElementsMatch(t, []string{common.KubernetesInternalAPIServerAddr, "http://minikube"}, addClusterServers)
	})

	t.Run("ClusterMod", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		updatedClusters := make(chan *v1alpha1.Cluster)

		//Start with clean data
		db.DeleteCluster(ctx, "http://minikube")

		go func() {
			assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
				updatedClusters <- newCluster
			}, func(clusterServer string) {
				assert.Fail(t, "no cluster removals expected")
			}))
		}()

		cluster, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
		if !assert.NoError(t, err) {
			return
		}

		message := "sync successful"
		cluster.ConnectionState.Message = message
		cluster, err = db.UpdateCluster(ctx, cluster)
		if !assert.NoError(t, err) {
			return
		}

		select {
		case updatedCluster := <-updatedClusters:
			assert.Equal(t, message, updatedCluster.ConnectionState.Message)
		case <-time.After(timeout):
			assert.Fail(t, "no cluster event within timeout")
		}
	})

	t.Run("ClusterDel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		//Start with clean data
		db.DeleteCluster(ctx, "http://minikube")

		deletedClusters := make(chan string)

		go func() {
			assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
				assert.Fail(t, "no cluster update expected")
			}, func(clusterServer string) {
				deletedClusters <- clusterServer
			}))
		}()

		cluster, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: "http://minikube"})
		if !assert.NoError(t, err) {
			return
		}

		err = db.DeleteCluster(ctx, cluster.Server)
		if !assert.NoError(t, err) {
			return
		}

		var deletedClusterServers []string
		select {
		case deletedCluster := <-deletedClusters:
			deletedClusterServers = append(deletedClusterServers, deletedCluster)
		case <-time.After(timeout):
			assert.Fail(t, "no cluster event within timeout")
		}

		assert.ElementsMatch(t, []string{"http://minikube"}, deletedClusterServers)
	})
}
