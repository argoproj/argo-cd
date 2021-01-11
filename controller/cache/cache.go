package cache

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	logutils "github.com/argoproj/argo-cd/util/log"
	"github.com/argoproj/argo-cd/util/lua"
	"github.com/argoproj/argo-cd/util/settings"
)

type LiveStateCache interface {
	// Returns k8s server version
	GetVersionsInfo(serverURL string) (string, []metav1.APIGroup, error)
	// Returns true of given group kind is a namespaced resource
	IsNamespaced(server string, gk schema.GroupKind) (bool, error)
	// Returns synced cluster cache
	GetClusterCache(server string) (clustercache.ClusterCache, error)
	// Executes give callback against resource specified by the key and all its children
	IterateHierarchy(server string, key kube.ResourceKey, action func(child appv1.ResourceNode, appName string)) error
	// Returns state of live nodes which correspond for target nodes of specified application.
	GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// IterateResources iterates all resource stored in cache
	IterateResources(server string, callback func(res *clustercache.Resource, info *ResourceInfo)) error
	// Returns all top level resources (resources without owner references) of a specified namespace
	GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error)
	// Starts watching resources of each controlled cluster.
	Run(ctx context.Context) error
	// Returns information about monitored clusters
	GetClustersInfo() []clustercache.ClusterInfo
	// Init must be executed before cache can be used
	Init() error
}

type ObjectUpdatedHandler = func(managedByApp map[string]bool, ref v1.ObjectReference)

type PodInfo struct {
	NodeName         string
	ResourceRequests v1.ResourceList
}

type NodeInfo struct {
	Name       string
	Capacity   v1.ResourceList
	SystemInfo v1.NodeSystemInfo
}

type ResourceInfo struct {
	Info    []appv1.InfoItem
	AppName string
	Images  []string
	Health  *health.HealthStatus
	// NetworkingInfo are available only for known types involved into networking: Ingress, Service, Pod
	NetworkingInfo *appv1.ResourceNetworkingInfo
	// PodInfo is available for pods only
	PodInfo *PodInfo
	// NodeInfo is available for nodes only
	NodeInfo *NodeInfo
}

func NewLiveStateCache(
	db db.ArgoDB,
	appInformer cache.SharedIndexInformer,
	settingsMgr *settings.SettingsManager,
	kubectl kube.Kubectl,
	metricsServer *metrics.MetricsServer,
	onObjectUpdated ObjectUpdatedHandler,
	clusterFilter func(cluster *appv1.Cluster) bool) LiveStateCache {

	return &liveStateCache{
		appInformer:     appInformer,
		db:              db,
		clusters:        make(map[string]clustercache.ClusterCache),
		onObjectUpdated: onObjectUpdated,
		kubectl:         kubectl,
		settingsMgr:     settingsMgr,
		metricsServer:   metricsServer,
		// The default limit of 50 is chosen based on experiments.
		listSemaphore: semaphore.NewWeighted(50),
		clusterFilter: clusterFilter,
	}
}

type cacheSettings struct {
	clusterSettings     clustercache.Settings
	appInstanceLabelKey string
}

type liveStateCache struct {
	db              db.ArgoDB
	appInformer     cache.SharedIndexInformer
	onObjectUpdated ObjectUpdatedHandler
	kubectl         kube.Kubectl
	settingsMgr     *settings.SettingsManager
	metricsServer   *metrics.MetricsServer
	clusterFilter   func(cluster *appv1.Cluster) bool

	// listSemaphore is used to limit the number of concurrent memory consuming operations on the
	// k8s list queries results across all clusters to avoid memory spikes during cache initialization.
	listSemaphore *semaphore.Weighted

	clusters      map[string]clustercache.ClusterCache
	cacheSettings cacheSettings
	lock          sync.RWMutex
}

func (c *liveStateCache) loadCacheSettings() (*cacheSettings, error) {
	appInstanceLabelKey, err := c.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, err
	}
	resourcesFilter, err := c.settingsMgr.GetResourcesFilter()
	if err != nil {
		return nil, err
	}
	resourceOverrides, err := c.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, err
	}
	clusterSettings := clustercache.Settings{
		ResourceHealthOverride: lua.ResourceHealthOverrides(resourceOverrides),
		ResourcesFilter:        resourcesFilter,
	}
	return &cacheSettings{clusterSettings, appInstanceLabelKey}, nil
}

