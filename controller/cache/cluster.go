package cache

import (
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/kube"
	"github.com/argoproj/argo-cd/util/settings"
)

const (
	clusterSyncTimeout  = 24 * time.Hour
	clusterRetryTimeout = 10 * time.Second
)

type gkInfo struct {
	resource    metav1.APIResource
	listVersion string
}

type clusterInfo struct {
	apis         map[schema.GroupKind]*gkInfo
	nodes        map[kube.ResourceKey]*node
	nsIndex      map[string]map[kube.ResourceKey]*node
	lock         *sync.Mutex
	onAppUpdated func(appName string)
	kubectl      kube.Kubectl
	cluster      *appv1.Cluster
	syncLock     *sync.Mutex
	syncTime     *time.Time
	syncError    error
	log          *log.Entry
	settings     *settings.ArgoCDSettings
}

func (c *clusterInfo) getResourceVersion(gk schema.GroupKind) string {
	c.lock.Lock()
	defer c.lock.Unlock()
	info, ok := c.apis[gk]
	if ok {
		return info.listVersion
	}
	return ""
}

func (c *clusterInfo) updateCache(gk schema.GroupKind, resourceVersion string, objs []unstructured.Unstructured) {
	c.lock.Lock()
	defer c.lock.Unlock()
	info, ok := c.apis[gk]
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
		info.listVersion = resourceVersion
	}
}

func createObjInfo(un *unstructured.Unstructured, appInstanceLabel string) *node {
	ownerRefs := un.GetOwnerReferences()
	// Special case for endpoint. Remove after https://github.com/kubernetes/kubernetes/issues/28483 is fixed
	if un.GroupVersionKind().Group == "" && un.GetKind() == kube.EndpointsKind && len(un.GetOwnerReferences()) == 0 {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			Name:       un.GetName(),
			Kind:       kube.ServiceKind,
			APIVersion: "",
		})
	}
	info := &node{
		resourceVersion: un.GetResourceVersion(),
		ref: v1.ObjectReference{
			APIVersion: un.GetAPIVersion(),
			Kind:       un.GetKind(),
			Name:       un.GetName(),
			Namespace:  un.GetNamespace(),
		},
		ownerRefs: ownerRefs,
		info:      getNodeInfo(un),
	}
	appName := kube.GetAppInstanceLabel(un, appInstanceLabel)
	if len(ownerRefs) == 0 && appName != "" {
		info.appName = appName
		info.resource = un
	}
	return info
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
	c.syncTime = nil
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

func (c *clusterInfo) sync() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
	}()

	c.log.Info("Start syncing cluster")

	c.apis = make(map[schema.GroupKind]*gkInfo)
	c.nodes = make(map[kube.ResourceKey]*node)

	resources, err := c.kubectl.GetResources(c.cluster.RESTConfig(), c.settings, "")
	if err != nil {
		log.Errorf("Failed to sync cluster %s: %v", c.cluster.Server, err)
		return err
	}

	appLabelKey := c.settings.GetAppInstanceLabelKey()
	for res := range resources {
		if res.Error != nil {
			return res.Error
		}
		if _, ok := c.apis[res.GVK.GroupKind()]; !ok {
			c.apis[res.GVK.GroupKind()] = &gkInfo{
				listVersion: res.ListResourceVersion,
				resource:    res.ResourceInfo,
			}
		}
		for i := range res.Objects {
			c.setNode(createObjInfo(&res.Objects[i], appLabelKey))
		}
	}

	c.log.Info("Cluster successfully synced")
	return nil
}

