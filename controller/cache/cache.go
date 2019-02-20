package cache

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

type LiveStateCache interface {
	IsNamespaced(server string, gvk schema.GroupVersionKind) (bool, error)
	// Returns child nodes for a given k8s resource
	GetChildren(server string, obj *unstructured.Unstructured) ([]appv1.ResourceNode, error)
	// Returns state of live nodes which correspond for target nodes of specified application.
	GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error)
	// Starts watching resources of each controlled cluster.
	Run(ctx context.Context)
	// Deletes specified resource from cluster.
	Delete(server string, obj *unstructured.Unstructured) error
	// Invalidate invalidates the entire cluster state cache
	Invalidate()
}

func GetTargetObjKey(a *appv1.Application, un *unstructured.Unstructured, isNamespaced bool) kube.ResourceKey {
	key := kube.GetResourceKey(un)
	if !isNamespaced {
		key.Namespace = ""
	} else if isNamespaced && key.Namespace == "" {
		key.Namespace = a.Spec.Destination.Namespace
	}

	return key
}

func NewLiveStateCache(db db.ArgoDB, appInformer cache.SharedIndexInformer, settings *settings.ArgoCDSettings, kubectl kube.Kubectl, onAppUpdated func(appName string)) LiveStateCache {
	return &liveStateCache{
		appInformer:  appInformer,
		db:           db,
		clusters:     make(map[string]*clusterInfo),
		lock:         &sync.Mutex{},
		onAppUpdated: onAppUpdated,
		kubectl:      kubectl,
		settings:     settings,
	}
}

type liveStateCache struct {
	db           db.ArgoDB
	clusters     map[string]*clusterInfo
	lock         *sync.Mutex
	appInformer  cache.SharedIndexInformer
	onAppUpdated func(appName string)
	kubectl      kube.Kubectl
	settings     *settings.ArgoCDSettings
}

func (c *liveStateCache) processEvent(event watch.EventType, obj *unstructured.Unstructured, url string) error {
	info, err := c.getSyncedCluster(url)
	if err != nil {
		return err
	}
	return info.processEvent(event, obj)
}

func (c *liveStateCache) removeCluster(server string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	delete(c.clusters, server)
	log.Infof("Dropped cluster %s cache", server)
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
			apis:         make(map[schema.GroupVersionKind]metav1.APIResource),
			lock:         &sync.Mutex{},
			nodes:        make(map[kube.ResourceKey]*node),
			nsIndex:      make(map[string]map[kube.ResourceKey]*node),
			onAppUpdated: c.onAppUpdated,
			kubectl:      c.kubectl,
			cluster:      cluster,
			syncTime:     nil,
			syncLock:     &sync.Mutex{},
			log:          log.WithField("server", cluster.Server),
			settings:     c.settings,
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

func (c *liveStateCache) Delete(server string, obj *unstructured.Unstructured) error {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return err
	}
	return clusterInfo.delete(obj)
}

func (c *liveStateCache) IsNamespaced(server string, gvk schema.GroupVersionKind) (bool, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return false, err
	}
	return clusterInfo.isNamespaced(gvk), nil
}

func (c *liveStateCache) GetChildren(server string, obj *unstructured.Unstructured) ([]appv1.ResourceNode, error) {
	clusterInfo, err := c.getSyncedCluster(server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.getChildren(obj), nil
}

func (c *liveStateCache) GetManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getSyncedCluster(a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.getManagedLiveObjs(a, targetObjs)
}

func isClusterHasApps(apps []interface{}, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		if app, ok := obj.(*appv1.Application); ok && app.Spec.Destination.Server == cluster.Server {
			return true
		}
	}
	return false
}

