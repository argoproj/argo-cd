package cache

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache/mocks"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/controller/sharding"
	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	dbmocks "github.com/argoproj/argo-cd/v3/util/db/mocks"
)

func TestGetClusterHosts(t *testing.T) {
	cpu := func(q string) corev1.ResourceList {
		return corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(q)}
	}
	cluster := &appv1.Cluster{Server: "https://test"}
	appPod := kube.NewResourceKey("", kube.PodKind, "default", "app-pod")
	appPods := []kube.ResourceKey{appPod}

	// The live cluster: tests change these between calls. node-c only exists
	// once appPodNode points at it, as when the autoscaler adds a node.
	type live struct {
		appPodNode string
		noNodes    bool // namespaced cluster without cluster resources
		newPod     bool // a pod created after the index was built
		syncErr    error
		// onLiveRead runs when the app's pods are read live, onScan when the
		// index is rebuilt.
		onLiveRead, onScan func()
		syncedAt           *time.Time
	}
	setup := func(t *testing.T, l *live) (*liveStateCache, *atomic.Int32) {
		t.Helper()
		resources := func() []*cache.Resource {
			var out []*cache.Resource
			add := func(ref corev1.ObjectReference, info *ResourceInfo) {
				out = append(out, &cache.Resource{Ref: ref, Info: info})
			}
			node := func(name string) {
				add(corev1.ObjectReference{Name: name, Kind: "Node", APIVersion: "v1"},
					&ResourceInfo{NodeInfo: &NodeInfo{Name: name, Capacity: cpu("10")}})
			}
			pod := func(name, nodeName, req string, phase corev1.PodPhase) {
				add(corev1.ObjectReference{Name: name, Kind: kube.PodKind, APIVersion: "v1", Namespace: "default"},
					&ResourceInfo{PodInfo: &PodInfo{NodeName: nodeName, ResourceRequests: cpu(req), Phase: phase}})
			}
			if !l.noNodes {
				node("node-a")
				node("node-b")
				if l.appPodNode == "node-c" {
					node("node-c")
				}
			}
			pod("app-pod", l.appPodNode, "1", "")
			if l.newPod {
				pod("new-pod", "node-a", "1", "")
			}
			pod("neighbor-a", "node-a", "2", "")
			pod("completed-a", "node-a", "8", corev1.PodSucceeded) // finished pods don't count
			pod("neighbor-b", "node-b", "4", "")
			return out
		}
		scans := &atomic.Int32{}
		clusterCache := &mocks.ClusterCache{}
		clusterCache.EXPECT().EnsureSynced().RunAndReturn(func() error { return l.syncErr }).Maybe()
		syncedAt := time.Now()
		if l.syncedAt == nil {
			l.syncedAt = &syncedAt
		}
		clusterCache.EXPECT().GetClusterInfo().RunAndReturn(func() cache.ClusterInfo {
			return cache.ClusterInfo{LastCacheSyncTime: l.syncedAt}
		}).Maybe()
		// Variadic args are unrolled: handleDeleteEvent, handleModEvent and invalidate pass 0, 1 and 4.
		clusterCache.EXPECT().Invalidate().Maybe()
		clusterCache.EXPECT().Invalidate(mock.Anything).Maybe()
		clusterCache.EXPECT().Invalidate(mock.Anything, mock.Anything, mock.Anything, mock.Anything).Maybe()
		clusterCache.EXPECT().FindResources(mock.Anything, mock.Anything).RunAndReturn(
			func(namespace string, predicates ...func(*cache.Resource) bool) map[kube.ResourceKey]*cache.Resource {
				if namespace == "" {
					scans.Add(1)
					if l.onScan != nil {
						l.onScan()
					}
				} else if l.onLiveRead != nil {
					l.onLiveRead()
				}
				found := map[kube.ResourceKey]*cache.Resource{}
				for _, res := range resources() {
					if (namespace == "" || res.Ref.Namespace == namespace) && predicates[0](res) {
						found[res.ResourceKey()] = res
					}
				}
				return found
			}).Maybe()
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetApplicationControllerReplicas().Return(1).Maybe()
		return &liveStateCache{
			clusters:        map[string]cache.ClusterCache{cluster.Server: clusterCache},
			clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
		}, scans
	}
	healthy := func() *live { return &live{appPodNode: "node-a"} }
	setTTL := func(t *testing.T, ttl time.Duration) {
		t.Helper()
		orig := appHostsIndexTTL
		appHostsIndexTTL = ttl
		t.Cleanup(func() { appHostsIndexTTL = orig })
	}

	t.Run("indexes nodes and the requests of running pods", func(t *testing.T) {
		c, _ := setup(t, healthy())
		livePods, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, "node-a", livePods[appPod].NodeName)
		assert.Len(t, index.Nodes, 2)
		assert.Len(t, index.Pods, 4)
		assert.Equal(t, int64(3000), index.RequestsByNode["node-a"][corev1.ResourceCPU])
	})

	t.Run("reuses the index until appHostsIndexTTL", func(t *testing.T) {
		c, scans := setup(t, healthy())
		for range 3 {
			_, _, err := c.GetClusterHosts(cluster, appPods)
			require.NoError(t, err)
		}
		assert.Equal(t, int32(1), scans.Load())

		setTTL(t, 0)
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(2), scans.Load())
	})

	t.Run("rebuilds at once for a node the index hasn't seen", func(t *testing.T) {
		l := healthy()
		c, scans := setup(t, l)
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		l.appPodNode = "node-c"
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(2), scans.Load())
		assert.Contains(t, index.Nodes, "node-c")
	})

	t.Run("does not rebuild for nodes known to be absent", func(t *testing.T) {
		l := &live{appPodNode: "node-a", noNodes: true}
		c, scans := setup(t, l)
		for range 3 {
			_, index, err := c.GetClusterHosts(cluster, appPods)
			require.NoError(t, err)
			assert.Empty(t, index.Nodes)
		}
		// Nor for new pods, when the cluster caches no Nodes at all.
		l.newPod = true
		_, _, err := c.GetClusterHosts(cluster, append(appPods, kube.NewResourceKey("", kube.PodKind, "default", "new-pod")))
		require.NoError(t, err)
		assert.Equal(t, int32(1), scans.Load())
	})

	t.Run("skips the index while the pods are unscheduled", func(t *testing.T) {
		c, scans := setup(t, &live{})
		livePods, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Contains(t, livePods, appPod)
		assert.Nil(t, index)
		assert.Equal(t, int32(0), scans.Load())
	})

	t.Run("serves the expired index while another reconcile rebuilds it", func(t *testing.T) {
		c, scans := setup(t, healthy())
		_, first, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		setTTL(t, 0)
		entry := c.hosts[cluster.Server]
		entry.rebuild.Lock()
		defer entry.rebuild.Unlock()
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Same(t, first, index)
		assert.Equal(t, int32(1), scans.Load())
	})

	t.Run("a waiter takes the index rebuilt while it waited", func(t *testing.T) {
		l := healthy()
		c, scans := setup(t, l)
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		entry := c.hosts[cluster.Server]
		entry.rebuild.Lock()

		// The app's pod moves to node-c, which the index lacks, so the
		// reconcile below waits for the rebuild this test holds.
		l.appPodNode = "node-c"
		read := make(chan struct{})
		l.onLiveRead = func() { close(read) }
		got := make(chan *ClusterHosts)
		go func() {
			_, index, err := c.GetClusterHosts(cluster, appPods)
			assert.NoError(t, err)
			got <- index
		}()
		<-read
		time.Sleep(50 * time.Millisecond) // let it reach entry.rebuild.Lock
		rebuilt := &ClusterHosts{BuiltAt: time.Now(), Nodes: map[string]NodeInfo{"node-c": {}}}
		entry.current.Store(rebuilt)
		entry.rebuild.Unlock()

		assert.Same(t, rebuilt, <-got)
		assert.Equal(t, int32(1), scans.Load())
	})

	t.Run("readers keep their snapshot while it is rebuilt", func(t *testing.T) {
		c, _ := setup(t, healthy())
		setTTL(t, 0) // every call rebuilds or serves the previous index
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 20 {
					_, index, err := c.GetClusterHosts(cluster, appPods)
					if assert.NoError(t, err) {
						assert.Contains(t, index.Nodes, "node-a")
					}
				}
			})
		}
		wg.Wait()
	})

	t.Run("does not cache a failed sync", func(t *testing.T) {
		l := healthy()
		l.syncErr = errors.New("cluster not synced")
		c, scans := setup(t, l)
		for range 2 {
			_, _, err := c.GetClusterHosts(cluster, appPods)
			require.Error(t, err)
		}
		assert.Equal(t, int32(0), scans.Load())
	})

	t.Run("does not cache an index for a cluster removed mid-call", func(t *testing.T) {
		l := healthy()
		c, _ := setup(t, l)
		// handleDeleteEvent runs after this call got the cluster cache.
		l.onLiveRead = func() { c.handleDeleteEvent(cluster.Server) }
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Nil(t, index)
		assert.NotContains(t, c.hosts, cluster.Server)
	})

	t.Run("does not publish a scan that raced an invalidation", func(t *testing.T) {
		l := healthy()
		c, scans := setup(t, l)
		l.onScan = func() { l.syncedAt = nil } // Invalidate clears the sync time
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.NotNil(t, index, "the caller still gets its scan")
		assert.Nil(t, c.hosts[cluster.Server].current.Load())

		resynced := time.Now()
		l.onScan = func() { l.syncedAt = &resynced }
		_, _, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(2), scans.Load())
		assert.Nil(t, c.hosts[cluster.Server].current.Load(), "sync time changed during the scan")

		l.onScan = nil
		_, _, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.NotNil(t, c.hosts[cluster.Server].current.Load())
	})

	t.Run("drops the index when the cluster cache is invalidated", func(t *testing.T) {
		build := func(t *testing.T, c *liveStateCache) {
			t.Helper()
			_, _, err := c.GetClusterHosts(cluster, appPods)
			require.NoError(t, err)
			require.Contains(t, c.hosts, cluster.Server)
		}
		c, _ := setup(t, healthy())

		build(t, c)
		c.handleModEvent(cluster, &appv1.Cluster{Server: cluster.Server, Namespaces: []string{"default"}})
		assert.NotContains(t, c.hosts, cluster.Server)

		build(t, c)
		c.invalidate(cacheSettings{})
		assert.NotContains(t, c.hosts, cluster.Server)

		build(t, c)
		c.handleDeleteEvent(cluster.Server)
		assert.NotContains(t, c.hosts, cluster.Server)
	})
}
