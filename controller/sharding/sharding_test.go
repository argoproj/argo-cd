package sharding

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/argoproj/argo-cd/v2/util/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestGetShardByID_NotEmptyID(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	replicasCount := 1
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	assert.Equal(t, 0, LegacyDistributionFunction(replicasCount)(&v1alpha1.Cluster{ID: "1"}))
	assert.Equal(t, 0, LegacyDistributionFunction(replicasCount)(&v1alpha1.Cluster{ID: "2"}))
	assert.Equal(t, 0, LegacyDistributionFunction(replicasCount)(&v1alpha1.Cluster{ID: "3"}))
	assert.Equal(t, 0, LegacyDistributionFunction(replicasCount)(&v1alpha1.Cluster{ID: "4"}))
}

func TestGetShardByID_EmptyID(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	replicasCount := 1
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	distributionFunction := LegacyDistributionFunction
	shard := distributionFunction(replicasCount)(&v1alpha1.Cluster{})
	assert.Equal(t, 0, shard)
}

func TestGetShardByID_NoReplicas(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(0)
	distributionFunction := LegacyDistributionFunction
	shard := distributionFunction(0)(&v1alpha1.Cluster{})
	assert.Equal(t, -1, shard)
}

func TestGetShardByID_NoReplicasUsingHashDistributionFunction(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(0)
	distributionFunction := LegacyDistributionFunction
	shard := distributionFunction(0)(&v1alpha1.Cluster{})
	assert.Equal(t, -1, shard)
}

func TestGetShardByID_NoReplicasUsingHashDistributionFunctionWithClusters(t *testing.T) {
	clusters, db, cluster1, cluster2, cluster3, cluster4, cluster5 := createTestClusters()
	// Test with replicas set to 0
	db.On("GetApplicationControllerReplicas").Return(0)
	t.Setenv(common.EnvControllerShardingAlgorithm, common.RoundRobinShardingAlgorithm)
	distributionFunction := RoundRobinDistributionFunction(clusters, 0)
	assert.Equal(t, -1, distributionFunction(nil))
	assert.Equal(t, -1, distributionFunction(&cluster1))
	assert.Equal(t, -1, distributionFunction(&cluster2))
	assert.Equal(t, -1, distributionFunction(&cluster3))
	assert.Equal(t, -1, distributionFunction(&cluster4))
	assert.Equal(t, -1, distributionFunction(&cluster5))
}

func TestGetClusterFilterDefault(t *testing.T) {
	//shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	clusterAccessor, _, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	os.Unsetenv(common.EnvControllerShardingAlgorithm)
	replicasCount := 2
	distributionFunction := RoundRobinDistributionFunction(clusterAccessor, replicasCount)
	assert.Equal(t, 0, distributionFunction(nil))
	assert.Equal(t, 0, distributionFunction(&cluster1))
	assert.Equal(t, 1, distributionFunction(&cluster2))
	assert.Equal(t, 0, distributionFunction(&cluster3))
	assert.Equal(t, 1, distributionFunction(&cluster4))
}

func TestGetClusterFilterLegacy(t *testing.T) {
	//shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	clusterAccessor, db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	replicasCount := 2
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	t.Setenv(common.EnvControllerShardingAlgorithm, common.LegacyShardingAlgorithm)
	distributionFunction := RoundRobinDistributionFunction(clusterAccessor, replicasCount)
	assert.Equal(t, 0, distributionFunction(nil))
	assert.Equal(t, 0, distributionFunction(&cluster1))
	assert.Equal(t, 1, distributionFunction(&cluster2))
	assert.Equal(t, 0, distributionFunction(&cluster3))
	assert.Equal(t, 1, distributionFunction(&cluster4))
}

