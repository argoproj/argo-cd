package cache

import (
	"context"
	"reflect"
	"sync"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/engine/pkg/utils/health"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	clustercache "github.com/argoproj/argo-cd/engine/pkg/utils/kube/cache"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/lua"
	"github.com/argoproj/argo-cd/util/settings"
)

type LiveStateCache interface {
	// Returns k8s server version
	GetVersionsInfo(serverURL string) (string, []metav1.APIGroup, error)
	// Returns true of given group kind is a namespaced resource
	IsNamespaced(server string, gk schema.GroupKind) (bool, error)
	// Executes give callback against resource specified by the key and all its children
	IterateHierarchy(server string, key kube.ResourceKey, action func(child appv1.ResourceNode, appName string)) error
	// Returns state of live nodes which correspond for target nodes of specified application.
	GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// Returns all top level resources (resources without owner references) of a specified namespace
	GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error)
	// Starts watching resources of each controlled cluster.
	Run(ctx context.Context) error
	// Returns information about monitored clusters
	GetClustersInfo() []clustercache.ClusterInfo
}

type ObjectUpdatedHandler = func(managedByApp map[string]bool, ref v1.ObjectReference)

type ResourceInfo struct {
	Info    []appv1.InfoItem
	AppName string
	// networkingInfo are available only for known types involved into networking: Ingress, Service, Pod
	NetworkingInfo *appv1.ResourceNetworkingInfo
	Images         []string
	Health         *health.HealthStatus
}

func NewLiveStateCache(
	db db.ArgoDB,
	appInformer cache.SharedIndexInformer,
	settingsMgr *settings.SettingsManager,
	kubectl kube.Kubectl,
	metricsServer *metrics.MetricsServer,
	onObjectUpdated ObjectUpdatedHandler) LiveStateCache {

	return &liveStateCache{
		appInformer:     appInformer,
		db:              db,
		clusters:        make(map[string]clustercache.ClusterCache),
		onObjectUpdated: onObjectUpdated,
		kubectl:         kubectl,
		settingsMgr:     settingsMgr,
		metricsServer:   metricsServer,
	}
}

type liveStateCache struct {
	db                  db.ArgoDB
	clusters            map[string]clustercache.ClusterCache
	lock                sync.RWMutex
	appInformer         cache.SharedIndexInformer
	onObjectUpdated     ObjectUpdatedHandler
	kubectl             kube.Kubectl
	settingsMgr         *settings.SettingsManager
	metricsServer       *metrics.MetricsServer
	cacheSettings       clustercache.Settings
	appInstanceLabelKey string
}

