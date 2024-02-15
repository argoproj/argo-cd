package sharding

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/stretchr/testify/assert"
)

func setupTestSharding(shard int, replicas int) *ClusterSharding {
	shardingAlgorithm := "legacy" // we are using the legacy algorithm as it is deterministic based on the cluster id which is easier to test
	db := &dbmocks.ArgoDB{}
	return NewClusterSharding(db, shard, replicas, shardingAlgorithm).(*ClusterSharding)
}

func TestNewClusterSharding(t *testing.T) {
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	assert.NotNil(t, sharding)
	assert.Equal(t, shard, sharding.Shard)
	assert.Equal(t, replicas, sharding.Replicas)
	assert.NotNil(t, sharding.Shards)
	assert.NotNil(t, sharding.Clusters)
}

func TestClusterSharding_Add(t *testing.T) {
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	clusterA := &v1alpha1.Cluster{
		ID:     "2",
		Server: "https://127.0.0.1:6443",
	}

	sharding.Add(clusterA)

	clusterB := v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
	}

	sharding.Add(&clusterB)

	distribution := sharding.GetDistribution()

	assert.Contains(t, sharding.Clusters, clusterA.Server)
	assert.Contains(t, sharding.Clusters, clusterB.Server)

	clusterDistribution, ok := distribution[clusterA.Server]
	assert.True(t, ok)
	assert.Equal(t, 1, clusterDistribution)

	myClusterDistribution, ok := distribution[clusterB.Server]
	assert.True(t, ok)
	assert.Equal(t, 0, myClusterDistribution)

	assert.Equal(t, 2, len(distribution))
}

func TestClusterSharding_AddRoundRobin_Redistributes(t *testing.T) {
	shard := 1
	replicas := 2

	db := &dbmocks.ArgoDB{}

	sharding := NewClusterSharding(db, shard, replicas, "round-robin").(*ClusterSharding)

	clusterA := &v1alpha1.Cluster{
		ID:     "1",
		Server: "https://127.0.0.1:6443",
	}
	sharding.Add(clusterA)

	clusterB := v1alpha1.Cluster{
		ID:     "3",
		Server: "https://kubernetes.default.svc",
	}
	sharding.Add(&clusterB)

	distributionBefore := sharding.GetDistribution()

	assert.Contains(t, sharding.Clusters, clusterA.Server)
	assert.Contains(t, sharding.Clusters, clusterB.Server)

	clusterDistributionA, ok := distributionBefore[clusterA.Server]
	assert.True(t, ok)
	assert.Equal(t, 0, clusterDistributionA)

	clusterDistributionB, ok := distributionBefore[clusterB.Server]
	assert.True(t, ok)
	assert.Equal(t, 1, clusterDistributionB)

	assert.Equal(t, 2, len(distributionBefore))

	clusterC := v1alpha1.Cluster{
		ID:     "2",
		Server: "https://1.1.1.1",
	}
	sharding.Add(&clusterC)

	distributionAfter := sharding.GetDistribution()

	assert.Contains(t, sharding.Clusters, clusterA.Server)
	assert.Contains(t, sharding.Clusters, clusterB.Server)
	assert.Contains(t, sharding.Clusters, clusterC.Server)

	clusterDistributionA, ok = distributionAfter[clusterA.Server]
	assert.True(t, ok)
	assert.Equal(t, 0, clusterDistributionA)

	clusterDistributionC, ok := distributionAfter[clusterC.Server]
	assert.True(t, ok)
	assert.Equal(t, 1, clusterDistributionC) // will be assigned to shard 1 because the .ID is smaller then the "B" cluster

	clusterDistributionB, ok = distributionAfter[clusterB.Server]
	assert.True(t, ok)
	assert.Equal(t, 0, clusterDistributionB) // will be reassigned to shard 0 because the .ID is bigger then the "C" cluster
}

func TestClusterSharding_Delete(t *testing.T) {
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{
					ID:     "2",
					Server: "https://127.0.0.1:6443",
				},
				{
					ID:     "1",
					Server: "https://kubernetes.default.svc",
				},
			},
		},
	)

	sharding.Delete("https://kubernetes.default.svc")
	distribution := sharding.GetDistribution()
	assert.Equal(t, 1, len(distribution))
}

func TestClusterSharding_Update(t *testing.T) {
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{
					ID:     "2",
					Server: "https://127.0.0.1:6443",
				},
				{
					ID:     "1",
					Server: "https://kubernetes.default.svc",
				},
			},
		},
	)

	distributionBefore := sharding.GetDistribution()
	assert.Equal(t, 2, len(distributionBefore))

	distributionA, ok := distributionBefore["https://kubernetes.default.svc"]
	assert.True(t, ok)
	assert.Equal(t, 0, distributionA)

	sharding.Update(&v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
	}, &v1alpha1.Cluster{
		ID:     "4",
		Server: "https://kubernetes.default.svc",
	})

	distributionAfter := sharding.GetDistribution()
	assert.Equal(t, 2, len(distributionAfter))

	distributionA, ok = distributionAfter["https://kubernetes.default.svc"]
	assert.True(t, ok)
	assert.Equal(t, 1, distributionA)
}