func TestGetClusterFilterUnknown(t *testing.T) {
	clusterAccessor, db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	// Test with replicas set to 0
	t.Setenv(common.EnvControllerReplicas, "2")
	os.Unsetenv(common.EnvControllerShardingAlgorithm)
	t.Setenv(common.EnvControllerShardingAlgorithm, "unknown")
	replicasCount := 2
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	distributionFunction := GetDistributionFunction(clusterAccessor, "unknown", replicasCount)
	assert.Equal(t, 0, distributionFunction(nil))
	assert.Equal(t, 0, distributionFunction(&cluster1))
	assert.Equal(t, 1, distributionFunction(&cluster2))
	assert.Equal(t, 0, distributionFunction(&cluster3))
	assert.Equal(t, 1, distributionFunction(&cluster4))
}

func TestLegacyGetClusterFilterWithFixedShard(t *testing.T) {
	//shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	t.Setenv(common.EnvControllerReplicas, "5")
	clusterAccessor, db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	replicasCount := 5
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	filter := GetDistributionFunction(clusterAccessor, common.DefaultShardingAlgorithm, replicasCount)
	assert.Equal(t, 0, filter(nil))
	assert.Equal(t, 4, filter(&cluster1))
	assert.Equal(t, 1, filter(&cluster2))
	assert.Equal(t, 2, filter(&cluster3))
	assert.Equal(t, 2, filter(&cluster4))

	var fixedShard int64 = 4
	cluster5 := &v1alpha1.Cluster{ID: "5", Shard: &fixedShard}
	clusterAccessor = getClusterAccessor([]v1alpha1.Cluster{cluster1, cluster2, cluster2, cluster4, *cluster5})
	filter = GetDistributionFunction(clusterAccessor, common.DefaultShardingAlgorithm, replicasCount)
	assert.Equal(t, int(fixedShard), filter(cluster5))

	fixedShard = 1
	cluster5.Shard = &fixedShard
	clusterAccessor = getClusterAccessor([]v1alpha1.Cluster{cluster1, cluster2, cluster2, cluster4, *cluster5})
	filter = GetDistributionFunction(clusterAccessor, common.DefaultShardingAlgorithm, replicasCount)
	assert.Equal(t, int(fixedShard), filter(&v1alpha1.Cluster{ID: "4", Shard: &fixedShard}))
}

func TestRoundRobinGetClusterFilterWithFixedShard(t *testing.T) {
	//shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	t.Setenv(common.EnvControllerReplicas, "4")
	clusterAccessor, db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	replicasCount := 4
	db.On("GetApplicationControllerReplicas").Return(replicasCount)

	filter := GetDistributionFunction(clusterAccessor, common.RoundRobinShardingAlgorithm, replicasCount)
	assert.Equal(t, filter(nil), 0)
	assert.Equal(t, filter(&cluster1), 0)
	assert.Equal(t, filter(&cluster2), 1)
	assert.Equal(t, filter(&cluster3), 2)
	assert.Equal(t, filter(&cluster4), 3)

	// a cluster with a fixed shard should be processed by the specified exact
	// same shard unless the specified shard index is greater than the number of replicas.
	var fixedShard int64 = 1
	cluster5 := v1alpha1.Cluster{Name: "cluster5", ID: "5", Shard: &fixedShard}
	clusters := []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5}
	clusterAccessor = getClusterAccessor(clusters)
	filter = GetDistributionFunction(clusterAccessor, common.RoundRobinShardingAlgorithm, replicasCount)
	assert.Equal(t, int(fixedShard), filter(&cluster5))

	fixedShard = 1
	cluster5 = v1alpha1.Cluster{Name: "cluster5", ID: "5", Shard: &fixedShard}
	clusters = []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5}
	clusterAccessor = getClusterAccessor(clusters)
	filter = GetDistributionFunction(clusterAccessor, common.RoundRobinShardingAlgorithm, replicasCount)
	assert.Equal(t, int(fixedShard), filter(&v1alpha1.Cluster{Name: "cluster4", ID: "4", Shard: &fixedShard}))
}

