package sharding

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v2/util/db/mocks"
	"github.com/stretchr/testify/assert"
)

func setupTestSharding(shard int, replicas int) *ClusterSharding {
	shardingAlgorithm := "legacy" // we are using the legacy algorithm as it is deterministic based on the cluster id

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
	sharding := setupTestSharding(shard, replicas) // Implement a helper function to setup the test environment

	cluster := &v1alpha1.Cluster{
		ID:     "2",
		Server: "https://127.0.0.1:6443",
	}

	sharding.Add(cluster)

	myCluster := v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
	}

	sharding.Add(&myCluster)

	distribution := sharding.GetDistribution()

	assert.Contains(t, sharding.Clusters, cluster.Server)
	assert.Contains(t, sharding.Clusters, myCluster.Server)

	clusterDistribution, ok := distribution[cluster.Server]
	assert.True(t, ok)
	assert.Equal(t, 1, clusterDistribution)

	myClusterDistribution, ok := distribution[myCluster.Server]
	assert.True(t, ok)
	assert.Equal(t, 0, myClusterDistribution)

	assert.Equal(t, 2, len(distribution))
}

func TestClusterSharding_AddRoundRobin(t *testing.T) {
	shard := 1
	replicas := 2

	db := &dbmocks.ArgoDB{}

	sharding := NewClusterSharding(db, shard, replicas, "round-robin").(*ClusterSharding)

	firstCluster := &v1alpha1.Cluster{
		ID:     "1",
		Server: "https://127.0.0.1:6443",
	}
	sharding.Add(firstCluster)

	secondCluster := v1alpha1.Cluster{
		ID:     "2",
		Server: "https://kubernetes.default.svc",
	}
	sharding.Add(&secondCluster)

	distribution := sharding.GetDistribution()

	assert.Contains(t, sharding.Clusters, firstCluster.Server)
	assert.Contains(t, sharding.Clusters, secondCluster.Server)

	clusterDistribution, ok := distribution[firstCluster.Server]
	assert.True(t, ok)
	assert.Equal(t, 0, clusterDistribution)

	myClusterDistribution, ok := distribution[secondCluster.Server]
	assert.True(t, ok)
	assert.Equal(t, 1, myClusterDistribution)

	assert.Equal(t, 2, len(distribution))
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

	distribution := sharding.GetDistribution()
	assert.Equal(t, 2, len(distribution))

	myClusterDistribution, ok := distribution["https://kubernetes.default.svc"]
	assert.True(t, ok)
	assert.Equal(t, 0, myClusterDistribution)

	sharding.Update(&v1alpha1.Cluster{
		ID:     "4",
		Server: "https://kubernetes.default.svc",
	})

	distribution = sharding.GetDistribution()
	assert.Equal(t, 2, len(distribution))

	myClusterDistribution, ok = distribution["https://kubernetes.default.svc"]
	assert.True(t, ok)
	assert.Equal(t, 1, myClusterDistribution)
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

func Int64Ptr(i int64) *int64 {
	return &i
}

func TestHasShardingUpdates(t *testing.T) {
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
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, hasShardingUpdates(tc.old, tc.new))
		})
	}
}
