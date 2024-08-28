package sharding

import (
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
)

func TestLargeShuffle(t *testing.T) {
	t.Skip()
	db := dbmocks.ArgoDB{}
	clusterList := &v1alpha1.ClusterList{Items: []v1alpha1.Cluster{}}
	for i := 0; i < math.MaxInt/4096; i += 256 {
		// fmt.Fprintf(os.Stdout, "%d", i)
		cluster := createCluster(fmt.Sprintf("cluster-%d", i), fmt.Sprintf("%d", i))
		clusterList.Items = append(clusterList.Items, cluster)
	}
	db.On("ListClusters", mock.Anything).Return(clusterList, nil)
	clusterAccessor := getClusterAccessor(clusterList.Items)
	// Test with replicas set to 256
	replicasCount := 256
	t.Setenv(common.EnvControllerReplicas, strconv.Itoa(replicasCount))
	distributionFunction := RoundRobinDistributionFunction(clusterAccessor, replicasCount)
	for i, c := range clusterList.Items {
		assert.Equal(t, i%2567, distributionFunction(&c))
	}
}

func TestShuffle(t *testing.T) {
	t.Skip()
	db := dbmocks.ArgoDB{}
	cluster1 := createCluster("cluster1", "10")
	cluster2 := createCluster("cluster2", "20")
	cluster3 := createCluster("cluster3", "30")
	cluster4 := createCluster("cluster4", "40")
	cluster5 := createCluster("cluster5", "50")
	cluster6 := createCluster("cluster6", "60")
	cluster25 := createCluster("cluster6", "25")

	clusterList := &v1alpha1.ClusterList{Items: []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5, cluster6}}
	db.On("ListClusters", mock.Anything).Return(clusterList, nil)
	clusterAccessor := getClusterAccessor(clusterList.Items)
	// Test with replicas set to 3
	t.Setenv(common.EnvControllerReplicas, "3")
	replicasCount := 3
	distributionFunction := RoundRobinDistributionFunction(clusterAccessor, replicasCount)
	assert.Equal(t, 0, distributionFunction(nil))
	assert.Equal(t, 0, distributionFunction(&cluster1))
	assert.Equal(t, 1, distributionFunction(&cluster2))
	assert.Equal(t, 2, distributionFunction(&cluster3))
	assert.Equal(t, 0, distributionFunction(&cluster4))
	assert.Equal(t, 1, distributionFunction(&cluster5))
	assert.Equal(t, 2, distributionFunction(&cluster6))

	// Now, we remove cluster1, it should be unassigned, and all the other should be resuffled
	clusterList.Items = Remove(clusterList.Items, 0)
	assert.Equal(t, -1, distributionFunction(&cluster1))
	assert.Equal(t, 0, distributionFunction(&cluster2))
	assert.Equal(t, 1, distributionFunction(&cluster3))
	assert.Equal(t, 2, distributionFunction(&cluster4))
	assert.Equal(t, 0, distributionFunction(&cluster5))
	assert.Equal(t, 1, distributionFunction(&cluster6))

	// Now, we add a cluster with an id=25 so it will be placed right after cluster2
	clusterList.Items = append(clusterList.Items, cluster25)
	assert.Equal(t, -1, distributionFunction(&cluster1))
	assert.Equal(t, 0, distributionFunction(&cluster2))
	assert.Equal(t, 1, distributionFunction(&cluster25))
	assert.Equal(t, 2, distributionFunction(&cluster3))
	assert.Equal(t, 0, distributionFunction(&cluster4))
	assert.Equal(t, 1, distributionFunction(&cluster5))
	assert.Equal(t, 2, distributionFunction(&cluster6))
}

func Remove(slice []v1alpha1.Cluster, s int) []v1alpha1.Cluster {
	return append(slice[:s], slice[s+1:]...)
}