func TestGetShardByIndexModuloReplicasCountDistributionFunction2(t *testing.T) {
	clusters, db, cluster1, cluster2, cluster3, cluster4, cluster5 := createTestClusters()

	t.Run("replicas set to 1", func(t *testing.T) {
		replicasCount := 1
		db.On("GetApplicationControllerReplicas").Return(replicasCount).Once()
		distributionFunction := RoundRobinDistributionFunction(clusters, replicasCount)
		assert.Equal(t, 0, distributionFunction(nil))
		assert.Equal(t, 0, distributionFunction(&cluster1))
		assert.Equal(t, 0, distributionFunction(&cluster2))
		assert.Equal(t, 0, distributionFunction(&cluster3))
		assert.Equal(t, 0, distributionFunction(&cluster4))
		assert.Equal(t, 0, distributionFunction(&cluster5))
	})

	t.Run("replicas set to 2", func(t *testing.T) {
		replicasCount := 2
		db.On("GetApplicationControllerReplicas").Return(replicasCount).Once()
		distributionFunction := RoundRobinDistributionFunction(clusters, replicasCount)
		assert.Equal(t, 0, distributionFunction(nil))
		assert.Equal(t, 0, distributionFunction(&cluster1))
		assert.Equal(t, 1, distributionFunction(&cluster2))
		assert.Equal(t, 0, distributionFunction(&cluster3))
		assert.Equal(t, 1, distributionFunction(&cluster4))
		assert.Equal(t, 0, distributionFunction(&cluster5))
	})

	t.Run("replicas set to 3", func(t *testing.T) {
		replicasCount := 3
		db.On("GetApplicationControllerReplicas").Return(replicasCount).Once()
		distributionFunction := RoundRobinDistributionFunction(clusters, replicasCount)
		assert.Equal(t, 0, distributionFunction(nil))
		assert.Equal(t, 0, distributionFunction(&cluster1))
		assert.Equal(t, 1, distributionFunction(&cluster2))
		assert.Equal(t, 2, distributionFunction(&cluster3))
		assert.Equal(t, 0, distributionFunction(&cluster4))
		assert.Equal(t, 1, distributionFunction(&cluster5))
	})
}

func TestGetShardByIndexModuloReplicasCountDistributionFunctionWhenClusterNumberIsHigh(t *testing.T) {
	// Unit test written to evaluate the cost of calling db.ListCluster on every call of distributionFunction
	// Doing that allows to accept added and removed clusters on the fly.
	// Initial tests where showing that under 1024 clusters, execution time was around 400ms
	// and for 4096 clusters, execution time was under 9s
	// The other implementation was giving almost linear time of 400ms up to 10'000 clusters
	clusterPointers := []*v1alpha1.Cluster{}
	for i := 0; i < 2048; i++ {
		cluster := createCluster(fmt.Sprintf("cluster-%d", i), fmt.Sprintf("%d", i))
		clusterPointers = append(clusterPointers, &cluster)
	}
	replicasCount := 2
	t.Setenv(common.EnvControllerReplicas, strconv.Itoa(replicasCount))
	_, db, _, _, _, _, _ := createTestClusters()
	clusterAccessor := func() []*v1alpha1.Cluster { return clusterPointers }
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	distributionFunction := RoundRobinDistributionFunction(clusterAccessor, replicasCount)
	for i, c := range clusterPointers {
		assert.Equal(t, i%2, distributionFunction(c))
	}
}

