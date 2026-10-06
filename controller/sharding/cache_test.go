package sharding

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

// requireMatchesFreshReplica asserts that, after a sequence of cluster events,
// the distribution of sharding is the one a replica would compute if it started
// now from the surviving clusters. Every replica must arrive at the same
// distribution, whatever event history it observed: a replica that diverges
// from a freshly started one owns clusters nobody else thinks it owns, or
// drops clusters every other replica thinks it has.
func requireMatchesFreshReplica(t *testing.T, sharding *ClusterSharding, algorithm string, surviving ...v1alpha1.Cluster) {
	t.Helper()
	fresh := NewClusterSharding(&dbmocks.ArgoDB{}, sharding.Shard, sharding.Replicas, algorithm).(*ClusterSharding)
	fresh.Init(&v1alpha1.ClusterList{Items: surviving}, &v1alpha1.ApplicationList{})
	require.Equal(t, fresh.GetDistribution(), sharding.GetDistribution())
}

var updateScenarioAlgorithms = []string{
	common.LegacyShardingAlgorithm,
	common.RoundRobinShardingAlgorithm,
	common.ConsistentHashingWithBoundedLoadsAlgorithm,
}

// TestClusterSharding_UpdateAfterDuplicateSecretDelete covers a cluster that
// reaches Update without being in the cache. Two cluster secrets with the same
// server URL share one cache entry, because the cache is keyed by server.
// Deleting one of them removes that entry (Delete is keyed by server too), and
// the informer's next resync of the surviving secret is an Update for a server
// the cache no longer knows, with no sharding-relevant change between old and
// new.
func TestClusterSharding_UpdateAfterDuplicateSecretDelete(t *testing.T) {
	t.Parallel()
	for _, algorithm := range updateScenarioAlgorithms {
		t.Run(algorithm, func(t *testing.T) {
			t.Parallel()
			other := v1alpha1.Cluster{ID: "1", Server: "https://other"}
			third := v1alpha1.Cluster{ID: "4", Server: "https://third"}
			first := v1alpha1.Cluster{ID: "2", Server: "https://shared"}
			second := v1alpha1.Cluster{ID: "3", Server: "https://shared"}

			sharding := NewClusterSharding(&dbmocks.ArgoDB{}, 0, 2, algorithm).(*ClusterSharding)
			sharding.Init(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{other, third}}, &v1alpha1.ApplicationList{})
			sharding.Add(&first)
			sharding.Add(&second)
			sharding.Delete(first.Server)
			sharding.Update(&second, &second)

			_, assigned := sharding.GetDistribution()[second.Server]
			require.True(t, assigned, "the surviving cluster must have a shard")
			requireMatchesFreshReplica(t, sharding, algorithm, other, second, third)
		})
	}
}

// TestClusterSharding_UpdateDuplicateSecretResync covers the cached entry and
// the event disagreeing on the cluster ID. With two secrets for the same server,
// the cache holds whichever was stored last, so a resync Update of the other
// one carries no change between its own old and new copies but does change the
// cluster ID the distribution is computed from.
func TestClusterSharding_UpdateDuplicateSecretResync(t *testing.T) {
	t.Parallel()
	for _, algorithm := range updateScenarioAlgorithms {
		t.Run(algorithm, func(t *testing.T) {
			t.Parallel()
			other := v1alpha1.Cluster{ID: "2", Server: "https://other"}
			third := v1alpha1.Cluster{ID: "4", Server: "https://third"}
			first := v1alpha1.Cluster{ID: "1", Server: "https://shared"}
			second := v1alpha1.Cluster{ID: "3", Server: "https://shared"}

			sharding := NewClusterSharding(&dbmocks.ArgoDB{}, 0, 2, algorithm).(*ClusterSharding)
			sharding.Init(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{other, third}}, &v1alpha1.ApplicationList{})
			sharding.Add(&first)
			sharding.Add(&second)
			sharding.Update(&first, &first)

			requireMatchesFreshReplica(t, sharding, algorithm, other, first, third)
		})
	}
}

