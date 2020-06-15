package db

import (
	"context"
	"sync"
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
	syncMessage   = "Sync successful"
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
	_, err := db.UpdateCluster(context.Background(), &v1alpha1.Cluster{
		Name:   "test",
		Server: "http://mycluster",
		ConnectionState: v1alpha1.ConnectionState{
			Status:  v1alpha1.ConnectionStatusFailed,
			Message: "test message",
		},
	})
	if !assert.NoError(t, err) {
		return
	}

	secret, err := kubeclientset.CoreV1().Secrets(fakeNamespace).Get("mycluster", metav1.GetOptions{})
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, secret.Annotations["status"], v1alpha1.ConnectionStatusFailed)
	assert.Equal(t, secret.Annotations["message"], "test message")
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

//In this test we crud a new cluster
func TestWatchClusters(t *testing.T) {
	timeout := time.Second * 5
	const cluserServerAddr = "http://minikube"

	kubeclientset := fake.NewSimpleClientset()
	settingsManager := settings.NewSettingsManager(context.Background(), kubeclientset, fakeNamespace)
	db := NewDB(fakeNamespace, settingsManager, kubeclientset)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	addedClusters := make([]string, 0)
	updatedClusters := make([]string, 0)
	deletedClusters := make([]string, 0)

	milestoneTestSetupDone := make(chan bool, 1)

	var wg sync.WaitGroup
	//It includes events of create, mod, delete this cluster and also create local cluster. So total is 4 events
	wg.Add(4)

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			addedClusters = append(addedClusters, cluster.Server)
			milestoneTestSetupDone <- true
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

	select {
	case <-milestoneTestSetupDone:
	case <-time.After(timeout):
		assert.Fail(t, "Failed due to timeout when starting clusterSecretInformer")
	}

	time.Sleep(300 * time.Millisecond)
	err := crudCluster(ctx, db, cluserServerAddr, syncMessage)
	assert.NoError(t, err, "Test prepare test data crdCluster failed")

	wg.Wait()

	//addedClusters contains local cluster and the new crud cluster
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

	milestoneTestSetupDone := make(chan bool, 1)
	var wg sync.WaitGroup
	wg.Add(3)

	addedClusters := make([]string, 0)
	updatedClusters := make([]string, 0)

	go func() {
		assert.NoError(t, db.WatchClusters(ctx, func(cluster *v1alpha1.Cluster) {
			addedClusters = append(addedClusters, cluster.Server)
			milestoneTestSetupDone <- true
			wg.Done()
		}, func(oldCluster *v1alpha1.Cluster, newCluster *v1alpha1.Cluster) {
			updatedClusters = append(updatedClusters, newCluster.Server)
			wg.Done()
		}, func(clusterServer string) {
			assert.Fail(t, "Not expecting delete for local cluster")
		}))
	}()

	select {
	case <-milestoneTestSetupDone:
	case <-time.After(timeout):
		assert.Fail(t, "Failed due to timeout when starting clusterSecretInformer")
	}

	//crud local cluster
	time.Sleep(300 * time.Millisecond)
	err := crudCluster(ctx, db, common.KubernetesInternalAPIServerAddr, syncMessage)
	assert.NoError(t, err, "Test prepare test data crdCluster failed ")

	wg.Wait()

	assert.ElementsMatch(t, []string{common.KubernetesInternalAPIServerAddr}, addedClusters)
	assert.ElementsMatch(t, []string{common.KubernetesInternalAPIServerAddr, common.KubernetesInternalAPIServerAddr}, updatedClusters)
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