func TestGetShardByIndexModuloReplicasCountDistributionFunctionWhenClusterIsAddedAndRemoved(t *testing.T) {
	db := dbmocks.ArgoDB{}
	cluster1 := createCluster("cluster1", "1")
	cluster2 := createCluster("cluster2", "2")
	cluster3 := createCluster("cluster3", "3")
	cluster4 := createCluster("cluster4", "4")
	cluster5 := createCluster("cluster5", "5")
	cluster6 := createCluster("cluster6", "6")

	clusters := []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5}
	clusterAccessor := getClusterAccessor(clusters)

	clusterList := &v1alpha1.ClusterList{Items: []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5}}
	db.On("ListClusters", mock.Anything).Return(clusterList, nil)
	// Test with replicas set to 2
	replicasCount := 2
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	distributionFunction := RoundRobinDistributionFunction(clusterAccessor, replicasCount)
	assert.Equal(t, 0, distributionFunction(nil))
	assert.Equal(t, 0, distributionFunction(&cluster1))
	assert.Equal(t, 1, distributionFunction(&cluster2))
	assert.Equal(t, 0, distributionFunction(&cluster3))
	assert.Equal(t, 1, distributionFunction(&cluster4))
	assert.Equal(t, 0, distributionFunction(&cluster5))
	assert.Equal(t, -1, distributionFunction(&cluster6)) // as cluster6 is not in the DB, this one should not have a shard assigned

	// Now, the database knows cluster6. Shard should be assigned a proper shard
	clusterList.Items = append(clusterList.Items, cluster6)
	distributionFunction = RoundRobinDistributionFunction(getClusterAccessor(clusterList.Items), replicasCount)
	assert.Equal(t, 1, distributionFunction(&cluster6))

	// Now, we remove the last added cluster, it should be unassigned as well
	clusterList.Items = clusterList.Items[:len(clusterList.Items)-1]
	distributionFunction = RoundRobinDistributionFunction(getClusterAccessor(clusterList.Items), replicasCount)
	assert.Equal(t, -1, distributionFunction(&cluster6))
}

func TestGetShardByIndexModuloReplicasCountDistributionFunction(t *testing.T) {
	clusters, db, cluster1, cluster2, _, _, _ := createTestClusters()
	replicasCount := 2
	db.On("GetApplicationControllerReplicas").Return(replicasCount)
	distributionFunction := RoundRobinDistributionFunction(clusters, replicasCount)

	// Test that the function returns the correct shard for cluster1 and cluster2
	expectedShardForCluster1 := 0
	expectedShardForCluster2 := 1
	shardForCluster1 := distributionFunction(&cluster1)
	shardForCluster2 := distributionFunction(&cluster2)

	if shardForCluster1 != expectedShardForCluster1 {
		t.Errorf("Expected shard for cluster1 to be %d but got %d", expectedShardForCluster1, shardForCluster1)
	}
	if shardForCluster2 != expectedShardForCluster2 {
		t.Errorf("Expected shard for cluster2 to be %d but got %d", expectedShardForCluster2, shardForCluster2)
	}
}

func TestInferShard(t *testing.T) {
	// Override the os.Hostname function to return a specific hostname for testing
	defer func() { osHostnameFunction = os.Hostname }()

	expectedShard := 3
	osHostnameFunction = func() (string, error) { return "example-shard-3", nil }
	actualShard, _ := InferShard()
	assert.Equal(t, expectedShard, actualShard)

	osHostnameError := errors.New("cannot resolve hostname")
	osHostnameFunction = func() (string, error) { return "exampleshard", osHostnameError }
	_, err := InferShard()
	assert.NotNil(t, err)
	assert.Equal(t, err, osHostnameError)

	osHostnameFunction = func() (string, error) { return "exampleshard", nil }
	_, err = InferShard()
	assert.Nil(t, err)

	osHostnameFunction = func() (string, error) { return "example-shard", nil }
	_, err = InferShard()
	assert.Nil(t, err)
}

func createTestClusters() (clusterAccessor, *dbmocks.ArgoDB, v1alpha1.Cluster, v1alpha1.Cluster, v1alpha1.Cluster, v1alpha1.Cluster, v1alpha1.Cluster) {
	db := dbmocks.ArgoDB{}
	cluster1 := createCluster("cluster1", "1")
	cluster2 := createCluster("cluster2", "2")
	cluster3 := createCluster("cluster3", "3")
	cluster4 := createCluster("cluster4", "4")
	cluster5 := createCluster("cluster5", "5")

	clusters := []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5}

	db.On("ListClusters", mock.Anything).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{
		cluster1, cluster2, cluster3, cluster4, cluster5,
	}}, nil)
	return getClusterAccessor(clusters), &db, cluster1, cluster2, cluster3, cluster4, cluster5
}

