package cache

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/argoproj/argo-cd/controller/metrics"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/kube"
)

const (
	clusterSyncTimeout         = 24 * time.Hour
	clusterRetryTimeout        = 10 * time.Second
	watchResourcesRetryTimeout = 1 * time.Second
)

type apiMeta struct {
	namespaced      bool
	resourceVersion string
	watchCancel     context.CancelFunc
}

type clusterInfo struct {
	syncLock  *sync.Mutex
	syncTime  *time.Time
	syncError error
	apisMeta  map[schema.GroupKind]*apiMeta

	lock    *sync.Mutex
	nodes   map[kube.ResourceKey]*node
	nsIndex map[string]map[kube.ResourceKey]*node

	onAppUpdated     AppUpdatedHandler
	kubectl          kube.Kubectl
	cluster          *appv1.Cluster
	log              *log.Entry
	cacheSettingsSrc func() *cacheSettings
}

func (c *clusterInfo) replaceResourceCache(gk schema.GroupKind, resourceVersion string, objs []unstructured.Unstructured) {
	c.lock.Lock()
	defer c.lock.Unlock()
	info, ok := c.apisMeta[gk]
	if ok {
		objByKind := make(map[kube.ResourceKey]*unstructured.Unstructured)
		for i := range objs {
			objByKind[kube.GetResourceKey(&objs[i])] = &objs[i]
		}

		for i := range objs {
			obj := &objs[i]
			key := kube.GetResourceKey(&objs[i])
			existingNode, exists := c.nodes[key]
			c.onNodeUpdated(exists, existingNode, obj, key)
		}

		for key, existingNode := range c.nodes {
			if key.Kind != gk.Kind || key.Group != gk.Group {
				continue
			}

			if _, ok := objByKind[key]; !ok {
				c.onNodeRemoved(key, existingNode)
			}
		}
		info.resourceVersion = resourceVersion
	}
}

func (c *clusterInfo) createObjInfo(un *unstructured.Unstructured, appInstanceLabel string) *node {
	ownerRefs := un.GetOwnerReferences()
	// Special case for endpoint. Remove after https://github.com/kubernetes/kubernetes/issues/28483 is fixed
	if un.GroupVersionKind().Group == "" && un.GetKind() == kube.EndpointsKind && len(un.GetOwnerReferences()) == 0 {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			Name:       un.GetName(),
			Kind:       kube.ServiceKind,
			APIVersion: "v1",
		})
	}
	nodeInfo := &node{
		resourceVersion: un.GetResourceVersion(),
		ref:             kube.GetObjectRef(un),
		ownerRefs:       ownerRefs,
	}
	populateNodeInfo(un, nodeInfo)
	appName := kube.GetAppInstanceLabel(un, appInstanceLabel)
	if len(ownerRefs) == 0 && appName != "" {
		nodeInfo.appName = appName
		nodeInfo.resource = un
	}
	nodeInfo.health, _ = health.GetResourceHealth(un, c.cacheSettingsSrc().ResourceOverrides)
	return nodeInfo
}

func (c *clusterInfo) setNode(n *node) {
	key := n.resourceKey()
	c.nodes[key] = n
	ns, ok := c.nsIndex[key.Namespace]
	if !ok {
		ns = make(map[kube.ResourceKey]*node)
		c.nsIndex[key.Namespace] = ns
	}
	ns[key] = n
}

func (c *clusterInfo) removeNode(key kube.ResourceKey) {
	delete(c.nodes, key)
	if ns, ok := c.nsIndex[key.Namespace]; ok {
		delete(ns, key)
		if len(ns) == 0 {
			delete(c.nsIndex, key.Namespace)
		}
	}
}

func (c *clusterInfo) invalidate() {
	c.syncLock.Lock()
	defer c.syncLock.Unlock()
	c.syncTime = nil
	for i := range c.apisMeta {
		c.apisMeta[i].watchCancel()
	}
	c.apisMeta = nil
}

func (c *clusterInfo) synced() bool {
	if c.syncTime == nil {
		return false
	}
	if c.syncError != nil {
		return time.Now().Before(c.syncTime.Add(clusterRetryTimeout))
	}
	return time.Now().Before(c.syncTime.Add(clusterSyncTimeout))
}

func (c *clusterInfo) stopWatching(gk schema.GroupKind) {
	c.syncLock.Lock()
	defer c.syncLock.Unlock()
	if info, ok := c.apisMeta[gk]; ok {
		info.watchCancel()
		delete(c.apisMeta, gk)
		c.replaceResourceCache(gk, "", []unstructured.Unstructured{})
		log.Warnf("Stop watching %s not found on %s.", gk, c.cluster.Server)
	}
}