func asResourceNode(r *clustercache.Resource) appv1.ResourceNode {
	gv, err := schema.ParseGroupVersion(r.Ref.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	parentRefs := make([]appv1.ResourceRef, len(r.OwnerRefs))
	for _, ownerRef := range r.OwnerRefs {
		ownerGvk := schema.FromAPIVersionAndKind(ownerRef.APIVersion, ownerRef.Kind)
		ownerKey := kube.NewResourceKey(ownerGvk.Group, ownerRef.Kind, r.Ref.Namespace, ownerRef.Name)
		parentRefs[0] = appv1.ResourceRef{Name: ownerRef.Name, Kind: ownerKey.Kind, Namespace: r.Ref.Namespace, Group: ownerKey.Group, UID: string(ownerRef.UID)}
	}
	var resHealth *appv1.HealthStatus
	resourceInfo := resInfo(r)
	if resourceInfo.Health != nil {
		resHealth = &appv1.HealthStatus{Status: resourceInfo.Health.Status, Message: resourceInfo.Health.Message}
	}
	return appv1.ResourceNode{
		ResourceRef: appv1.ResourceRef{
			UID:       string(r.Ref.UID),
			Name:      r.Ref.Name,
			Group:     gv.Group,
			Version:   gv.Version,
			Kind:      r.Ref.Kind,
			Namespace: r.Ref.Namespace,
		},
		ParentRefs:      parentRefs,
		Info:            resourceInfo.Info,
		ResourceVersion: r.ResourceVersion,
		NetworkingInfo:  resourceInfo.NetworkingInfo,
		Images:          resourceInfo.Images,
		Health:          resHealth,
		CreatedAt:       r.CreationTimestamp,
	}
}

func resInfo(r *clustercache.Resource) *ResourceInfo {
	info, ok := r.Info.(*ResourceInfo)
	if !ok || info == nil {
		info = &ResourceInfo{}
	}
	return info
}

func isRootAppNode(r *clustercache.Resource) bool {
	return resInfo(r).AppName != "" && len(r.OwnerRefs) == 0
}

func getApp(r *clustercache.Resource, ns map[kube.ResourceKey]*clustercache.Resource) string {
	return getAppRecursive(r, ns, map[kube.ResourceKey]bool{})
}

func ownerRefGV(ownerRef metav1.OwnerReference) schema.GroupVersion {
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return gv
}

func getAppRecursive(r *clustercache.Resource, ns map[kube.ResourceKey]*clustercache.Resource, visited map[kube.ResourceKey]bool) string {
	if !visited[r.ResourceKey()] {
		visited[r.ResourceKey()] = true
	} else {
		log.Warnf("Circular dependency detected: %v.", visited)
		return resInfo(r).AppName
	}

	if resInfo(r).AppName != "" {
		return resInfo(r).AppName
	}
	for _, ownerRef := range r.OwnerRefs {
		gv := ownerRefGV(ownerRef)
		if parent, ok := ns[kube.NewResourceKey(gv.Group, ownerRef.Kind, r.Ref.Namespace, ownerRef.Name)]; ok {
			app := getAppRecursive(parent, ns, visited)
			if app != "" {
				return app
			}
		}
	}
	return ""
}

var (
	ignoredRefreshResources = map[string]bool{
		"/" + kube.EndpointsKind: true,
	}
)

// skipAppRequeuing checks if the object is an API type which we want to skip requeuing against.
// We ignore API types which have a high churn rate, and/or whose updates are irrelevant to the app
func skipAppRequeuing(key kube.ResourceKey) bool {
	return ignoredRefreshResources[key.Group+"/"+key.Kind]
}

func (c *liveStateCache) getCluster(server string) (clustercache.ClusterCache, error) {
	c.lock.RLock()
	clusterCache, ok := c.clusters[server]
	cacheSettings := c.cacheSettings
	c.lock.RUnlock()

	if ok {
		return clusterCache, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	clusterCache, ok = c.clusters[server]
	if ok {
		return clusterCache, nil
	}

	cluster, err := c.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, err
	}

	if !c.canHandleCluster(cluster) {
		return nil, fmt.Errorf("controller is configured to ignore cluster %s", cluster.Server)
	}

	clusterCache = clustercache.NewClusterCache(cluster.RESTConfig(),
		clustercache.SetListSemaphore(c.listSemaphore),
		clustercache.SetResyncTimeout(common.K8SClusterResyncDuration),
		clustercache.SetSettings(cacheSettings.clusterSettings),
		clustercache.SetNamespaces(cluster.Namespaces),
		clustercache.SetPopulateResourceInfoHandler(func(un *unstructured.Unstructured, isRoot bool) (interface{}, bool) {
			res := &ResourceInfo{}
			populateNodeInfo(un, res)
			res.Health, _ = health.GetResourceHealth(un, cacheSettings.clusterSettings.ResourceHealthOverride)
			appName := kube.GetAppInstanceLabel(un, cacheSettings.appInstanceLabelKey)
			if isRoot && appName != "" {
				res.AppName = appName
			}
			gvk := un.GroupVersionKind()

			// edge case. we do not label CRDs, so they miss the tracking label we inject. But we still
			// want the full resource to be available in our cache (to diff), so we store all CRDs
			return res, res.AppName != "" || gvk.Kind == kube.CustomResourceDefinitionKind
		}),
		clustercache.SetLogr(logutils.NewLogrusLogger(log.WithField("server", cluster.Server))),
	)

	_ = clusterCache.OnResourceUpdated(func(newRes *clustercache.Resource, oldRes *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) {
		toNotify := make(map[string]bool)
		var ref v1.ObjectReference
		if newRes != nil {
			ref = newRes.Ref
		} else {
			ref = oldRes.Ref
		}
		for _, r := range []*clustercache.Resource{newRes, oldRes} {
			if r == nil {
				continue
			}
			app := getApp(r, namespaceResources)
			if app == "" || skipAppRequeuing(r.ResourceKey()) {
				continue
			}
			toNotify[app] = isRootAppNode(r) || toNotify[app]
		}
		c.onObjectUpdated(toNotify, ref)
	})

	_ = clusterCache.OnEvent(func(event watch.EventType, un *unstructured.Unstructured) {
		gvk := un.GroupVersionKind()
		c.metricsServer.IncClusterEventsCount(cluster.Server, gvk.Group, gvk.Kind)
	})

	c.clusters[cluster.Server] = clusterCache

	return clusterCache, nil
}

func (c *liveStateCache) getSyncedCluster(server string) (clustercache.ClusterCache, error) {
	clusterCache, err := c.getCluster(server)
	if err != nil {
		return nil, err
	}
	err = clusterCache.EnsureSynced()
	if err != nil {
		return nil, err
	}
	return clusterCache, nil
}

func (c *liveStateCache) invalidate(cacheSettings cacheSettings) {
	log.Info("invalidating live state cache")
	c.lock.Lock()
	defer c.lock.Unlock()

	c.cacheSettings = cacheSettings
	for _, clust := range c.clusters {
		clust.Invalidate(clustercache.SetSettings(cacheSettings.clusterSettings))
	}
	log.Info("live state cache invalidated")
}

func (c *liveStateCache) IsNamespaced(server string, gk schema.GroupKind) (bool, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return false, err
	}
	return clusterInfo.IsNamespaced(gk)
}