func getClusterAccessor(clusters []v1alpha1.Cluster) clusterAccessor {
	// Convert the array to a slice of pointers
	clusterPointers := getClusterPointers(clusters)
	clusterAccessor := func() []*v1alpha1.Cluster { return clusterPointers }
	return clusterAccessor
}

func getClusterPointers(clusters []v1alpha1.Cluster) []*v1alpha1.Cluster {
	var clusterPointers []*v1alpha1.Cluster
	for i := range clusters {
		clusterPointers = append(clusterPointers, &clusters[i])
	}
	return clusterPointers
}

func createCluster(name string, id string) v1alpha1.Cluster {
	cluster := v1alpha1.Cluster{
		Name:   name,
		ID:     id,
		Server: "https://kubernetes.default.svc?" + id,
	}
	return cluster
}

func Test_getDefaultShardMappingData(t *testing.T) {
	expectedData := []shardApplicationControllerMapping{
		{
			ShardNumber:    0,
			ControllerName: "",
		}, {
			ShardNumber:    1,
			ControllerName: "",
		},
	}

	shardMappingData := getDefaultShardMappingData(2)
	assert.Equal(t, expectedData, shardMappingData)
}

func Test_generateDefaultShardMappingCM_NoPredefinedShard(t *testing.T) {
	replicas := 2
	expectedTime := metav1.Now()
	defer func() { osHostnameFunction = os.Hostname }()
	defer func() { heartbeatCurrentTime = metav1.Now }()

	expectedMapping := []shardApplicationControllerMapping{
		{
			ShardNumber:    0,
			ControllerName: "test-example",
			HeartbeatTime:  expectedTime,
		}, {
			ShardNumber: 1,
		},
	}

	expectedMappingCM, err := json.Marshal(expectedMapping)
	assert.NoError(t, err)

	expectedShadingCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDAppControllerShardConfigMapName,
			Namespace: "test",
		},
		Data: map[string]string{
			"shardControllerMapping": string(expectedMappingCM),
		},
	}
	heartbeatCurrentTime = func() metav1.Time { return expectedTime }
	osHostnameFunction = func() (string, error) { return "test-example", nil }
	shardingCM, err := generateDefaultShardMappingCM("test", "test-example", replicas, -1)
	assert.NoError(t, err)
	assert.Equal(t, expectedShadingCM, shardingCM)

}

func Test_generateDefaultShardMappingCM_PredefinedShard(t *testing.T) {
	replicas := 2
	expectedTime := metav1.Now()
	defer func() { osHostnameFunction = os.Hostname }()
	defer func() { heartbeatCurrentTime = metav1.Now }()

	expectedMapping := []shardApplicationControllerMapping{
		{
			ShardNumber: 0,
		}, {
			ShardNumber:    1,
			ControllerName: "test-example",
			HeartbeatTime:  expectedTime,
		},
	}

	expectedMappingCM, err := json.Marshal(expectedMapping)
	assert.NoError(t, err)

	expectedShadingCM := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.ArgoCDAppControllerShardConfigMapName,
			Namespace: "test",
		},
		Data: map[string]string{
			"shardControllerMapping": string(expectedMappingCM),
		},
	}
	heartbeatCurrentTime = func() metav1.Time { return expectedTime }
	osHostnameFunction = func() (string, error) { return "test-example", nil }
	shardingCM, err := generateDefaultShardMappingCM("test", "test-example", replicas, 1)
	assert.NoError(t, err)
	assert.Equal(t, expectedShadingCM, shardingCM)

}

