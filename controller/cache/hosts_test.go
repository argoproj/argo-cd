package cache

import (
	"errors"
	"maps"
	"runtime"
	"slices"
	"strconv"
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
		// nodeCLate holds node-c back while app-pod is on it; nodeC adds it.
		nodeCLate, nodeC bool
		syncErr          error
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
				if l.appPodNode == "node-c" && !l.nodeCLate || l.nodeC {
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
			hosts:           map[string]*clusterHostsEntry{cluster.Server: {}},
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
		setTTL(t, 0) // an index without Nodes doesn't expire
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

	t.Run("rebuilds once a node the index saw a pod on joins", func(t *testing.T) {
		// The Pod event reached the cache before its Node's.
		l := &live{appPodNode: "node-c", nodeCLate: true}
		c, scans := setup(t, l)
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.NotContains(t, index.Nodes, "node-c")

		nodeC := &cache.Resource{Ref: corev1.ObjectReference{Name: "node-c", Kind: "Node", APIVersion: "v1"}}
		c.dropHostsOnNodeAdded(cluster.Server, nodeC, nodeC) // an update, not an add
		_, _, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(1), scans.Load())

		l.nodeC = true
		c.dropHostsOnNodeAdded(cluster.Server, nodeC, nil)
		_, index, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(2), scans.Load())
		assert.Contains(t, index.Nodes, "node-c")
	})

	t.Run("rebuilds an index without Nodes after a full sync", func(t *testing.T) {
		l := &live{appPodNode: "node-a", noNodes: true}
		c, scans := setup(t, l)
		setTTL(t, 0)
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)

		// A full sync fires no events, so a Node it brings would go unseen.
		resynced := l.syncedAt.Add(time.Minute)
		l.syncedAt = &resynced
		_, _, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(2), scans.Load())
	})

	t.Run("a Node add during a rebuild discards its scan", func(t *testing.T) {
		l := &live{appPodNode: "node-a", noNodes: true}
		c, scans := setup(t, l)
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		setTTL(t, 0)

		// The scan predates node-a, whose add event lands before it publishes.
		l.onScan = func() {
			l.onScan = nil
			c.dropHostsOnNodeAdded(cluster.Server, &cache.Resource{Ref: corev1.ObjectReference{Name: "node-a", Kind: "Node", APIVersion: "v1"}}, nil)
		}
		resynced := l.syncedAt.Add(time.Minute) // expire the index without Nodes
		l.syncedAt = &resynced
		_, _, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Nil(t, c.hosts[cluster.Server].current.Load())

		l.noNodes = false

		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Contains(t, index.Nodes, "node-a")
		assert.Equal(t, int32(3), scans.Load())
	})

	t.Run("keeps the index when an unrelated node joins", func(t *testing.T) {
		c, scans := setup(t, healthy())
		_, first, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)

		c.dropHostsOnNodeAdded(cluster.Server, &cache.Resource{Ref: corev1.ObjectReference{Name: "node-d", Kind: "Node", APIVersion: "v1"}}, nil)
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Same(t, first, index)
		assert.Equal(t, int32(1), scans.Load())
	})

	t.Run("rebuilds once a cluster's first node joins", func(t *testing.T) {
		l := &live{appPodNode: "node-a", noNodes: true}
		c, scans := setup(t, l)
		_, index, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Empty(t, index.Nodes)

		l.noNodes = false
		c.dropHostsOnNodeAdded(cluster.Server, &cache.Resource{Ref: corev1.ObjectReference{Name: "node-a", Kind: "Node", APIVersion: "v1"}}, nil)
		_, index, err = c.GetClusterHosts(cluster, appPods)
		require.NoError(t, err)
		assert.Equal(t, int32(2), scans.Load())
		assert.Contains(t, index.Nodes, "node-a")
	})

	t.Run("drops the index when the cluster cache is invalidated", func(t *testing.T) {
		build := func(t *testing.T, c *liveStateCache) {
			t.Helper()
			_, _, err := c.GetClusterHosts(cluster, appPods)
			require.NoError(t, err)
			require.NotNil(t, c.hosts[cluster.Server].current.Load())
		}
		c, _ := setup(t, healthy())

		build(t, c)
		c.handleModEvent(cluster, &appv1.Cluster{Server: cluster.Server, Namespaces: []string{"default"}})
		assert.Nil(t, c.hosts[cluster.Server].current.Load())

		build(t, c)
		c.invalidate(cacheSettings{})
		assert.Nil(t, c.hosts[cluster.Server].current.Load())

		build(t, c)
		c.handleDeleteEvent(cluster.Server)
		assert.NotContains(t, c.hosts, cluster.Server)
	})
}

