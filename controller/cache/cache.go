package cache

import (
	"context"
	"reflect"
	"sync"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

type cacheSettings struct {
	ResourceOverrides   map[string]appv1.ResourceOverride
	AppInstanceLabelKey string
	ResourcesFilter     *settings.ResourcesFilter
}

type LiveStateCache interface {
	// Returns k8s server version
	GetServerVersion(serverURL string) (string, error)
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
		lock:              &sync.Mutex{},
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
	lock              *sync.Mutex
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
	c.lock.Lock()
	defer c.lock.Unlock()
	info, ok := c.clusters[server]
	if !ok {
		cluster, err := c.db.GetCluster(context.Background(), server)
		if err != nil {
			return nil, err
		}
		info = &clusterInfo{
			apisMeta:         make(map[schema.GroupKind]*apiMeta),
			lock:             &sync.Mutex{},
			nodes:            make(map[kube.ResourceKey]*node),
			nsIndex:          make(map[string]map[kube.ResourceKey]*node),
			onObjectUpdated:  c.onObjectUpdated,
			kubectl:          c.kubectl,
			cluster:          cluster,
			syncTime:         nil,
			syncLock:         &sync.Mutex{},
			log:              log.WithField("server", cluster.Server),
			cacheSettingsSrc: c.getCacheSettings,
		}

		c.clusters[cluster.Server] = info
	}
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
	c.lock.Lock()
	defer c.lock.Unlock()
	for _, clust := range c.clusters {
		clust.lock.Lock()
		clust.invalidate()
		clust.lock.Unlock()
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
	return clusterInfo.getManagedLiveObjs(a, targetObjs, c.metricsServer)
}
func (c *liveStateCache) GetServerVersion(serverURL string) (string, error) {
	clusterInfo, err := c.getSyncedCluster(serverURL)
	if err != nil {
		return "", err
	}
	return clusterInfo.serverVersion, nil
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
	c.cacheSettingsLock.Lock()
	defer c.cacheSettingsLock.Unlock()
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
			defer c.lock.Unlock()
			if cluster, ok := c.clusters[event.Cluster.Server]; ok {
				if event.Type == watch.Deleted {
					cluster.invalidate()
					delete(c.clusters, event.Cluster.Server)
				} else if event.Type == watch.Modified {
					cluster.cluster = event.Cluster
					cluster.invalidate()
				}
			} else if event.Type == watch.Added && isClusterHasApps(c.appInformer.GetStore().List(), event.Cluster) {
				go func() {
					// warm up cache for cluster with apps
					_, _ = c.getSyncedCluster(event.Cluster.Server)
				}()
			}
		}

		return c.db.WatchClusters(ctx, clusterEventCallback)

	}, "watch clusters", ctx, clusterRetryTimeout)

	<-ctx.Done()
	return nil
}