// Run watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (c *liveStateCache) Run(ctx context.Context) {
	watchingClusters := make(map[string]struct {
		cancel  context.CancelFunc
		cluster *appv1.Cluster
	})

	util.RetryUntilSucceed(func() error {
		clusterEventCallback := func(event *db.ClusterEvent) {
			info, ok := watchingClusters[event.Cluster.Server]
			hasApps := isClusterHasApps(c.appInformer.GetStore().List(), event.Cluster)

			// cluster resources must be watched only if cluster has at least one app
			if (event.Type == watch.Deleted || !hasApps) && ok {
				info.cancel()
				delete(watchingClusters, event.Cluster.Server)
			} else if event.Type != watch.Deleted && !ok && hasApps {
				ctx, cancel := context.WithCancel(ctx)
				watchingClusters[event.Cluster.Server] = struct {
					cancel  context.CancelFunc
					cluster *appv1.Cluster
				}{
					cancel: func() {
						c.removeCluster(event.Cluster.Server)
						cancel()
					},
					cluster: event.Cluster,
				}
				go c.watchClusterResources(ctx, c.settings, *event.Cluster)
			}
		}

		onAppModified := func(obj interface{}) {
			if app, ok := obj.(*appv1.Application); ok {
				var cluster *appv1.Cluster
				info, infoOk := watchingClusters[app.Spec.Destination.Server]
				if infoOk {
					cluster = info.cluster
				} else {
					cluster, _ = c.db.GetCluster(ctx, app.Spec.Destination.Server)
				}
				if cluster != nil {
					// trigger cluster event every time when app created/deleted to either start or stop watching resources
					clusterEventCallback(&db.ClusterEvent{Cluster: cluster, Type: watch.Modified})
				}
			}
		}

		c.appInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: onAppModified,
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldApp, oldOk := oldObj.(*appv1.Application)
				newApp, newOk := newObj.(*appv1.Application)
				if oldOk && newOk {
					if oldApp.Spec.Destination.Server != newApp.Spec.Destination.Server {
						onAppModified(oldObj)
						onAppModified(newApp)
					}
				}
			},
			DeleteFunc: onAppModified,
		})

		return c.db.WatchClusters(ctx, clusterEventCallback)

	}, "watch clusters", ctx, clusterRetryTimeout)

	<-ctx.Done()
}

// watchClusterResources watches for resource changes annotated with application label on specified cluster and schedule corresponding app refresh.
func (c *liveStateCache) watchClusterResources(ctx context.Context, settings *settings.ArgoCDSettings, item appv1.Cluster) {
	util.RetryUntilSucceed(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("Recovered from panic: %v\n", r)
			}
		}()
		config := item.RESTConfig()
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

		knownCRDs, err := getCRDs(config)
		if err != nil {
			return err
		}
		ch, err := c.kubectl.WatchResources(ctx, config, settings, "")
		if err != nil {
			return err
		}
		for event := range ch {
			eventObj := event.Object.(*unstructured.Unstructured)
			if kube.IsCRD(eventObj) {
				// restart if new CRD has been created after watch started
				if event.Type == watch.Added {
					if !knownCRDs[eventObj.GetName()] {
						c.removeCluster(item.Server)
						return fmt.Errorf("Restarting the watch because a new CRD %s was added", eventObj.GetName())
					} else {
						log.Infof("CRD %s updated", eventObj.GetName())
					}
				} else if event.Type == watch.Deleted {
					c.removeCluster(item.Server)
					return fmt.Errorf("Restarting the watch because CRD %s was deleted", eventObj.GetName())
				}
			}
			err = c.processEvent(event.Type, eventObj, item.Server)
			if err != nil {
				log.Warnf("Failed to process event %s for obj %v: %v", event.Type, event.Object, err)
			}
		}
		return fmt.Errorf("resource updates channel has closed")
	}, fmt.Sprintf("watch app resources on %s", item.Server), ctx, clusterRetryTimeout)
}

// getCRDs returns a map of crds
func getCRDs(config *rest.Config) (map[string]bool, error) {
	crdsByName := make(map[string]bool)
	apiextensionsClientset := apiextensionsclient.NewForConfigOrDie(config)
	crds, err := apiextensionsClientset.ApiextensionsV1beta1().CustomResourceDefinitions().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, crd := range crds.Items {
		crdsByName[crd.Name] = true
	}
	// TODO: support api service, like ServiceCatalog
	return crdsByName, nil
}
