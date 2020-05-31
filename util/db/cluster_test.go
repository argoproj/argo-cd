package db

import (
	"context"
	"sync"
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
	syncMessage   = "Sync successful"
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
	const cluserServerAddr = "http://minikube"

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addedClusters := make([]string, 0)
	updatedClusters := make([]string, 0)
	deletedClusters := make([]string, 0)

	var wg sync.WaitGroup
	wg.Add(4)

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			addedClusters = append(addedClusters, cluster.Server)
			wg.Done()
		}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
			updatedClusters = append(updatedClusters, newCluster.Server)
			assert.Equal(t, syncMessage, newCluster.ConnectionState.Message)
			assert.Empty(t, oldCluster.ConnectionState.Message)
			wg.Done()
		}, func(clusterServer string) {
			deletedClusters = append(deletedClusters, clusterServer)
			wg.Done()
		}))
	}()

	err := crudCluster(ctx, db, cluserServerAddr, syncMessage)
	assert.NoError(t, err, "Test prepare test data crdCluster failed")

	wg.Wait()

	assert.ElementsMatch(t, []string{common.KubernetesInternalAPIServerAddr, cluserServerAddr}, addedClusters)
	assert.ElementsMatch(t, []string{"http://minikube"}, updatedClusters)
	assert.ElementsMatch(t, []string{"http://minikube"}, deletedClusters)
}

//Cluster with address common.KubernetesInternalAPIServerAddr is local cluster
//In this test we crud local cluster
func TestWatchClustersLocalCluster(t *testing.T) {
	timeout := time.Second * 5

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan bool, 1)

	addedClusters := make(chan *v1alpha1.Cluster, 2)
	updatedClusters := make(chan *v1alpha1.Cluster, 2)

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			addedClusters <- cluster
			done <- true
		}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
			updatedClusters <- newCluster
		}, func(clusterServer string) {
			assert.Fail(t, "Not expecting delete for local cluster")
		}))
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		assert.Fail(t, "Failed due to timeout when starting clusterSecretInformer")
	}

	//crud local cluster
	err := crudCluster(ctx, db, common.KubernetesInternalAPIServerAddr, syncMessage)
	assert.NoError(t, err, "Test prepare test data crdCluster failed")

	for i := 0; i < 3; i++ {
		select {
		case addedCluster := <-addedClusters:
			assert.Equal(t, addedCluster.Server, common.KubernetesInternalAPIServerAddr)
		case updatedCluster := <-updatedClusters:
			assert.Equal(t, updatedCluster.Server, common.KubernetesInternalAPIServerAddr)
		case <-time.After(timeout):
			assert.Fail(t, "no cluster event within timeout")

		}
	}
}

func crudCluster(ctx context.Context, db ArgoDB, cluserServerAddr string, message string) error {
	cluster, err := db.CreateCluster(ctx, &v1alpha1.Cluster{Server: cluserServerAddr})
	if err != nil {
		return err
	}

	cluster.ConnectionState.Message = message
	cluster, err = db.UpdateCluster(ctx, cluster)
	if err != nil {
		return err
	}

	err = db.DeleteCluster(ctx, cluster.Server)
	if err != nil {
		return err
	}

	return err
}
