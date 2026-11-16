package cache

import (
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
	// that haven't succeeded or failed.
	RequestsByNode map[string]map[corev1.ResourceName]int64
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

	c.hostsLock.Lock()
	if c.hosts == nil {
		c.hosts = map[string]*clusterHostsEntry{}
	}
	entry := c.hosts[server.Server]
	if entry == nil {
		// Checked under hostsLock: removals delete the cluster before its
		// entry, so an entry created for a removed cluster would never be dropped.
		c.lock.RLock()
		_, managed := c.clusters[server.Server]
		c.lock.RUnlock()
		if !managed {
			c.hostsLock.Unlock()
			return live, nil, nil
		}
		entry = &clusterHostsEntry{}
		c.hosts[server.Server] = entry
	}
	c.hostsLock.Unlock()

	// An index covers the live pods if it has each one's node, or already saw
	// the pod on that node without it. A pod on a node the index has never
	// heard of forces a rebuild. An index without Nodes covers everything, as
	// namespaced clusters never cache them. Either way a Node joining drops the
	// index (dropHostsOnNodeAdded), so it shows up on the next call.
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
		if time.Since(current.BuiltAt) < appHostsIndexTTL {
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
			if pod.Phase == corev1.PodSucceeded || pod.Phase == corev1.PodFailed {
				return false
			}
			if next.RequestsByNode[pod.NodeName] == nil {
				next.RequestsByNode[pod.NodeName] = map[corev1.ResourceName]int64{}
			}
			for name, quantity := range pod.ResourceRequests {
				next.RequestsByNode[pod.NodeName][name] += quantity.MilliValue()
			}
		}
		return false
	})
	// Invalidate clears the sync time, and handleModEvent drops the entry only
	// after Invalidate; don't publish a scan that raced it (old settings).
	if after := clusterCache.GetClusterInfo().LastCacheSyncTime; syncedAt != nil && after != nil && syncedAt.Equal(*after) {
		entry.current.Store(next)
	}
	return live, next, nil
}

// dropHostsOnNodeAdded drops server's index when a Node is added, since an
// index may treat a node it hasn't seen yet as absent.
func (c *liveStateCache) dropHostsOnNodeAdded(server string, newRes, oldRes *clustercache.Resource) {
	if oldRes != nil || newRes == nil {
		return
	}
	if key := newRes.ResourceKey(); key.Group != "" || key.Kind != "Node" {
		return
	}
	c.hostsLock.Lock()
	delete(c.hosts, server)
	c.hostsLock.Unlock()
}