// startMissingWatches lists supported cluster resources and start watching for changes unless watch is already running
func (c *clusterInfo) startMissingWatches() error {

	apis, err := c.kubectl.GetAPIResources(c.cluster.RESTConfig(), c.cacheSettingsSrc().ResourcesFilter)
	if err != nil {
		return err
	}

	for i := range apis {
		api := apis[i]
		if _, ok := c.apisMeta[api.GroupKind]; !ok {
			ctx, cancel := context.WithCancel(context.Background())
			info := &apiMeta{namespaced: api.Meta.Namespaced, watchCancel: cancel}
			c.apisMeta[api.GroupKind] = info
			go c.watchEvents(ctx, api, info)
		}
	}
	return nil
}

func runSynced(lock *sync.Mutex, action func() error) error {
	lock.Lock()
	defer lock.Unlock()
	return action()
}

func (c *clusterInfo) watchEvents(ctx context.Context, api kube.APIResourceInfo, info *apiMeta) {
	util.RetryUntilSucceed(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
			}
		}()

		err = runSynced(c.syncLock, func() error {
			if info.resourceVersion == "" {
				list, err := api.Interface.List(metav1.ListOptions{})
				if err != nil {
					return err
				}
				c.replaceResourceCache(api.GroupKind, list.GetResourceVersion(), list.Items)
			}
			return nil
		})

		if err != nil {
			return err
		}

		w, err := api.Interface.Watch(metav1.ListOptions{ResourceVersion: info.resourceVersion})
		if errors.IsNotFound(err) {
			c.stopWatching(api.GroupKind)
			return nil
		}

		err = runSynced(c.syncLock, func() error {
			if errors.IsGone(err) {
				info.resourceVersion = ""
				log.Warnf("Resource version of %s on %s is too old.", api.GroupKind, c.cluster.Server)
			}
			return err
		})

		if err != nil {
			return err
		}
		defer w.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case event, ok := <-w.ResultChan():
				if ok {
					obj := event.Object.(*unstructured.Unstructured)
					info.resourceVersion = obj.GetResourceVersion()
					c.processEvent(event.Type, obj)
					if kube.IsCRD(obj) {
						if event.Type == watch.Deleted {
							group, groupOk, groupErr := unstructured.NestedString(obj.Object, "spec", "group")
							kind, kindOk, kindErr := unstructured.NestedString(obj.Object, "spec", "names", "kind")

							if groupOk && groupErr == nil && kindOk && kindErr == nil {
								gk := schema.GroupKind{Group: group, Kind: kind}
								c.stopWatching(gk)
							}
						} else {
							err = runSynced(c.syncLock, func() error {
								return c.startMissingWatches()
							})

						}
					}
					if err != nil {
						log.Warnf("Failed to start missing watch: %v", err)
					}
				} else {
					return fmt.Errorf("Watch %s on %s has closed", api.GroupKind, c.cluster.Server)
				}
			}
		}

	}, fmt.Sprintf("watch %s on %s", api.GroupKind, c.cluster.Server), ctx, watchResourcesRetryTimeout)
}

func (c *clusterInfo) sync() (err error) {

	c.log.Info("Start syncing cluster")

	for i := range c.apisMeta {
		c.apisMeta[i].watchCancel()
	}
	c.apisMeta = make(map[schema.GroupKind]*apiMeta)
	c.nodes = make(map[kube.ResourceKey]*node)

	apis, err := c.kubectl.GetAPIResources(c.cluster.RESTConfig(), c.cacheSettingsSrc().ResourcesFilter)
	if err != nil {
		return err
	}
	lock := sync.Mutex{}
	err = util.RunAllAsync(len(apis), func(i int) error {
		api := apis[i]
		list, err := api.Interface.List(metav1.ListOptions{})
		if err != nil {
			return err
		}

		lock.Lock()
		for i := range list.Items {
			c.setNode(c.createObjInfo(&list.Items[i], c.cacheSettingsSrc().AppInstanceLabelKey))
		}
		lock.Unlock()
		return nil
	})

	if err == nil {
		err = c.startMissingWatches()
	}

	if err != nil {
		log.Errorf("Failed to sync cluster %s: %v", c.cluster.Server, err)
		return err
	}

	c.log.Info("Cluster successfully synced")
	return nil
}

func (c *clusterInfo) ensureSynced() error {
	c.syncLock.Lock()
	defer c.syncLock.Unlock()
	if c.synced() {
		return c.syncError
	}

	err := c.sync()
	syncTime := time.Now()
	c.syncTime = &syncTime
	c.syncError = err
	return c.syncError
}

func (c *clusterInfo) iterateHierarchy(obj *unstructured.Unstructured, action func(child appv1.ResourceNode)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	key := kube.GetResourceKey(obj)
	if objInfo, ok := c.nodes[key]; ok {
		action(objInfo.asResourceNode())
		nsNodes := c.nsIndex[key.Namespace]
		childrenByUID := make(map[types.UID][]*node)
		for _, child := range nsNodes {
			if objInfo.isParentOf(child) {
				childrenByUID[child.ref.UID] = append(childrenByUID[child.ref.UID], child)
			}
		}
		// make sure children has no duplicates
		for _, children := range childrenByUID {
			if len(children) > 0 {
				// The object might have multiple children with the same UID (e.g. replicaset from apps and extensions group). It is ok to pick any object but we need to make sure
				// we pick the same child after every refresh.
				sort.Slice(children, func(i, j int) bool {
					key1 := children[i].resourceKey()
					key2 := children[j].resourceKey()
					return strings.Compare(key1.String(), key2.String()) < 0
				})
				child := children[0]
				action(child.asResourceNode())
				child.iterateChildren(nsNodes, map[kube.ResourceKey]bool{objInfo.resourceKey(): true}, action)
			}
		}
	} else {
		action(c.createObjInfo(obj, c.cacheSettingsSrc().AppInstanceLabelKey).asResourceNode())
	}
}

