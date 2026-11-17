package cache

import (
	"maps"
	"sync"
	"sync/atomic"
	"time"

	clustercache "github.com/argoproj/argo-cd/gitops-engine/v3/pkg/cache"
	"github.com/argoproj/argo-cd/gitops-engine/v3/pkg/utils/kube"
	corev1 "k8s.io/api/core/v1"

	appv1 "github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
)

// ClusterHosts is a cluster's Nodes and Pods as of BuiltAt. It is never
// modified after it is published.
type ClusterHosts struct {
	BuiltAt time.Time
	Nodes   map[string]NodeInfo
	Pods    map[kube.ResourceKey]PodInfo
	// RequestsByNode sums, in milli-units, the requests of each node's pods
	// that haven't succeeded or failed. Every node a pod is on has an entry.
	RequestsByNode map[string]map[corev1.ResourceName]int64

	syncedAt time.Time // the cluster cache's LastCacheSyncTime at build
}

// NeighborRequests returns RequestsByNode for each of nodes, less the requests
// of pods.
func (h *ClusterHosts) NeighborRequests(nodes []string, pods []kube.ResourceKey) map[string]map[corev1.ResourceName]int64 {
	out := make(map[string]map[corev1.ResourceName]int64, len(nodes))
	for _, node := range nodes {
		if _, ok := h.Nodes[node]; ok {
			out[node] = maps.Clone(h.RequestsByNode[node])
		}
	}
	seen := make(map[kube.ResourceKey]bool, len(pods))
	for _, key := range pods {
		pod, ok := h.Pods[key]
		// Succeeded and failed pods aren't in RequestsByNode.
		if !ok || seen[key] || pod.Phase == corev1.PodSucceeded || pod.Phase == corev1.PodFailed || out[pod.NodeName] == nil {
			continue
		}
		seen[key] = true
		for name, quantity := range pod.ResourceRequests {
			out[pod.NodeName][name] -= quantity.MilliValue()
		}
	}
	return out
}

type clusterHostsEntry struct {
	current atomic.Pointer[ClusterHosts]
	// rebuild serializes rebuilds, so concurrent reconciles scan the cluster once.
	rebuild sync.Mutex
}

// appHostsIndexTTL is how long GetClusterHosts reuses a cluster's index.
var appHostsIndexTTL = 10 * time.Second