func (c *liveStateCache) IterateHierarchy(server string, key kube.ResourceKey, action func(child appv1.ResourceNode, appName string)) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	clusterInfo.IterateHierarchy(key, func(resource *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) {
		action(asResourceNode(resource), getApp(resource, namespaceResources))
	})
	return nil
}

func (c *liveStateCache) IterateResources(server string, callback func(res *clustercache.Resource, info *ResourceInfo)) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	_ = clusterInfo.FindResources("", func(r *clustercache.Resource) bool {
		if info, ok := r.Info.(*ResourceInfo); ok {
			callback(r, info)
		}
		return false
	})
	return nil
}

func (c *liveStateCache) GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, err
	}
	resources := clusterInfo.FindResources(namespace, clustercache.TopLevelResource)
	res := make(map[kube.ResourceKey]appv1.ResourceNode)
	for k, r := range resources {
		res[k] = asResourceNode(r)
	}
	return res, nil
}

func (c *liveStateCache) GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getSyncedCluster(a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.GetManagedLiveObjs(targetObjs, func(r *clustercache.Resource) bool {
		return resInfo(r).AppName == a.Name
	})
}

func (c *liveStateCache) GetVersionsInfo(serverURL string) (string, []metav1.APIGroup, error) {
	clusterInfo, err := c.getSyncedCluster(serverURL)
	if err != nil {
		return "", nil, err
	}
	return clusterInfo.GetServerVersion(), clusterInfo.GetAPIGroups(), nil
}

func (c *liveStateCache) isClusterHasApps(apps []interface{}, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		app, ok := obj.(*appv1.Application)
		if !ok {
			continue
		}
		err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, c.db)
		if err != nil {
			continue
		}
		if app.Spec.Destination.Server == cluster.Server {
			return true
		}
	}
	return false
}

