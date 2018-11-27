package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
)

const (
	watchResourcesRetryTimeout = 10 * time.Second
)

type LiveStateCache interface {
	// Returns child nodes for a given k8s resource
	GetChildren(server string, obj *unstructured.Unstructured) ([]appv1.ResourceNode, error)
	// Returns state of live nodes which correspond for target nodes of specified application.
	GetControlledLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[string]*unstructured.Unstructured, error)
	// Starts watching resources of each controlled cluster.
	Run(ctx context.Context)
}

func NewLiveStateCache(db db.ArgoDB, appInformer cache.SharedIndexInformer, kubectl kube.Kubectl, onAppUpdated func(appName string)) LiveStateCache {
	return &liveStateCache{
		appInformer:  appInformer,
		db:           db,
		clusters:     make(map[string]*clusterInfo),
		lock:         &sync.Mutex{},
		onAppUpdated: onAppUpdated,
		kubectl:      kubectl,
	}
}

type liveStateCache struct {
	db           db.ArgoDB
	clusters     map[string]*clusterInfo
	lock         *sync.Mutex
	appInformer  cache.SharedIndexInformer
	onAppUpdated func(appName string)
	kubectl      kube.Kubectl
}

func (c *liveStateCache) processEvent(event watch.EventType, obj *unstructured.Unstructured, url string) error {
	info, err := c.getCluster(url)
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
	info, ok := c.clusters[server]
	if !ok {
		cluster, err := c.db.GetCluster(context.Background(), server)
		if err != nil {
			return nil, err
		}

		info = &clusterInfo{
			lock:         &sync.Mutex{},
			nodes:        make(map[string]*node),
			onAppUpdated: c.onAppUpdated,
			kubectl:      c.kubectl,
			cluster:      cluster,
			syncTime:     nil,
			syncLock:     &sync.Mutex{},
		}

		c.clusters[cluster.Server] = info
	}
	c.lock.Unlock()
	err := info.ensureSynced()
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (c *liveStateCache) GetChildren(server string, obj *unstructured.Unstructured) ([]appv1.ResourceNode, error) {
	clusterInfo, err := c.getCluster(server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.getChildren(obj), nil
}

func (c *liveStateCache) GetControlledLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[string]*unstructured.Unstructured, error) {
	clusterInfo, err := c.getCluster(a.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	return clusterInfo.getControlledLiveObjs(a, targetObjs)
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
						c.removeCluster(info.cluster.Server)
						cancel()
					},
					cluster: event.Cluster,
				}
				go c.watchClusterResources(ctx, *event.Cluster)
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

		c.appInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: onAppModified, DeleteFunc: onAppModified})

		return c.db.WatchClusters(ctx, clusterEventCallback)

	}, "watch clusters", ctx, watchResourcesRetryTimeout)

	<-ctx.Done()
}

// watchClusterResources watches for resource changes annotated with application label on specified cluster and schedule corresponding app refresh.
func (c *liveStateCache) watchClusterResources(ctx context.Context, item appv1.Cluster) {
	util.RetryUntilSucceed(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("Recovered from panic: %v\n", r)
			}
		}()
		config := item.RESTConfig()
		watchStartTime := time.Now()
		ch, err := c.kubectl.WatchResources(ctx, config, "")

		if err != nil {
			return err
		}
		for event := range ch {
			eventObj := event.Object.(*unstructured.Unstructured)
			if kube.IsCRD(eventObj) {
				// restart if new CRD has been created after watch started
				if event.Type == watch.Added && watchStartTime.Before(eventObj.GetCreationTimestamp().Time) {
					return fmt.Errorf("Restarting the watch because a new CRD was added.")
				} else if event.Type == watch.Deleted {
					return fmt.Errorf("Restarting the watch because a CRD was deleted.")
				}
			}
			err = c.processEvent(event.Type, eventObj, item.Server)
			if err != nil {
				log.Warnf("Failed to process event %s for obj %v: %v", event.Type, event.Object, err)
			}
		}
		return fmt.Errorf("resource updates channel has closed")
	}, fmt.Sprintf("watch app resources on %s", item.Server), ctx, watchResourcesRetryTimeout)
}
