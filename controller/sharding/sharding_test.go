package sharding

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGetShardByID_NotEmptyID(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	assert.Equal(t, 0, LegacyDistributionFunction(db)(&v1alpha1.Cluster{ID: "1"}))
	assert.Equal(t, 0, LegacyDistributionFunction(db)(&v1alpha1.Cluster{ID: "2"}))
	assert.Equal(t, 0, LegacyDistributionFunction(db)(&v1alpha1.Cluster{ID: "3"}))
	assert.Equal(t, 0, LegacyDistributionFunction(db)(&v1alpha1.Cluster{ID: "4"}))
}

func TestGetShardByID_EmptyID(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(1)
	distributionFunction := LegacyDistributionFunction
	shard := distributionFunction(db)(&v1alpha1.Cluster{})
	assert.Equal(t, 0, shard)
}

func TestGetShardByID_NoReplicas(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(0)
	distributionFunction := LegacyDistributionFunction
	shard := distributionFunction(db)(&v1alpha1.Cluster{})
	assert.Equal(t, -1, shard)
}

func TestGetShardByID_NoReplicasUsingHashDistributionFunction(t *testing.T) {
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(0)
	distributionFunction := LegacyDistributionFunction
	shard := distributionFunction(db)(&v1alpha1.Cluster{})
	assert.Equal(t, -1, shard)
}

func TestGetShardByID_NoReplicasUsingHashDistributionFunctionWithClusters(t *testing.T) {
	db, cluster1, cluster2, cluster3, cluster4, cluster5 := createTestClusters()
	// Test with replicas set to 0
	db.On("GetApplicationControllerReplicas").Return(0)
	t.Setenv(common.EnvControllerShardingAlgorithm, common.RoundRobinShardingAlgorithm)
	distributionFunction := RoundRobinDistributionFunction(db)
	assert.Equal(t, -1, distributionFunction(nil))
	assert.Equal(t, -1, distributionFunction(&cluster1))
	assert.Equal(t, -1, distributionFunction(&cluster2))
	assert.Equal(t, -1, distributionFunction(&cluster3))
	assert.Equal(t, -1, distributionFunction(&cluster4))
	assert.Equal(t, -1, distributionFunction(&cluster5))
}

func TestGetClusterFilterDefault(t *testing.T) {
	shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	os.Unsetenv(common.EnvControllerShardingAlgorithm)
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(2)
	filter := GetClusterFilter(db, GetDistributionFunction(db, common.DefaultShardingAlgorithm), shardIndex)
	assert.False(t, filter(&v1alpha1.Cluster{ID: "1"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "2"}))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "3"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "4"}))
}

func TestGetClusterFilterLegacy(t *testing.T) {
	shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(2)
	t.Setenv(common.EnvControllerShardingAlgorithm, common.LegacyShardingAlgorithm)
	filter := GetClusterFilter(db, GetDistributionFunction(db, common.LegacyShardingAlgorithm), shardIndex)
	assert.False(t, filter(&v1alpha1.Cluster{ID: "1"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "2"}))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "3"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "4"}))
}

func TestGetClusterFilterUnknown(t *testing.T) {
	shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(2)
	t.Setenv(common.EnvControllerShardingAlgorithm, "unknown")
	filter := GetClusterFilter(db, GetDistributionFunction(db, "unknown"), shardIndex)
	assert.False(t, filter(&v1alpha1.Cluster{ID: "1"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "2"}))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "3"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "4"}))
}

func TestLegacyGetClusterFilterWithFixedShard(t *testing.T) {
	shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	db := &dbmocks.ArgoDB{}
	db.On("GetApplicationControllerReplicas").Return(2)
	filter := GetClusterFilter(db, GetDistributionFunction(db, common.DefaultShardingAlgorithm), shardIndex)
	assert.False(t, filter(nil))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "1"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "2"}))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "3"}))
	assert.True(t, filter(&v1alpha1.Cluster{ID: "4"}))

	var fixedShard int64 = 4
	filter = GetClusterFilter(db, GetDistributionFunction(db, common.DefaultShardingAlgorithm), int(fixedShard))
	assert.False(t, filter(&v1alpha1.Cluster{ID: "4", Shard: &fixedShard}))

	fixedShard = 1
	filter = GetClusterFilter(db, GetDistributionFunction(db, common.DefaultShardingAlgorithm), int(fixedShard))
	assert.True(t, filter(&v1alpha1.Cluster{Name: "cluster4", ID: "4", Shard: &fixedShard}))
}