func TestNeighborRequests(t *testing.T) {
	cpu := func(q string) corev1.ResourceList {
		return corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(q)}
	}
	running := kube.NewResourceKey("", kube.PodKind, "default", "running")
	done := kube.NewResourceKey("", kube.PodKind, "default", "done")
	other := kube.NewResourceKey("", kube.PodKind, "default", "other")
	h := &ClusterHosts{
		Pods: map[kube.ResourceKey]PodInfo{
			running: {NodeName: "node-a", ResourceRequests: cpu("1")},
			done:    {NodeName: "node-a", ResourceRequests: cpu("8"), Phase: corev1.PodFailed},
			other:   {NodeName: "node-b", ResourceRequests: cpu("4")},
		},
		RequestsByNode: map[string]map[corev1.ResourceName]int64{
			"node-a": {corev1.ResourceCPU: 3000},
			"node-b": {corev1.ResourceCPU: 4000},
		},
	}
	got := h.NeighborRequests([]string{"node-a", "node-c"}, []kube.ResourceKey{running, done, other})
	// done never counted toward node-a; node-b wasn't asked for.
	assert.Equal(t, map[string]map[corev1.ResourceName]int64{
		"node-a": {corev1.ResourceCPU: 2000},
		"node-c": nil,
	}, got)
	assert.Equal(t, int64(3000), h.RequestsByNode["node-a"][corev1.ResourceCPU], "published index is unchanged")
}

// benchClusterCache serves FindResources from maps laid out like the real store's.
type benchClusterCache struct {
	cache.ClusterCache
	resources map[kube.ResourceKey]*cache.Resource
	nsIndex   map[string]map[kube.ResourceKey]*cache.Resource
	syncedAt  time.Time
}

func (b *benchClusterCache) add(res *cache.Resource) {
	key := res.ResourceKey()
	b.resources[key] = res
	if key.Namespace != "" {
		if b.nsIndex[key.Namespace] == nil {
			b.nsIndex[key.Namespace] = map[kube.ResourceKey]*cache.Resource{}
		}
		b.nsIndex[key.Namespace][key] = res
	}
}

func (b *benchClusterCache) EnsureSynced() error { return nil }

func (b *benchClusterCache) GetClusterInfo() cache.ClusterInfo {
	return cache.ClusterInfo{LastCacheSyncTime: &b.syncedAt}
}

func (b *benchClusterCache) FindResources(namespace string, predicates ...func(*cache.Resource) bool) map[kube.ResourceKey]*cache.Resource {
	resources := b.resources
	if namespace != "" {
		resources = b.nsIndex[namespace]
	}
	found := map[kube.ResourceKey]*cache.Resource{}
	for key, res := range resources {
		if predicates[0](res) {
			found[key] = res
		}
	}
	return found
}

