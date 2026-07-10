package sharding

import (
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"
)

func setupTestSharding(shard int, replicas int) *ClusterSharding {
	shardingAlgorithm := "legacy" // we are using the legacy algorithm as it is deterministic based on the cluster id which is easier to test
	db := &dbmocks.ArgoDB{}
	return NewClusterSharding(db, shard, replicas, shardingAlgorithm).(*ClusterSharding)
}

func TestNewClusterSharding(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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

	assert.Len(t, distribution, 2)
}

func TestClusterSharding_AddRoundRobin_Redistributes(t *testing.T) {
	t.Parallel()
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

	assert.Len(t, distributionBefore, 2)

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
	t.Parallel()
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
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app2", "https://127.0.0.1:6443"),
				createApp("app1", "https://kubernetes.default.svc"),
			},
		},
	)

	sharding.Delete("https://kubernetes.default.svc")
	distribution := sharding.GetDistribution()
	assert.Len(t, distribution, 1)
}

func TestClusterSharding_Update(t *testing.T) {
	t.Parallel()
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
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app2", "https://127.0.0.1:6443"),
				createApp("app1", "https://kubernetes.default.svc"),
			},
		},
	)

	distributionBefore := sharding.GetDistribution()
	assert.Len(t, distributionBefore, 2)

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
	assert.Len(t, distributionAfter, 2)

	distributionA, ok = distributionAfter["https://kubernetes.default.svc"]
	assert.True(t, ok)
	assert.Equal(t, 1, distributionA)
}

func TestClusterSharding_UpdateServerName(t *testing.T) {
	t.Parallel()
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
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app2", "https://127.0.0.1:6443"),
				createApp("app1", "https://kubernetes.default.svc"),
			},
		},
	)

	distributionBefore := sharding.GetDistribution()
	assert.Len(t, distributionBefore, 2)

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
	assert.Len(t, distributionAfter, 2)

	_, ok = distributionAfter["https://kubernetes.default.svc"]
	assert.False(t, ok) // the old server name should not be present anymore

	_, ok = distributionAfter["https://server2"]
	assert.True(t, ok) // the new server name should be present
}

func TestClusterSharding_IsManagedCluster(t *testing.T) {
	t.Parallel()
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
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app2", "https://127.0.0.1:6443"),
				createApp("app1", "https://kubernetes.default.svc"),
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
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app2", "https://127.0.0.1:6443"),
				createApp("app1", "https://kubernetes.default.svc"),
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

func TestIsManagedCluster_SkipReconcileAnnotation(t *testing.T) {
	t.Parallel()
	sharding := setupTestSharding(0, 1)
	sharding.Init(
		&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{{ID: "1", Server: "https://cluster1"}}},
		&v1alpha1.ApplicationList{},
	)

	assert.True(t, sharding.IsManagedCluster(&v1alpha1.Cluster{Server: "https://cluster1"}))

	assert.False(t, sharding.IsManagedCluster(&v1alpha1.Cluster{
		Server:      "https://cluster1",
		Annotations: map[string]string{common.AnnotationKeyAppSkipReconcile: "true"},
	}))

	assert.True(t, sharding.IsManagedCluster(&v1alpha1.Cluster{
		Server:      "https://cluster1",
		Annotations: map[string]string{common.AnnotationKeyAppSkipReconcile: "false"},
	}))

	assert.True(t, sharding.IsManagedCluster(nil))
}

func TestClusterSharding_ClusterShardOfResourceShouldNotBeChanged(t *testing.T) {
	t.Parallel()
	shard := 1
	replicas := 2
	sharding := setupTestSharding(shard, replicas)

	clusterWithNil := &v1alpha1.Cluster{
		ID:     "2",
		Server: "https://127.0.0.1:6443",
		Shard:  nil,
	}

	clusterWithValue := &v1alpha1.Cluster{
		ID:     "1",
		Server: "https://kubernetes.default.svc",
		Shard:  new(int64(1)),
	}

	clusterWithToBigValue := &v1alpha1.Cluster{
		ID:     "3",
		Server: "https://1.1.1.1",
		Shard:  new(int64(999)), // shard value is explicitly bigger than the number of replicas
	}

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				*clusterWithNil,
				*clusterWithValue,
				*clusterWithToBigValue,
			},
		},
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app2", "https://127.0.0.1:6443"),
				createApp("app1", "https://kubernetes.default.svc"),
			},
		},
	)
	distribution := sharding.GetDistribution()
	assert.Len(t, distribution, 3)

	assert.Nil(t, sharding.Clusters[clusterWithNil.Server].Shard)

	assert.NotNil(t, sharding.Clusters[clusterWithValue.Server].Shard)
	assert.Equal(t, int64(1), *sharding.Clusters[clusterWithValue.Server].Shard)
	assert.Equal(t, 1, distribution[clusterWithValue.Server])

	assert.NotNil(t, sharding.Clusters[clusterWithToBigValue.Server].Shard)
	assert.Equal(t, int64(999), *sharding.Clusters[clusterWithToBigValue.Server].Shard)
	assert.Equal(t, 0, distribution[clusterWithToBigValue.Server]) // will be assigned to shard 0 because the value is bigger than the number of replicas
}