func Test_getOrUpdateShardNumberForController(t *testing.T) {
	expectedTime := metav1.Now()

	testCases := []struct {
		name                              string
		shardApplicationControllerMapping []shardApplicationControllerMapping
		hostname                          string
		replicas                          int
		shard                             int
		expectedShard                     int
		expectedShardMappingData          []shardApplicationControllerMapping
	}{
		{
			name: "length of shard mapping less than number of replicas - Existing controller",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  metav1.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			hostname:      "test-example",
			replicas:      2,
			shard:         -1,
			expectedShard: 0,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "",
					ShardNumber:    1,
					HeartbeatTime:  metav1.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				},
			},
		},
		{
			name: "length of shard mapping less than number of replicas - New controller",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				},
			},
			hostname:      "test-example-1",
			replicas:      2,
			shard:         -1,
			expectedShard: 1,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				},
			},
		},
		{
			name: "length of shard mapping more than number of replicas",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				},
			},
			hostname:      "test-example",
			replicas:      1,
			shard:         -1,
			expectedShard: 0,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				},
			},
		},
		{
			name: "shard number is pre-specified and length of shard mapping less than number of replicas - Existing controller",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  metav1.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				}, {
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				},
			},
			hostname:      "test-example-1",
			replicas:      2,
			shard:         1,
			expectedShard: 1,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				},
			},
		},
		{
			name: "shard number is pre-specified and length of shard mapping less than number of replicas - New controller",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				},
			},
			hostname:      "test-example-1",
			replicas:      2,
			shard:         1,
			expectedShard: 1,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				},
			},
		},
		{
			name: "shard number is pre-specified and length of shard mapping more than number of replicas",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-2",
					ShardNumber:    2,
					HeartbeatTime:  expectedTime,
				},
			},
			hostname:      "test-example",
			replicas:      2,
			shard:         1,
			expectedShard: 1,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "",
					ShardNumber:    0,
					HeartbeatTime:  metav1.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				}, {
					ControllerName: "test-example",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				},
			},
		},
		{
			name: "updating heartbeat",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  metav1.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			hostname:      "test-example-1",
			replicas:      2,
			shard:         -1,
			expectedShard: 1,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				},
			},
		},
		{
			name: "updating heartbeat - shard pre-defined",
			shardApplicationControllerMapping: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  metav1.Date(1, time.January, 1, 0, 0, 0, 0, time.UTC),
				},
			},
			hostname:      "test-example-1",
			replicas:      2,
			shard:         1,
			expectedShard: 1,
			expectedShardMappingData: []shardApplicationControllerMapping{
				{
					ControllerName: "test-example",
					ShardNumber:    0,
					HeartbeatTime:  expectedTime,
				}, {
					ControllerName: "test-example-1",
					ShardNumber:    1,
					HeartbeatTime:  expectedTime,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() { osHostnameFunction = os.Hostname }()
			heartbeatCurrentTime = func() metav1.Time { return expectedTime }
			shard, shardMappingData := getOrUpdateShardNumberForController(tc.shardApplicationControllerMapping, tc.hostname, tc.replicas, tc.shard)
			assert.Equal(t, tc.expectedShard, shard)
			assert.Equal(t, tc.expectedShardMappingData, shardMappingData)
		})
	}
}