func (c *clusterInfo) isNamespaced(obj *unstructured.Unstructured) bool {
	if api, ok := c.apisMeta[kube.GetResourceKey(obj).GroupKind()]; ok && !api.namespaced {
		return false
	}
	return true
}

func (c *clusterInfo) getManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured, metricsServer *metrics.MetricsServer) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	managedObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
	// iterate all objects in live state cache to find ones associated with app
	for key, o := range c.nodes {
		if o.appName == a.Name && o.resource != nil && len(o.ownerRefs) == 0 {
			managedObjs[key] = o.resource
		}
	}
	config := metrics.AddMetricsTransportWrapper(metricsServer, a, c.cluster.RESTConfig())
	// iterate target objects and identify ones that already exist in the cluster,\
	// but are simply missing our label
	lock := &sync.Mutex{}
	err := util.RunAllAsync(len(targetObjs), func(i int) error {
		targetObj := targetObjs[i]
		key := GetTargetObjKey(a, targetObj, c.isNamespaced(targetObj))
		lock.Lock()
		managedObj := managedObjs[key]
		lock.Unlock()

		if managedObj == nil {
			if existingObj, exists := c.nodes[key]; exists {
				if existingObj.resource != nil {
					managedObj = existingObj.resource
				} else {
					var err error
					managedObj, err = c.kubectl.GetResource(config, targetObj.GroupVersionKind(), existingObj.ref.Name, existingObj.ref.Namespace)
					if err != nil {
						if errors.IsNotFound(err) {
							return nil
						}
						return err
					}
				}
			} else if _, watched := c.apisMeta[key.GroupKind()]; !watched {
				var err error
				managedObj, err = c.kubectl.GetResource(config, targetObj.GroupVersionKind(), targetObj.GetName(), targetObj.GetNamespace())
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
			}
		}

		if managedObj != nil {
			converted, err := c.kubectl.ConvertToVersion(managedObj, targetObj.GroupVersionKind().Group, targetObj.GroupVersionKind().Version)
			if err != nil {
				// fallback to loading resource from kubernetes if conversion fails
				log.Warnf("Failed to convert resource: %v", err)
				managedObj, err = c.kubectl.GetResource(config, targetObj.GroupVersionKind(), managedObj.GetName(), managedObj.GetNamespace())
				if err != nil {
					if errors.IsNotFound(err) {
						return nil
					}
					return err
				}
			} else {
				managedObj = converted
			}
			lock.Lock()
			managedObjs[key] = managedObj
			lock.Unlock()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return managedObjs, nil
}

func (c *clusterInfo) processEvent(event watch.EventType, un *unstructured.Unstructured) {
	c.lock.Lock()
	defer c.lock.Unlock()
	key := kube.GetResourceKey(un)
	existingNode, exists := c.nodes[key]
	if event == watch.Deleted {
		if exists {
			c.onNodeRemoved(key, existingNode)
		}
	} else if event != watch.Deleted {
		c.onNodeUpdated(exists, existingNode, un, key)
	}
}

func (c *clusterInfo) onNodeUpdated(exists bool, existingNode *node, un *unstructured.Unstructured, key kube.ResourceKey) {
	nodes := make([]*node, 0)
	if exists {
		nodes = append(nodes, existingNode)
	}
	newObj := c.createObjInfo(un, c.cacheSettingsSrc().AppInstanceLabelKey)
	c.setNode(newObj)
	nodes = append(nodes, newObj)
	toNotify := make(map[string]bool)
	for i := range nodes {
		n := nodes[i]
		if ns, ok := c.nsIndex[n.ref.Namespace]; ok {
			app := n.getApp(ns)
			if app == "" || skipAppRequeing(key) {
				continue
			}
			toNotify[app] = n.isRootAppNode() || toNotify[app]
		}
	}
	for name, isRootAppNode := range toNotify {
		c.onAppUpdated(name, isRootAppNode, newObj.ref)
	}
}

func (c *clusterInfo) onNodeRemoved(key kube.ResourceKey, n *node) {
	appName := n.appName
	if ns, ok := c.nsIndex[key.Namespace]; ok {
		appName = n.getApp(ns)
	}

	c.removeNode(key)
	if appName != "" {
		c.onAppUpdated(appName, n.isRootAppNode(), n.ref)
	}
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