func (c *clusterInfo) ensureSynced() error {
	if c.synced() {
		return c.syncError
	}
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

func (c *clusterInfo) getChildren(obj *unstructured.Unstructured) []appv1.ResourceNode {
	c.lock.Lock()
	defer c.lock.Unlock()
	children := make([]appv1.ResourceNode, 0)
	if objInfo, ok := c.nodes[kube.GetResourceKey(obj)]; ok {
		nsNodes := c.nsIndex[obj.GetNamespace()]
		for _, child := range nsNodes {
			if objInfo.isParentOf(child) {
				children = append(children, child.childResourceNodes(nsNodes, map[kube.ResourceKey]bool{objInfo.resourceKey(): true}))
			}
		}
	}
	return children
}

func (c *clusterInfo) isNamespaced(gk schema.GroupKind) bool {
	if api, ok := c.apis[gk]; ok && !api.resource.Namespaced {
		return false
	}
	return true
}

func (c *clusterInfo) getManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	managedObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
	// iterate all objects in live state cache to find ones associated with app
	for key, o := range c.nodes {
		if o.appName == a.Name && o.resource != nil && len(o.ownerRefs) == 0 {
			managedObjs[key] = o.resource
		}
	}
	// iterate target objects and identify ones that already exist in the cluster,\
	// but are simply missing our label
	lock := &sync.Mutex{}
	err := util.RunAllAsync(len(targetObjs), func(i int) error {
		targetObj := targetObjs[i]
		key := GetTargetObjKey(a, targetObj, c.isNamespaced(targetObj.GroupVersionKind().GroupKind()))
		lock.Lock()
		managedObj := managedObjs[key]
		lock.Unlock()

		if managedObj == nil {
			if existingObj, exists := c.nodes[key]; exists {
				if existingObj.resource != nil {
					managedObj = existingObj.resource
				} else {
					var err error
					managedObj, err = c.kubectl.GetResource(c.cluster.RESTConfig(), targetObj.GroupVersionKind(), existingObj.ref.Name, existingObj.ref.Namespace)
					if err != nil {
						if errors.IsNotFound(err) {
							c.checkAndInvalidateStaleCache(targetObj.GroupVersionKind(), existingObj.ref.Namespace, existingObj.ref.Name)
							return nil
						}
						return err
					}
				}
			}
		}

		if managedObj != nil {
			managedObj, err := c.kubectl.ConvertToVersion(managedObj, targetObj.GroupVersionKind().Group, targetObj.GroupVersionKind().Version)
			if err != nil {
				return err
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

func (c *clusterInfo) delete(obj *unstructured.Unstructured) error {
	err := c.kubectl.DeleteResource(c.cluster.RESTConfig(), obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), false)
	if err != nil && errors.IsNotFound(err) {
		// a delete request came in for an object which does not exist. it's possible that our cache
		// is stale. Check and invalidate if it is
		c.lock.Lock()
		c.checkAndInvalidateStaleCache(obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName())
		c.lock.Unlock()
		return nil
	}
	return err
}

// checkAndInvalidateStaleCache checks if our cache is stale and invalidate it based on error
// should be called whenever we suspect our cache is stale
func (c *clusterInfo) checkAndInvalidateStaleCache(gvk schema.GroupVersionKind, namespace string, name string) {
	if _, ok := c.nodes[kube.NewResourceKey(gvk.Group, gvk.Kind, namespace, name)]; ok {
		if c.syncTime != nil {
			c.log.Warnf("invalidated stale cache due to mismatch of %s, %s/%s", gvk, namespace, name)
			c.invalidate()
		}
	}
}

func (c *clusterInfo) processEvent(event watch.EventType, un *unstructured.Unstructured) error {
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

	return nil
}

func (c *clusterInfo) onNodeUpdated(exists bool, existingNode *node, un *unstructured.Unstructured, key kube.ResourceKey) {
	nodes := make([]*node, 0)
	if exists {
		nodes = append(nodes, existingNode)
	}
	newObj := createObjInfo(un, c.settings.GetAppInstanceLabelKey())
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
			toNotify[app] = true
		}
	}
	for name := range toNotify {
		c.onAppUpdated(name)
	}
}

func (c *clusterInfo) onNodeRemoved(key kube.ResourceKey, existingNode *node) {
	c.removeNode(key)
	if existingNode.appName != "" {
		c.onAppUpdated(existingNode.appName)
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