// TestClusterSharding_UpdateRenameOntoCachedServer covers a server URL change
// whose destination is already in the cache with the same cluster, which
// happens when the Init snapshot and the informer's initial list straddle a
// rename: Init caches the cluster at its new server, the informer then adds it
// at the old one, and the rename event follows. Removing the old server shrinks
// the cluster set, so the distribution must be recomputed even though the
// destination entry itself is unchanged.
func TestClusterSharding_UpdateRenameOntoCachedServer(t *testing.T) {
	t.Parallel()
	for _, algorithm := range updateScenarioAlgorithms {
		t.Run(algorithm, func(t *testing.T) {
			t.Parallel()
			first := v1alpha1.Cluster{ID: "1", Server: "https://first"}
			third := v1alpha1.Cluster{ID: "3", Server: "https://third"}
			before := v1alpha1.Cluster{ID: "2", Server: "https://before"}
			after := v1alpha1.Cluster{ID: "2", Server: "https://after"}

			sharding := NewClusterSharding(&dbmocks.ArgoDB{}, 0, 2, algorithm).(*ClusterSharding)
			sharding.Init(&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{first, after, third}}, &v1alpha1.ApplicationList{})
			sharding.Add(&before)
			sharding.Update(&before, &after)

			_, stale := sharding.GetDistribution()[before.Server]
			require.False(t, stale, "the old server must be removed")
			requireMatchesFreshReplica(t, sharding, algorithm, first, after, third)
		})
	}
}

// TestClusterSharding_UpdateUnchangedClusterSkipsRedistribution verifies that the
// periodic resync of a known, unchanged cluster does not recompute the
// distribution. updateDistribution is idempotent, so a sentinel shard value is
// planted that a recomputation would overwrite.
func TestClusterSharding_UpdateUnchangedClusterSkipsRedistribution(t *testing.T) {
	t.Parallel()
	sharding := setupTestSharding(1, 2)
	cluster := v1alpha1.Cluster{ID: "1", Server: "https://kubernetes.default.svc"}
	sharding.Init(
		&v1alpha1.ClusterList{Items: []v1alpha1.Cluster{cluster}},
		&v1alpha1.ApplicationList{},
	)

	sharding.Shards[cluster.Server] = 42

	sharding.Update(&cluster, &cluster)

	assert.Equal(t, 42, sharding.Shards[cluster.Server], "resync of an unchanged cluster must not recompute the distribution")
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

func TestClusterSharding_IsManagedClusterByServer(t *testing.T) {
	t.Parallel()
	replicas := 2
	shard0, shard1 := int64(0), int64(1)
	clusters := &v1alpha1.ClusterList{
		Items: []v1alpha1.Cluster{
			{ID: "1", Server: "https://kubernetes.default.svc", Shard: &shard0},
			{ID: "2", Server: "https://127.0.0.1:6443", Shard: &shard1},
			{ID: "3", Server: "https://skipped", Shard: &shard0, Annotations: map[string]string{common.AnnotationKeyAppSkipReconcile: "true"}},
			{ID: "4", Server: "https://not-skipped", Shard: &shard0, Annotations: map[string]string{common.AnnotationKeyAppSkipReconcile: "false"}},
		},
	}
	apps := &v1alpha1.ApplicationList{
		Items: []v1alpha1.Application{
			createApp("app1", "https://kubernetes.default.svc"),
			createApp("app2", "https://127.0.0.1:6443"),
		},
	}

	sharding0 := setupTestSharding(0, replicas)
	sharding0.Init(clusters, apps)
	sharding1 := setupTestSharding(1, replicas)
	sharding1.Init(clusters, apps)

	tests := []struct {
		name       string
		server     string
		wantShard0 bool
		wantShard1 bool
		wantKnown  bool
	}{
		{"cluster assigned to shard 0", "https://kubernetes.default.svc", true, false, true},
		{"cluster assigned to shard 1", "https://127.0.0.1:6443", false, true, true},
		{"cluster with skip-reconcile annotation", "https://skipped", false, false, true},
		{"cluster with skip-reconcile annotation set to false", "https://not-skipped", true, false, true},
		// An unknown server reports managed, matching IsManagedCluster(nil), with known=false so
		// callers can fall back to the full lookup.
		{"server the cache holds no cluster for", "https://unknown", true, true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			managed, known := sharding0.IsManagedClusterByServer(tt.server)
			assert.Equal(t, tt.wantShard0, managed, "shard 0 managed")
			assert.Equal(t, tt.wantKnown, known, "shard 0 known")

			managed, known = sharding1.IsManagedClusterByServer(tt.server)
			assert.Equal(t, tt.wantShard1, managed, "shard 1 managed")
			assert.Equal(t, tt.wantKnown, known, "shard 1 known")
		})
	}
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
