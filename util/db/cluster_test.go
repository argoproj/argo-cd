package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestWatchClustersNoClustersRegistered(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)
	timeout := time.Second * 5

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
}

func TestWatchClusters(t *testing.T) {
	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addedClusters := make(chan *v1alpha1.Cluster, 5)
	updatedClusters := make(chan *v1alpha1.Cluster, 5)
	deletedClusters := make(chan string, 5)

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			addedClusters <- cluster
		}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
			updatedClusters <- newCluster
		}, func(clusterServer string) {
			deletedClusters <- clusterServer
			close(addedClusters)
			close(updatedClusters)
			close(deletedClusters)
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

	err = db.DeleteCluster(ctx, cluster.Server)
	if !assert.NoError(t, err) {
		return
	}

	var addClusterServers []string
	for elem := range addedClusters {
		addClusterServers = append(addClusterServers, elem.Server)
	}

	var updatedClusterServers []string
	for elem := range updatedClusters {
		updatedClusterServers = append(updatedClusterServers, elem.Server)
	}

	var deletedClusterServers []string
	for elem := range deletedClusters {
		deletedClusterServers = append(deletedClusterServers, elem)
	}

	assert.ElementsMatch(t, []string{common.KubernetesInternalAPIServerAddr, "http://minikube"}, addClusterServers)
	assert.ElementsMatch(t, []string{"http://minikube"}, updatedClusterServers)
	assert.ElementsMatch(t, []string{"http://minikube"}, deletedClusterServers)
}