func TestClusterSharding_UpdateServerName(t *testing.T) {
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{
					ID:     "2",
					Server: "https://127.0.0.1:6443",
				},
				{
					ID:     "1",
					Server: "https://kubernetes.default.svc",
				},
			},
		},
	)

	distributionBefore := sharding.GetDistribution()
	assert.Equal(t, 2, len(distributionBefore))

	distributionA, ok := distributionBefore["https://kubernetes.default.svc"]
	assert.True(t, ok)
	assert.Equal(t, 0, distributionA)

	sharding.Update(&v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
	}, &v1alpha1.Cluster{
		ID:     "1",
		Server: "https://server2",
	})

	distributionAfter := sharding.GetDistribution()
	assert.Equal(t, 2, len(distributionAfter))

	_, ok = distributionAfter["https://kubernetes.default.svc"]
	assert.False(t, ok) // the old server name should not be present anymore

	_, ok = distributionAfter["https://server2"]
	assert.True(t, ok) // the new server name should be present
}

func TestClusterSharding_IsManagedCluster(t *testing.T) {
	replicas := 2
	sharding0 := setupTestSharding(0, replicas)

	sharding0.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{
					ID:     "1",
					Server: "https://kubernetes.default.svc",
				},
				{
					ID:     "2",
					Server: "https://127.0.0.1:6443",
				},
			},
		},
	)

	assert.True(t, sharding0.IsManagedCluster(&v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
	}))

	assert.False(t, sharding0.IsManagedCluster(&v1alpha1.Cluster{
		ID:     "2",
		Server: "https://127.0.0.1:6443",
	}))

	sharding1 := setupTestSharding(1, replicas)

	sharding1.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{
					ID:     "2",
					Server: "https://127.0.0.1:6443",
				},
				{
					ID:     "1",
					Server: "https://kubernetes.default.svc",
				},
			},
		},
	)

	assert.False(t, sharding1.IsManagedCluster(&v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
	}))

	assert.True(t, sharding1.IsManagedCluster(&v1alpha1.Cluster{
		ID:     "2",
		Server: "https://127.0.0.1:6443",
	}))

}

func TestClusterSharding_ClusterShardOfResourceShouldNotBeChanged(t *testing.T) {
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	Int64Ptr := func(i int64) *int64 {
		return &i
	}

	clusterWithNil := &v1alpha1.Cluster{
		ID:     "2",
		Server: "https://127.0.0.1:6443",
		Shard:  nil,
	}

	clusterWithValue := &v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
		Shard:  Int64Ptr(1),
	}

	clusterWithToBigValue := &v1alpha1.Cluster{
		ID:     "3",
		Server: "https://1.1.1.1",
		Shard:  Int64Ptr(999), // shard value is explicitly bigger than the number of replicas
	}

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				*clusterWithNil,
				*clusterWithValue,
				*clusterWithToBigValue,
			},
		},
	)
	distribution := sharding.GetDistribution()
	assert.Equal(t, 3, len(distribution))

	assert.Nil(t, sharding.Clusters[clusterWithNil.Server].Shard)

	assert.NotNil(t, sharding.Clusters[clusterWithValue.Server].Shard)
	assert.Equal(t, int64(1), *sharding.Clusters[clusterWithValue.Server].Shard)
	assert.Equal(t, 1, distribution[clusterWithValue.Server])

	assert.NotNil(t, sharding.Clusters[clusterWithToBigValue.Server].Shard)
	assert.Equal(t, int64(999), *sharding.Clusters[clusterWithToBigValue.Server].Shard)
	assert.Equal(t, 0, distribution[clusterWithToBigValue.Server]) // will be assigned to shard 0 because the value is bigger than the number of replicas
}

func TestHasShardingUpdates(t *testing.T) {
	Int64Ptr := func(i int64) *int64 {
		return &i
	}

	testCases := []struct {
		name     string
		old      *v1alpha1.Cluster
		new      *v1alpha1.Cluster
		expected bool
	}{
		{
			name: "No updates",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(1),
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(1),
			},
			expected: false,
		},
		{
			name: "Updates",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(1),
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			expected: true,
		},
		{
			name: "Old is nil",
			old:  nil,
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			expected: false,
		},
		{
			name: "New is nil",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			new:      nil,
			expected: false,
		},
		{
			name:     "Both are nil",
			old:      nil,
			new:      nil,
			expected: false,
		},
		{
			name: "Both shards are nil",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  nil,
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  nil,
			},
			expected: false,
		},
		{
			name: "Old shard is nil",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  nil,
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			expected: true,
		},
		{
			name: "New shard is nil",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  nil,
			},
			expected: true,
		},
		{
			name: "Cluster ID has changed",
			old: &v1alpha1.Cluster{
				ID:     "1",
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			new: &v1alpha1.Cluster{
				ID:     "2",
				Server: "https://kubernetes.default.svc",
				Shard:  Int64Ptr(2),
			},
			expected: true,
		},
		{
			name: "Server has changed",
			old: &v1alpha1.Cluster{
				ID:     "1",
				Server: "https://server1",
				Shard:  Int64Ptr(2),
			},
			new: &v1alpha1.Cluster{
				ID:     "1",
				Server: "https://server2",
				Shard:  Int64Ptr(2),
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, hasShardingUpdates(tc.old, tc.new))
		})
	}
}