func (c *liveStateCache) loadCacheSettings() (*clustercache.Settings, string, error) {
	appInstanceLabelKey, err := c.settingsMgr.GetAppInstanceLabelKey()
	if err != nil {
		return nil, "", err
	}
	resourcesFilter, err := c.settingsMgr.GetResourcesFilter()
	if err != nil {
		return nil, "", err
	}
	resourceOverrides, err := c.settingsMgr.GetResourceOverrides()
	if err != nil {
		return nil, "", err
	}
	return &clustercache.Settings{ResourceHealthOverride: lua.ResourceHealthOverrides(resourceOverrides), ResourcesFilter: resourcesFilter}, appInstanceLabelKey, nil
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

// skipAppRequeing checks if the object is an API type which we want to skip requeuing against.
// We ignore API types which have a high churn rate, and/or whose updates are irrelevant to the app
func skipAppRequeing(key kube.ResourceKey) bool {
	return ignoredRefreshResources[key.Group+"/"+key.Kind]
}

func (c *liveStateCache) getCluster(server string) (clustercache.ClusterCache, error) {
	c.lock.RLock()
	clusterCache, ok := c.clusters[server]
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

	clusterCache = clustercache.NewClusterCache(c.cacheSettings, cluster.RESTConfig(), cluster.Namespaces, c.kubectl, clustercache.EventHandlers{
		OnEvent: func(event watch.EventType, un *unstructured.Unstructured) {
			gvk := un.GroupVersionKind()
			c.metricsServer.IncClusterEventsCount(cluster.Server, gvk.Group, gvk.Kind)
		},
		OnPopulateResourceInfo: func(un *unstructured.Unstructured, isRoot bool) (interface{}, bool) {
			res := &ResourceInfo{}
			populateNodeInfo(un, res)
			res.Health, _ = health.GetResourceHealth(un, c.cacheSettings.ResourceHealthOverride)
			appName := kube.GetAppInstanceLabel(un, c.appInstanceLabelKey)
			if isRoot && appName != "" {
				res.AppName = appName
			}
			// edge case. we do not label CRDs, so they miss the tracking label we inject. But we still
			// want the full resource to be available in our cache (to diff), so we store all CRDs
			return res, res.AppName != "" || un.GroupVersionKind().Kind == kube.CustomResourceDefinitionKind
		},
		OnResourceUpdated: func(newRes *clustercache.Resource, oldRes *clustercache.Resource, namespaceResources map[kube.ResourceKey]*clustercache.Resource) {
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
				if app == "" || skipAppRequeing(r.ResourceKey()) {
					continue
				}
				toNotify[app] = isRootAppNode(r) || toNotify[app]
			}
			c.onObjectUpdated(toNotify, ref)
		},
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

func (c *liveStateCache) invalidate(settings clustercache.Settings, appInstanceLabelKey string) {
	log.Info("invalidating live state cache")
	c.lock.Lock()
	defer c.lock.Unlock()

	c.appInstanceLabelKey = appInstanceLabelKey
	for _, clust := range c.clusters {
		clust.Invalidate(func(config *rest.Config, namespaces []string, _ clustercache.Settings) (*rest.Config, []string, clustercache.Settings) {
			return config, namespaces, settings
		})
	}
	log.Info("live state cache invalidated")
}

func (c *liveStateCache) IsNamespaced(server string, gk schema.GroupKind) (bool, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return false, err
	}
	return clusterInfo.IsNamespaced(gk), nil
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

func (c *liveStateCache) GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, err
	}
	resources := clusterInfo.GetNamespaceTopLevelResources(namespace)
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

func isClusterHasApps(apps []interface{}, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		if app, ok := obj.(*appv1.Application); ok && app.Spec.Destination.Server == cluster.Server {
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
			nextCacheSettings, appInstanceLabelKey, err := c.loadCacheSettings()
			if err != nil {
				log.Warnf("Failed to read updated settings: %v", err)
				continue
			}

			c.lock.Lock()
			needInvalidate := false
			if !reflect.DeepEqual(c.cacheSettings, nextCacheSettings) {
				c.cacheSettings = *nextCacheSettings
				needInvalidate = true
			}
			c.lock.Unlock()
			if needInvalidate {
				c.invalidate(*nextCacheSettings, appInstanceLabelKey)
			}
		case <-ctx.Done():
			done = true
		}
	}
	log.Info("shutting down settings watch")
	c.settingsMgr.Unsubscribe(updateCh)
	close(updateCh)
}

// Run watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (c *liveStateCache) Run(ctx context.Context) error {
	cacheSettings, appInstanceLabelKey, err := c.loadCacheSettings()
	if err != nil {
		return err
	}
	c.cacheSettings = *cacheSettings
	c.appInstanceLabelKey = appInstanceLabelKey

	go c.watchSettings(ctx)

	kube.RetryUntilSucceed(func() error {
		clusterEventCallback := func(event *db.ClusterEvent) {
			c.lock.Lock()
			cluster, ok := c.clusters[event.Cluster.Server]
			if ok {
				defer c.lock.Unlock()
				if event.Type == watch.Deleted {
					cluster.Invalidate(nil)
					delete(c.clusters, event.Cluster.Server)
				} else if event.Type == watch.Modified {
					cluster.Invalidate(func(cfg *rest.Config, namespaces []string, settings clustercache.Settings) (*rest.Config, []string, clustercache.Settings) {
						return event.Cluster.RESTConfig(), event.Cluster.Namespaces, settings
					})
				}
			} else {
				c.lock.Unlock()
				if event.Type == watch.Added && isClusterHasApps(c.appInformer.GetStore().List(), event.Cluster) {
					go func() {
						// warm up cache for cluster with apps
						_, _ = c.getSyncedCluster(event.Cluster.Server)
					}()
				}
			}
		}

		return c.db.WatchClusters(ctx, clusterEventCallback)

	}, "watch clusters", ctx, clustercache.ClusterRetryTimeout)

	<-ctx.Done()
	c.invalidate(c.cacheSettings, c.appInstanceLabelKey)
	return nil
}

func (c *liveStateCache) GetClustersInfo() []clustercache.ClusterInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()
	res := make([]clustercache.ClusterInfo, 0)
	for _, info := range c.clusters {
		res = append(res, info.GetClusterInfo())
	}
	return res
}
