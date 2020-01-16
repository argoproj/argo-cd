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
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/engine/pkg/utils/kube"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/settings"
)

type cacheSettings struct {
	ResourceOverrides   map[string]appv1.ResourceOverride
	AppInstanceLabelKey string
	ResourcesFilter     *settings.ResourcesFilter
}

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
	// Invalidate invalidates the entire cluster state cache
	Invalidate()
	// Returns information about monitored clusters
	GetClustersInfo() []metrics.ClusterInfo
}

type ObjectUpdatedHandler = func(managedByApp map[string]bool, ref v1.ObjectReference)

func GetTargetObjKey(a *appv1.Application, un *unstructured.Unstructured, isNamespaced bool) kube.ResourceKey {
	key := kube.GetResourceKey(un)
	if !isNamespaced {
		key.Namespace = ""
	} else if isNamespaced && key.Namespace == "" {
		key.Namespace = a.Spec.Destination.Namespace
	}

	return key
}

func NewLiveStateCache(
	db db.ArgoDB,
	appInformer cache.SharedIndexInformer,
	settingsMgr *settings.SettingsManager,
	kubectl kube.Kubectl,
	metricsServer *metrics.MetricsServer,
	onObjectUpdated ObjectUpdatedHandler) LiveStateCache {

	return &liveStateCache{
		appInformer:       appInformer,
		db:                db,
		clusters:          make(map[string]*clusterInfo),
		lock:              &sync.RWMutex{},
		onObjectUpdated:   onObjectUpdated,
		kubectl:           kubectl,
		settingsMgr:       settingsMgr,
		metricsServer:     metricsServer,
		cacheSettingsLock: &sync.Mutex{},
	}
}

type liveStateCache struct {
	db                db.ArgoDB
	clusters          map[string]*clusterInfo
	lock              *sync.RWMutex
	appInformer       cache.SharedIndexInformer
	onObjectUpdated   ObjectUpdatedHandler
	kubectl           kube.Kubectl
	settingsMgr       *settings.SettingsManager
	metricsServer     *metrics.MetricsServer
	cacheSettingsLock *sync.Mutex
	cacheSettings     *cacheSettings
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
	return &cacheSettings{AppInstanceLabelKey: appInstanceLabelKey, ResourceOverrides: resourceOverrides, ResourcesFilter: resourcesFilter}, nil
}

func (c *liveStateCache) getCluster(server string) (*clusterInfo, error) {
	c.lock.RLock()
	info, ok := c.clusters[server]
	c.lock.RUnlock()

	if ok {
		return info, nil
	}

	c.lock.Lock()
	defer c.lock.Unlock()

	info, ok = c.clusters[server]
	if ok {
		return info, nil
	}

	logCtx := log.WithField("server", server)
	logCtx.Info("initializing cluster")
	cluster, err := c.db.GetCluster(context.Background(), server)
	if err != nil {
		return nil, err
	}
	info = &clusterInfo{
		apisMeta:         make(map[schema.GroupKind]*apiMeta),
		lock:             &sync.RWMutex{},
		nodes:            make(map[kube.ResourceKey]*node),
		nsIndex:          make(map[string]map[kube.ResourceKey]*node),
		onObjectUpdated:  c.onObjectUpdated,
		kubectl:          c.kubectl,
		cluster:          cluster,
		syncTime:         nil,
		log:              logCtx,
		cacheSettingsSrc: c.getCacheSettings,
		onEventReceived: func(event watch.EventType, un *unstructured.Unstructured) {
			gvk := un.GroupVersionKind()
			c.metricsServer.IncClusterEventsCount(cluster.Server, gvk.Group, gvk.Kind)
		},
		metricsServer: c.metricsServer,
	}
	c.clusters[cluster.Server] = info

	return info, nil
}

func (c *liveStateCache) getSyncedCluster(server string) (*clusterInfo, error) {
	info, err := c.getCluster(server)
	if err != nil {
		return nil, err
	}
	err = info.ensureSynced()
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *liveStateCache) Invalidate() {
	log.Info("invalidating live state cache")
	c.lock.RLock()
	defer c.lock.RLock()
	for _, clust := range c.clusters {
		clust.invalidate()
	}
	log.Info("live state cache invalidated")
}

func (c *liveStateCache) IsNamespaced(server string, gk schema.GroupKind) (bool, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return false, err
	}
	return clusterInfo.isNamespaced(gk), nil
}

func (c *liveStateCache) IterateHierarchy(server string, key kube.ResourceKey, action func(child appv1.ResourceNode, appName string)) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	clusterInfo.iterateHierarchy(key, action)
	return nil
}

func (c *liveStateCache) GetNamespaceTopLevelResources(server string, namespace string) (map[kube.ResourceKey]appv1.ResourceNode, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.getNamespaceTopLevelResources(namespace), nil
}

func (c *liveStateCache) GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getSyncedCluster(a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.getManagedLiveObjs(a, targetObjs)
}

func (c *liveStateCache) GetVersionsInfo(serverURL string) (string, []metav1.APIGroup, error) {
	clusterInfo, err := c.getSyncedCluster(serverURL)
	if err != nil {
		return "", nil, err
	}
	return clusterInfo.serverVersion, clusterInfo.apiGroups, nil
}

func isClusterHasApps(apps []interface{}, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		if app, ok := obj.(*appv1.Application); ok && app.Spec.Destination.Server == cluster.Server {
			return true
		}
	}
	return false
}

func (c *liveStateCache) getCacheSettings() *cacheSettings {
	return c.cacheSettings
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

			c.cacheSettingsLock.Lock()
			needInvalidate := false
			if !reflect.DeepEqual(c.cacheSettings, nextCacheSettings) {
				c.cacheSettings = nextCacheSettings
				needInvalidate = true
			}
			c.cacheSettingsLock.Unlock()
			if needInvalidate {
				c.Invalidate()
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
	cacheSettings, err := c.loadCacheSettings()
	if err != nil {
		return err
	}
	c.cacheSettings = cacheSettings

	go c.watchSettings(ctx)

	util.RetryUntilSucceed(func() error {
		clusterEventCallback := func(event *db.ClusterEvent) {
			c.lock.Lock()
			cluster, ok := c.clusters[event.Cluster.Server]
			if ok {
				defer c.lock.Unlock()
				if event.Type == watch.Deleted {
					cluster.invalidate()
					delete(c.clusters, event.Cluster.Server)
				} else if event.Type == watch.Modified {
					cluster.cluster = event.Cluster
					cluster.invalidate()
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

	}, "watch clusters", ctx, clusterRetryTimeout)

	<-ctx.Done()
	return nil
}

func (c *liveStateCache) GetClustersInfo() []metrics.ClusterInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()
	res := make([]metrics.ClusterInfo, 0)
	for _, info := range c.clusters {
		res = append(res, info.getClusterInfo())
	}
	return res
}