func TestRoundRobinGetClusterFilterWithFixedShard(t *testing.T) {
	shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	db.On("GetApplicationControllerReplicas").Return(2)
	filter := GetClusterFilter(db, GetDistributionFunction(db, common.RoundRobinShardingAlgorithm), shardIndex)
	assert.False(t, filter(nil))
	assert.False(t, filter(&cluster1))
	assert.True(t, filter(&cluster2))
	assert.False(t, filter(&cluster3))
	assert.True(t, filter(&cluster4))

	// a cluster with a fixed shard should be processed by the specified exact
	// same shard unless the specified shard index is greater than the number of replicas.
	var fixedShard int64 = 4
	filter = GetClusterFilter(db, GetDistributionFunction(db, common.RoundRobinShardingAlgorithm), int(fixedShard))
	assert.False(t, filter(&v1alpha1.Cluster{Name: "cluster4", ID: "4", Shard: &fixedShard}))

	fixedShard = 1
	filter = GetClusterFilter(db, GetDistributionFunction(db, common.RoundRobinShardingAlgorithm), int(fixedShard))
	assert.True(t, filter(&v1alpha1.Cluster{Name: "cluster4", ID: "4", Shard: &fixedShard}))
}

func TestGetClusterFilterLegacyHash(t *testing.T) {
	shardIndex := 1 // ensuring that a shard with index 1 will process all the clusters with an "even" id (2,4,6,...)
	t.Setenv(common.EnvControllerShardingAlgorithm, "hash")
	db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	db.On("GetApplicationControllerReplicas").Return(2)
	filter := GetClusterFilter(db, GetDistributionFunction(db, common.LegacyShardingAlgorithm), shardIndex)
	assert.False(t, filter(&cluster1))
	assert.True(t, filter(&cluster2))
	assert.False(t, filter(&cluster3))
	assert.True(t, filter(&cluster4))

	// a cluster with a fixed shard should be processed by the specified exact
	// same shard unless the specified shard index is greater than the number of replicas.
	var fixedShard int64 = 4
	filter = GetClusterFilter(db, GetDistributionFunction(db, common.LegacyShardingAlgorithm), int(fixedShard))
	assert.False(t, filter(&v1alpha1.Cluster{Name: "cluster4", ID: "4", Shard: &fixedShard}))

	fixedShard = 1
	filter = GetClusterFilter(db, GetDistributionFunction(db, common.LegacyShardingAlgorithm), int(fixedShard))
	assert.True(t, filter(&v1alpha1.Cluster{Name: "cluster4", ID: "4", Shard: &fixedShard}))
}

func TestGetClusterFilterWithEnvControllerShardingAlgorithms(t *testing.T) {
	db, cluster1, cluster2, cluster3, cluster4, _ := createTestClusters()
	shardIndex := 1
	db.On("GetApplicationControllerReplicas").Return(2)

	t.Run("legacy", func(t *testing.T) {
		t.Setenv(common.EnvControllerShardingAlgorithm, common.LegacyShardingAlgorithm)
		shardShouldProcessCluster := GetClusterFilter(db, GetDistributionFunction(db, common.LegacyShardingAlgorithm), shardIndex)
		assert.False(t, shardShouldProcessCluster(&cluster1))
		assert.True(t, shardShouldProcessCluster(&cluster2))
		assert.False(t, shardShouldProcessCluster(&cluster3))
		assert.True(t, shardShouldProcessCluster(&cluster4))
		assert.False(t, shardShouldProcessCluster(nil))
	})

	t.Run("roundrobin", func(t *testing.T) {
		t.Setenv(common.EnvControllerShardingAlgorithm, common.RoundRobinShardingAlgorithm)
		shardShouldProcessCluster := GetClusterFilter(db, GetDistributionFunction(db, common.LegacyShardingAlgorithm), shardIndex)
		assert.False(t, shardShouldProcessCluster(&cluster1))
		assert.True(t, shardShouldProcessCluster(&cluster2))
		assert.False(t, shardShouldProcessCluster(&cluster3))
		assert.True(t, shardShouldProcessCluster(&cluster4))
		assert.False(t, shardShouldProcessCluster(nil))
	})
}