func TestGetClusterSharding(t *testing.T) {
	IntPtr := func(i int32) *int32 {
		return &i
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      common.DefaultApplicationControllerName,
			Namespace: "argocd",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: IntPtr(1),
		},
	}

	deploymentMultiReplicas := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argocd-application-controller-multi-replicas",
			Namespace: "argocd",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: IntPtr(3),
		},
	}

	objects := append([]runtime.Object{}, deployment, deploymentMultiReplicas)
	kubeclientset := kubefake.NewSimpleClientset(objects...)

	settingsMgr := settings.NewSettingsManager(context.TODO(), kubeclientset, "argocd", settings.WithRepoOrClusterChangedHandler(func() {
	}))

	testCases := []struct {
		name               string
		useDynamicSharding bool
		envsSetter         func(t *testing.T)
		cleanup            func()
		expectedShard      int
		expectedReplicas   int
		expectedErr        error
	}{
		{
			name: "Default sharding with statefulset",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvControllerReplicas, "1")
			},
			cleanup:            func() {},
			useDynamicSharding: false,
			expectedShard:      0,
			expectedReplicas:   1,
			expectedErr:        nil,
		},
		{
			name: "Default sharding with deployment",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvAppControllerName, common.DefaultApplicationControllerName)
			},
			cleanup:            func() {},
			useDynamicSharding: true,
			expectedShard:      0,
			expectedReplicas:   1,
			expectedErr:        nil,
		},
		{
			name: "Default sharding with deployment and multiple replicas",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvAppControllerName, "argocd-application-controller-multi-replicas")
			},
			cleanup:            func() {},
			useDynamicSharding: true,
			expectedShard:      0,
			expectedReplicas:   3,
			expectedErr:        nil,
		},
		{
			name: "Statefulset multiple replicas",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvControllerReplicas, "3")
				osHostnameFunction = func() (string, error) { return "example-shard-3", nil }
			},
			cleanup: func() {
				osHostnameFunction = os.Hostname
			},
			useDynamicSharding: false,
			expectedShard:      3,
			expectedReplicas:   3,
			expectedErr:        nil,
		},
		{
			name: "Explicit shard with statefulset and 1 replica",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvControllerReplicas, "1")
				t.Setenv(common.EnvControllerShard, "3")
			},
			cleanup:            func() {},
			useDynamicSharding: false,
			expectedShard:      0,
			expectedReplicas:   1,
			expectedErr:        nil,
		},
		{
			name: "Explicit shard with statefulset and 2 replica - and to high shard",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvControllerReplicas, "2")
				t.Setenv(common.EnvControllerShard, "3")
			},
			cleanup:            func() {},
			useDynamicSharding: false,
			expectedShard:      0,
			expectedReplicas:   2,
			expectedErr:        nil,
		},
		{
			name: "Explicit shard with statefulset and 2 replica",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvControllerReplicas, "2")
				t.Setenv(common.EnvControllerShard, "1")
			},
			cleanup:            func() {},
			useDynamicSharding: false,
			expectedShard:      1,
			expectedReplicas:   2,
			expectedErr:        nil,
		},
		{
			name: "Explicit shard with deployment",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvControllerShard, "3")
			},
			cleanup:            func() {},
			useDynamicSharding: true,
			expectedShard:      0,
			expectedReplicas:   1,
			expectedErr:        nil,
		},
		{
			name: "Explicit shard with deployment and multiple replicas will read from configmap",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvAppControllerName, "argocd-application-controller-multi-replicas")
				t.Setenv(common.EnvControllerShard, "3")
			},
			cleanup:            func() {},
			useDynamicSharding: true,
			expectedShard:      0,
			expectedReplicas:   3,
			expectedErr:        nil,
		},
		{
			name: "Dynamic sharding but missing deployment",
			envsSetter: func(t *testing.T) {
				t.Setenv(common.EnvAppControllerName, "missing-deployment")
			},
			cleanup:            func() {},
			useDynamicSharding: true,
			expectedShard:      0,
			expectedReplicas:   1,
			expectedErr:        fmt.Errorf("(dynamic cluster distribution) failed to get app controller deployment: deployments.apps \"missing-deployment\" not found"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.envsSetter(t)
			defer tc.cleanup()
			shardingCache, err := GetClusterSharding(kubeclientset, settingsMgr, "round-robin", tc.useDynamicSharding)

			if shardingCache != nil {
				clusterSharding := shardingCache.(*ClusterSharding)
				assert.Equal(t, tc.expectedShard, clusterSharding.Shard)
				assert.Equal(t, tc.expectedReplicas, clusterSharding.Replicas)
			}

			if tc.expectedErr != nil {
				if err != nil {
					assert.Equal(t, tc.expectedErr.Error(), err.Error())
				} else {
					t.Errorf("Expected error %v but got nil", tc.expectedErr)
				}
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