func TestHasShardingUpdates(t *testing.T) {
	t.Parallel()
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
				Shard:  new(int64(1)),
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(1)),
			},
			expected: false,
		},
		{
			name: "Updates",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(1)),
			},
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(2)),
			},
			expected: true,
		},
		{
			name: "Old is nil",
			old:  nil,
			new: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(2)),
			},
			expected: false,
		},
		{
			name: "New is nil",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(2)),
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
				Shard:  new(int64(2)),
			},
			expected: true,
		},
		{
			name: "New shard is nil",
			old: &v1alpha1.Cluster{
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(2)),
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
				Shard:  new(int64(2)),
			},
			new: &v1alpha1.Cluster{
				ID:     "2",
				Server: "https://kubernetes.default.svc",
				Shard:  new(int64(2)),
			},
			expected: true,
		},
		{
			name: "Server has changed",
			old: &v1alpha1.Cluster{
				ID:     "1",
				Server: "https://server1",
				Shard:  new(int64(2)),
			},
			new: &v1alpha1.Cluster{
				ID:     "1",
				Server: "https://server2",
				Shard:  new(int64(2)),
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.expected, hasShardingUpdates(tc.old, tc.new))
		})
	}
}

// TestClusterSharding_AddApp_SameNameDifferentNamespace ensures apps that share
// a name across namespaces (apps-in-any-namespace) are tracked separately and
// counted independently, instead of colliding on a name-only map key.
func TestClusterSharding_AddApp_SameNameDifferentNamespace(t *testing.T) {
	t.Parallel()
	sharding := setupTestSharding(0, 2)

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{ID: "1", Server: "https://serverA"},
			},
		},
		&v1alpha1.ApplicationList{},
	)

	appA := createAppWithNamespace("frontend", "team-a", "https://serverA")
	appB := createAppWithNamespace("frontend", "team-b", "https://serverA")
	sharding.AddApp(&appA)
	sharding.AddApp(&appB)

	assert.Len(t, sharding.Apps, 2, "same-named apps in different namespaces must not collide")

	appDistribution := sharding.GetAppDistribution()
	assert.Equal(t, 2, appDistribution["https://serverA"], "both apps must be counted for the cluster")
}

// TestClusterSharding_UpdateApp_DestinationChange ensures that changing an
// existing app's destination cluster triggers a redistribution (the consistent
// hashing algorithm weights clusters by app count), while an update that leaves
// the destination unchanged does not.
func TestClusterSharding_UpdateApp_DestinationChange(t *testing.T) {
	t.Parallel()
	sharding := setupTestSharding(0, 2)

	// Spy on the distribution function to observe whether updateDistribution ran.
	var shardCalls int
	sharding.getClusterShard = func(_ *v1alpha1.Cluster) int {
		shardCalls++
		return 0
	}

	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{ID: "1", Server: "https://serverA"},
				{ID: "2", Server: "https://serverB"},
			},
		},
		&v1alpha1.ApplicationList{
			Items: []v1alpha1.Application{
				createApp("app1", "https://serverA"),
			},
		},
	)

	// Moving the app to a different destination cluster must recompute.
	shardCalls = 0
	movedApp := createApp("app1", "https://serverB")
	sharding.UpdateApp(&movedApp)
	assert.Positive(t, shardCalls, "destination change should trigger updateDistribution")

	appDistribution := sharding.GetAppDistribution()
	assert.Equal(t, 1, appDistribution["https://serverB"], "app should now be counted on serverB")
	assert.Equal(t, 0, appDistribution["https://serverA"], "app should no longer be counted on serverA")

	// An update that does not change the destination must skip redistribution.
	shardCalls = 0
	sameApp := createApp("app1", "https://serverB")
	sharding.UpdateApp(&sameApp)
	assert.Zero(t, shardCalls, "no destination change should skip updateDistribution")
}

// TestClusterSharding_GetAppDistribution_ConcurrentWithWrites exercises
// GetAppDistribution concurrently with map writes. It must hold the read lock
// for the whole iteration; run with -race to detect a regression.
func TestClusterSharding_GetAppDistribution_ConcurrentWithWrites(t *testing.T) {
	sharding := setupTestSharding(0, 2)
	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{ID: "1", Server: "https://serverA"},
			},
		},
		&v1alpha1.ApplicationList{},
	)

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: continuously insert new apps (unique keys => real map writes).
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 5000; i++ {
			app := createApp(fmt.Sprintf("app-%d", i), "https://serverA")
			sharding.AddApp(&app)
		}
	}()

	// Reader: hammer GetAppDistribution for the whole lifetime of the writer,
	// so its (unlocked, in the buggy version) map iteration overlaps writes.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				_ = sharding.GetAppDistribution()
			}
		}
	}()

	wg.Wait()
}

// TestClusterSharding_UpdateShard_ConcurrentWithReads exercises UpdateShard
// concurrently with IsManagedCluster. UpdateShard must take the write lock
// while mutating sharding.Shard; run with -race to detect a regression.
func TestClusterSharding_UpdateShard_ConcurrentWithReads(t *testing.T) {
	sharding := setupTestSharding(0, 2)
	sharding.Init(
		&v1alpha1.ClusterList{
			Items: []v1alpha1.Cluster{
				{ID: "1", Server: "https://serverA"},
			},
		},
		&v1alpha1.ApplicationList{},
	)
	cluster := &v1alpha1.Cluster{ID: "1", Server: "https://serverA"}

	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	// Writer: flips the shard assignment back and forth (real writes to sharding.Shard).
	go func() {
		defer wg.Done()
		defer close(done)
		for i := 0; i < 5000; i++ {
			sharding.UpdateShard(i % 2)
		}
	}()

	// Reader: hammers IsManagedCluster, which reads sharding.Shard under RLock,
	// for the whole lifetime of the writer.
	go func() {
		defer wg.Done()
		for {
			select {
			case <-done:
				return
			default:
				sharding.IsManagedCluster(cluster)
			}
		}
	}()

	wg.Wait()
}