// BenchmarkGetClusterHosts: 500 nodes, 80k pods in 200 namespaces, each pod
// with a ReplicaSet, EndpointSlice and ConfigMap beside it; a 20-pod app.
func BenchmarkGetClusterHosts(b *testing.B) {
	const nodes, namespaces, podsPerNamespace, appPodCount = 500, 200, 400, 20
	requests := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("128Mi")}
	clusterCache := &benchClusterCache{
		resources: map[kube.ResourceKey]*cache.Resource{},
		nsIndex:   map[string]map[kube.ResourceKey]*cache.Resource{},
		syncedAt:  time.Now(),
	}
	for i := range nodes {
		name := "node-" + strconv.Itoa(i)
		clusterCache.add(&cache.Resource{
			Ref:  corev1.ObjectReference{Name: name, Kind: "Node", APIVersion: "v1"},
			Info: &ResourceInfo{NodeInfo: &NodeInfo{Name: name, Capacity: requests}},
		})
	}
	var appPods []kube.ResourceKey
	for n := range namespaces {
		ns := "ns-" + strconv.Itoa(n)
		for p := range podsPerNamespace {
			res := &cache.Resource{
				Ref:  corev1.ObjectReference{Name: "pod-" + strconv.Itoa(p), Kind: kube.PodKind, APIVersion: "v1", Namespace: ns},
				Info: &ResourceInfo{PodInfo: &PodInfo{NodeName: "node-" + strconv.Itoa((n*podsPerNamespace+p)%nodes), ResourceRequests: requests}},
			}
			clusterCache.add(res)
			for _, kind := range []struct{ apiVersion, kind string }{
				{"apps/v1", "ReplicaSet"}, {"discovery.k8s.io/v1", "EndpointSlice"}, {"v1", "ConfigMap"},
			} {
				clusterCache.add(&cache.Resource{
					Ref:  corev1.ObjectReference{Name: "pod-" + strconv.Itoa(p), Kind: kind.kind, APIVersion: kind.apiVersion, Namespace: ns},
					Info: &ResourceInfo{},
				})
			}
			if n == 0 && p < appPodCount {
				appPods = append(appPods, res.ResourceKey())
			}
		}
	}
	cluster := &appv1.Cluster{Server: "https://bench"}
	newCache := func() *liveStateCache {
		db := &dbmocks.ArgoDB{}
		db.EXPECT().GetApplicationControllerReplicas().Return(1).Maybe()
		return &liveStateCache{
			clusters:        map[string]cache.ClusterCache{cluster.Server: clusterCache},
			hosts:           map[string]*clusterHostsEntry{cluster.Server: {}},
			clusterSharding: sharding.NewClusterSharding(db, 0, 1, common.DefaultShardingAlgorithm),
		}
	}

	// master's getAppHosts (#28539): IterateResources over the whole cluster,
	// then the neighbors on the app's nodes.
	b.Run("iterate-resources", func(b *testing.B) {
		c := newCache()
		// master's liveStateCache.IterateResources.
		iterateResources := func(server *appv1.Cluster, callback func(res *cache.Resource, info *ResourceInfo)) error {
			clusterInfo, err := c.getSyncedCluster(server)
			if err != nil {
				return err
			}
			_ = clusterInfo.FindResources("", func(r *cache.Resource) bool {
				if info, ok := r.Info.(*ResourceInfo); ok {
					callback(r, info)
				}
				return false
			})
			return nil
		}
		b.ReportAllocs()
		for b.Loop() {
			wanted := map[kube.ResourceKey]bool{}
			for _, key := range appPods {
				wanted[key] = true
			}
			allNodesInfo := map[string]NodeInfo{}
			allPodsByNode := map[string][]PodInfo{}
			appPodsByNode := map[string][]PodInfo{}
			err := iterateResources(cluster, func(res *cache.Resource, info *ResourceInfo) {
				key := res.ResourceKey()
				switch {
				case info.NodeInfo != nil && key.Group == "" && key.Kind == "Node":
					allNodesInfo[key.Name] = *info.NodeInfo
				case info.PodInfo != nil && key.Group == "" && key.Kind == kube.PodKind:
					if wanted[key] {
						appPodsByNode[info.PodInfo.NodeName] = append(appPodsByNode[info.PodInfo.NodeName], *info.PodInfo)
					} else {
						allPodsByNode[info.PodInfo.NodeName] = append(allPodsByNode[info.PodInfo.NodeName], *info.PodInfo)
					}
				}
			})
			require.NoError(b, err)
			neighbors := map[string]map[corev1.ResourceName]int64{}
			for node := range appPodsByNode {
				neighbors[node] = map[corev1.ResourceName]int64{}
				for _, pod := range allPodsByNode[node] {
					if pod.Phase != corev1.PodSucceeded && pod.Phase != corev1.PodFailed {
						for name, quantity := range pod.ResourceRequests {
							neighbors[node][name] += quantity.MilliValue()
						}
					}
				}
			}
		}
	})
	// A reconcile within appHostsIndexTTL: reads the app's pods, reuses the index.
	b.Run("cached", func(b *testing.B) {
		c := newCache()
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(b, err)
		b.ReportAllocs()
		for b.Loop() {
			livePods, index, _ := c.GetClusterHosts(cluster, appPods)
			appPodsByNode := map[string][]PodInfo{}
			for _, pod := range livePods {
				appPodsByNode[pod.NodeName] = append(appPodsByNode[pod.NodeName], pod)
			}
			_ = index.NeighborRequests(slices.Collect(maps.Keys(appPodsByNode)), appPods)
		}
	})
	// The rebuild, at most once per appHostsIndexTTL per cluster. retained-B is
	// the heap the published index holds between rebuilds.
	b.Run("rebuild", func(b *testing.B) {
		c := newCache()
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		_, _, err := c.GetClusterHosts(cluster, appPods)
		require.NoError(b, err)
		runtime.GC()
		runtime.ReadMemStats(&after)
		b.ReportAllocs()
		for b.Loop() {
			c.hosts[cluster.Server].current.Store(nil)
			_, _, _ = c.GetClusterHosts(cluster, appPods)
		}
		// After the loop: b.Loop drops metrics reported before it.
		b.ReportMetric(float64(int64(after.HeapAlloc)-int64(before.HeapAlloc)), "retained-B")
	})
}