func (c *liveStateCache) watchSettings(ctx context.Context) {
	updateCh := make(chan *settings.ArgoCDSettings, 1)
	c.settingsMgr.Subscribe(updateCh)

	done := false
	for !done {
		select {
		case <-updateCh:
			nextCacheSettings, err := c.loadCacheSettings()
			if err != nil {
				log.Warnf("Failed to read updated settings: %v", err)
				continue
			}

			c.lock.Lock()
			needInvalidate := false
			if !reflect.DeepEqual(c.cacheSettings, *nextCacheSettings) {
				c.cacheSettings = *nextCacheSettings
				needInvalidate = true
			}
			c.lock.Unlock()
			if needInvalidate {
				c.invalidate(*nextCacheSettings)
			}
		case <-ctx.Done():
			done = true
		}
	}
	log.Info("shutting down settings watch")
	c.settingsMgr.Unsubscribe(updateCh)
	close(updateCh)
}

func (c *liveStateCache) Init() error {
	cacheSettings, err := c.loadCacheSettings()
	if err != nil {
		return err
	}
	c.cacheSettings = *cacheSettings
	return nil
}

// Run watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (c *liveStateCache) Run(ctx context.Context) error {
	go c.watchSettings(ctx)

	kube.RetryUntilSucceed(ctx, clustercache.ClusterRetryTimeout, "watch clusters", logutils.NewLogrusLogger(log.New()), func() error {
		return c.db.WatchClusters(ctx, c.handleAddEvent, c.handleModEvent, c.handleDeleteEvent)
	})

	<-ctx.Done()
	c.invalidate(c.cacheSettings)
	return nil
}

func (c *liveStateCache) canHandleCluster(cluster *appv1.Cluster) bool {
	if c.clusterFilter == nil {
		return true
	}
	return c.clusterFilter(cluster)
}

func (c *liveStateCache) handleAddEvent(cluster *appv1.Cluster) {
	if !c.canHandleCluster(cluster) {
		log.Infof("Ignoring cluster %s", cluster.Server)
		return
	}

	c.lock.Lock()
	_, ok := c.clusters[cluster.Server]
	c.lock.Unlock()
	if !ok {
		if c.isClusterHasApps(c.appInformer.GetStore().List(), cluster) {
			go func() {
				// warm up cache for cluster with apps
				_, _ = c.getSyncedCluster(cluster.Server)
			}()
		}
	}
}

func (c *liveStateCache) handleModEvent(oldCluster *appv1.Cluster, newCluster *appv1.Cluster) {
	c.lock.Lock()
	cluster, ok := c.clusters[newCluster.Server]
	c.lock.Unlock()
	if ok {
		if !c.canHandleCluster(newCluster) {
			cluster.Invalidate()
			c.lock.Lock()
			delete(c.clusters, newCluster.Server)
			c.lock.Unlock()
			return
		}

		var updateSettings []clustercache.UpdateSettingsFunc
		if !reflect.DeepEqual(oldCluster.Config, newCluster.Config) {
			updateSettings = append(updateSettings, clustercache.SetConfig(newCluster.RESTConfig()))
		}
		if !reflect.DeepEqual(oldCluster.Namespaces, newCluster.Namespaces) {
			updateSettings = append(updateSettings, clustercache.SetNamespaces(newCluster.Namespaces))
		}
		forceInvalidate := false
		if newCluster.RefreshRequestedAt != nil &&
			cluster.GetClusterInfo().LastCacheSyncTime != nil &&
			cluster.GetClusterInfo().LastCacheSyncTime.Before(newCluster.RefreshRequestedAt.Time) {
			forceInvalidate = true
		}

		if len(updateSettings) > 0 || forceInvalidate {
			cluster.Invalidate(updateSettings...)
			go func() {
				// warm up cluster cache
				_ = cluster.EnsureSynced()
			}()
		}
	}

}

func (c *liveStateCache) handleDeleteEvent(clusterServer string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	cluster, ok := c.clusters[clusterServer]
	if ok {
		cluster.Invalidate()
		delete(c.clusters, clusterServer)
	}
}

func (c *liveStateCache) GetClustersInfo() []clustercache.ClusterInfo {
	clusters := make(map[string]clustercache.ClusterCache)
	c.lock.RLock()
	for k := range c.clusters {
		clusters[k] = c.clusters[k]
	}
	c.lock.RUnlock()

	res := make([]clustercache.ClusterInfo, 0)
	for server, c := range clusters {
		info := c.GetClusterInfo()
		info.Server = server
		res = append(res, info)
	}
	return res
}

func (c *liveStateCache) GetClusterCache(server string) (clustercache.ClusterCache, error) {
	return c.getSyncedCluster(server)
}