func TestGetShardByIndexModuloReplicasCountDistributionFunction2(t *testing.T) {
	db, cluster1, cluster2, cluster3, cluster4, cluster5 := createTestClusters()

	t.Run("replicas set to 1", func(t *testing.T) {
		db.On("GetApplicationControllerReplicas").Return(1).Once()
		distributionFunction := RoundRobinDistributionFunction(db)
		assert.Equal(t, 0, distributionFunction(nil))
		assert.Equal(t, 0, distributionFunction(&cluster1))
		assert.Equal(t, 0, distributionFunction(&cluster2))
		assert.Equal(t, 0, distributionFunction(&cluster3))
		assert.Equal(t, 0, distributionFunction(&cluster4))
		assert.Equal(t, 0, distributionFunction(&cluster5))
	})

	t.Run("replicas set to 2", func(t *testing.T) {
		db.On("GetApplicationControllerReplicas").Return(2).Once()
		distributionFunction := RoundRobinDistributionFunction(db)
		assert.Equal(t, 0, distributionFunction(nil))
		assert.Equal(t, 0, distributionFunction(&cluster1))
		assert.Equal(t, 1, distributionFunction(&cluster2))
		assert.Equal(t, 0, distributionFunction(&cluster3))
		assert.Equal(t, 1, distributionFunction(&cluster4))
		assert.Equal(t, 0, distributionFunction(&cluster5))
	})

	t.Run("replicas set to 3", func(t *testing.T) {
		db.On("GetApplicationControllerReplicas").Return(3).Once()
		distributionFunction := RoundRobinDistributionFunction(db)
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
	db := dbmocks.ArgoDB{}
	clusterList := &v1alpha1.ClusterList{Items: []v1alpha1.Cluster{}}
	for i := 0; i < 2048; i++ {
		cluster := createCluster(fmt.Sprintf("cluster-%d", i), fmt.Sprintf("%d", i))
		clusterList.Items = append(clusterList.Items, cluster)
	}
	db.On("ListClusters", mock.Anything).Return(clusterList, nil)
	db.On("GetApplicationControllerReplicas").Return(2)
	distributionFunction := RoundRobinDistributionFunction(&db)
	for i, c := range clusterList.Items {
		assert.Equal(t, i%2, distributionFunction(&c))
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

	clusterList := &v1alpha1.ClusterList{Items: []v1alpha1.Cluster{cluster1, cluster2, cluster3, cluster4, cluster5}}
	db.On("ListClusters", mock.Anything).Return(clusterList, nil)

	// Test with replicas set to 2
	db.On("GetApplicationControllerReplicas").Return(2)
	distributionFunction := RoundRobinDistributionFunction(&db)
	assert.Equal(t, 0, distributionFunction(nil))
	assert.Equal(t, 0, distributionFunction(&cluster1))
	assert.Equal(t, 1, distributionFunction(&cluster2))
	assert.Equal(t, 0, distributionFunction(&cluster3))
	assert.Equal(t, 1, distributionFunction(&cluster4))
	assert.Equal(t, 0, distributionFunction(&cluster5))
	assert.Equal(t, -1, distributionFunction(&cluster6)) // as cluster6 is not in the DB, this one should not have a shard assigned

	// Now, the database knows cluster6. Shard should be assigned a proper shard
	clusterList.Items = append(clusterList.Items, cluster6)
	assert.Equal(t, 1, distributionFunction(&cluster6))

	// Now, we remove the last added cluster, it should be unassigned as well
	clusterList.Items = clusterList.Items[:len(clusterList.Items)-1]
	assert.Equal(t, -1, distributionFunction(&cluster6))
}

func TestGetShardByIndexModuloReplicasCountDistributionFunction(t *testing.T) {
	db, cluster1, cluster2, _, _, _ := createTestClusters()
	db.On("GetApplicationControllerReplicas").Return(2)
	distributionFunction := RoundRobinDistributionFunction(db)

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
	assert.NotNil(t, err)

	osHostnameFunction = func() (string, error) { return "example-shard", nil }
	_, err = InferShard()
	assert.NotNil(t, err)
}

func createTestClusters() (*dbmocks.ArgoDB, v1alpha1.Cluster, v1alpha1.Cluster, v1alpha1.Cluster, v1alpha1.Cluster, v1alpha1.Cluster) {
	db := dbmocks.ArgoDB{}
	cluster1 := createCluster("cluster1", "1")
	cluster2 := createCluster("cluster2", "2")
	cluster3 := createCluster("cluster3", "3")
	cluster4 := createCluster("cluster4", "4")
	cluster5 := createCluster("cluster5", "5")

	db.On("ListClusters", mock.Anything).Return(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{
		cluster1, cluster2, cluster3, cluster4, cluster5,
	}}, nil)
	return &db, cluster1, cluster2, cluster3, cluster4, cluster5
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