// GetClusterHosts reads pods live and returns them with server's Node/Pod
// index, at most appHostsIndexTTL old, so reconciles don't each walk the whole
// cluster cache (#28539). The index is nil if none of pods is scheduled.
func (c *liveStateCache) GetClusterHosts(server *appv1.Cluster, pods []kube.ResourceKey) (map[kube.ResourceKey]PodInfo, *ClusterHosts, error) {
	clusterCache, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, nil, err
	}

	wanted := map[kube.ResourceKey]bool{}
	namespaces := map[string]bool{}
	for _, key := range pods {
		wanted[key] = true
		namespaces[key.Namespace] = true
	}
	live := map[kube.ResourceKey]PodInfo{}
	scheduled := false
	for ns := range namespaces {
		for key, res := range clusterCache.FindResources(ns, func(r *clustercache.Resource) bool { return wanted[r.ResourceKey()] }) {
			if info, ok := res.Info.(*ResourceInfo); ok && info.PodInfo != nil {
				live[key] = *info.PodInfo
				scheduled = scheduled || info.PodInfo.NodeName != ""
			}
		}
	}
	if !scheduled {
		return live, nil, nil
	}

	c.lock.RLock()
	entry := c.hosts[server.Server]
	c.lock.RUnlock()
	if entry == nil { // removed since getSyncedCluster
		return live, nil, nil
	}

	// An index covers the live pods if it has each one's node, or already saw
	// the pod on that node without it. A pod on a node the index has never
	// heard of forces a rebuild. An index without Nodes covers everything, as
	// namespaced clusters never cache them. dropHostsOnNodeAdded resets the
	// index in both cases, so the Node shows up on the next call.
	covers := func(h *ClusterHosts) bool {
		if len(h.Nodes) == 0 {
			return true
		}
		for key, pod := range live {
			if pod.NodeName == "" {
				continue
			}
			if _, ok := h.Nodes[pod.NodeName]; ok {
				continue
			}
			if indexed, ok := h.Pods[key]; !ok || indexed.NodeName != pod.NodeName {
				return false
			}
		}
		return true
	}
	current := entry.current.Load()
	if current != nil && covers(current) {
		fresh := time.Since(current.BuiltAt) < appHostsIndexTTL
		if !fresh && len(current.Nodes) == 0 {
			// Without Nodes there are no hosts to refresh until a Node add resets
			// the index, or a full sync, which fires no events, adds one.
			synced := clusterCache.GetClusterInfo().LastCacheSyncTime
			fresh = synced != nil && synced.Equal(current.syncedAt)
		}
		if fresh {
			return live, current, nil
		}
		// Serve the expired index while another reconcile rebuilds it.
		if !entry.rebuild.TryLock() {
			return live, current, nil
		}
	} else {
		entry.rebuild.Lock()
	}
	defer entry.rebuild.Unlock()
	// Another reconcile may have rebuilt it while this one waited.
	if latest := entry.current.Load(); latest != current && covers(latest) {
		return live, latest, nil
	}

	syncedAt := clusterCache.GetClusterInfo().LastCacheSyncTime
	next := &ClusterHosts{
		BuiltAt:        time.Now(),
		Nodes:          map[string]NodeInfo{},
		Pods:           map[kube.ResourceKey]PodInfo{},
		RequestsByNode: map[string]map[corev1.ResourceName]int64{},
	}
	_ = clusterCache.FindResources("", func(r *clustercache.Resource) bool {
		info, ok := r.Info.(*ResourceInfo)
		if !ok {
			return false
		}
		key := r.ResourceKey()
		switch {
		case info.NodeInfo != nil && key.Group == "" && key.Kind == "Node":
			next.Nodes[key.Name] = *info.NodeInfo
		case info.PodInfo != nil && key.Group == "" && key.Kind == kube.PodKind:
			pod := *info.PodInfo
			next.Pods[key] = pod
			if next.RequestsByNode[pod.NodeName] == nil {
				next.RequestsByNode[pod.NodeName] = map[corev1.ResourceName]int64{}
			}
			if pod.Phase == corev1.PodSucceeded || pod.Phase == corev1.PodFailed {
				return false
			}
			for name, quantity := range pod.ResourceRequests {
				next.RequestsByNode[pod.NodeName][name] += quantity.MilliValue()
			}
		}
		return false
	})
	if len(next.Nodes) == 0 {
		// No hosts without Nodes; covers and dropHostsOnNodeAdded need no Pods.
		next.Pods, next.RequestsByNode = nil, nil
	}
	// Invalidate clears the sync time, and the index is reset only after
	// Invalidate; don't publish a scan that raced it (old settings).
	if after := clusterCache.GetClusterInfo().LastCacheSyncTime; syncedAt != nil && after != nil && syncedAt.Equal(*after) {
		next.syncedAt = *syncedAt
		entry.current.Store(next)
	}
	return live, next, nil
}

// dropHostsOnNodeAdded resets server's index when a Node is added that the
// index may hold wrongly: it has no Nodes, saw a pod on that node, holds a
// same-named Node since deleted, or is being rebuilt. covers already rebuilds
// for pods on other new nodes, so autoscaling doesn't reset it.
func (c *liveStateCache) dropHostsOnNodeAdded(server string, newRes, oldRes *clustercache.Resource) {
	if oldRes != nil || newRes == nil {
		return
	}
	key := newRes.ResourceKey()
	if key.Group != "" || key.Kind != "Node" {
		return
	}
	c.lock.RLock()
	entry := c.hosts[server]
	c.lock.RUnlock()
	if entry == nil {
		return
	}
	// A rebuild in progress may have scanned before this Node was cached.
	if entry.rebuild.TryLock() {
		h := entry.current.Load()
		entry.rebuild.Unlock()
		if h != nil && len(h.Nodes) > 0 && h.RequestsByNode[key.Name] == nil {
			if _, known := h.Nodes[key.Name]; !known {
				return
			}
		}
	}
	c.lock.Lock()
	// A fresh entry, so a rebuild already scanning publishes to the old one.
	if c.hosts[server] == entry {
		c.hosts[server] = &clusterHostsEntry{}
	}
	c.lock.Unlock()
}
