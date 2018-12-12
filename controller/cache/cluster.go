package cache

import (
	"sync"
	"time"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
)

const (
	clusterSyncTimeout = 1 * time.Hour
)

type clusterInfo struct {
	apis         map[schema.GroupVersionKind]metav1.APIResource
	nodes        map[kube.ResourceKey]*node
	nsIndex      map[string]map[kube.ResourceKey]*node
	lock         *sync.Mutex
	onAppUpdated func(appName string)
	kubectl      kube.Kubectl
	cluster      *appv1.Cluster
	syncLock     *sync.Mutex
	syncTime     *time.Time
	log          *log.Entry
}

func createObjInfo(un *unstructured.Unstructured) *node {
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
	appName := kube.GetAppInstanceLabel(un)
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

func (c *clusterInfo) synced() bool {
	return c.syncTime != nil && time.Now().Before(c.syncTime.Add(clusterSyncTimeout))
}

func (c *clusterInfo) ensureSynced() error {
	if c.synced() {
		return nil
	}
	c.syncLock.Lock()
	defer c.syncLock.Unlock()
	if c.synced() {
		return nil
	}
	c.log.Info("Start syncing cluster")
	c.nodes = make(map[kube.ResourceKey]*node)
	resources, err := c.kubectl.GetResources(c.cluster.RESTConfig(), "")
	if err != nil {
		log.Errorf("Failed to sync cluster %s: %v", c.cluster.Server, err)
		return err
	}

	for i := range resources {
		c.setNode(createObjInfo(resources[i]))
	}

	resyncTime := time.Now()
	c.syncTime = &resyncTime
	c.log.Info("Cluster successfully synced")
	return nil
}

func (c *clusterInfo) getChildren(obj *unstructured.Unstructured) []appv1.ResourceNode {
	c.lock.Lock()
	defer c.lock.Unlock()
	children := make([]appv1.ResourceNode, 0)
	if objInfo, ok := c.nodes[kube.GetResourceKey(obj)]; ok {
		nsNodes := c.nsIndex[obj.GetNamespace()]
		for _, child := range nsNodes {
			if objInfo.isParentOf(child) {
				children = append(children, child.childResourceNodes(nsNodes))
			}
		}
	}
	return children
}

func (c *clusterInfo) isNamespaced(gvk schema.GroupVersionKind) bool {
	if api, ok := c.apis[gvk]; ok && !api.Namespaced {
		return false
	}
	return true
}

func (c *clusterInfo) getManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	managedObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for key, o := range c.nodes {
		if o.appName == a.Name && o.resource != nil && len(o.ownerRefs) == 0 {
			managedObjs[key] = o.resource
		}
	}
	lock := &sync.Mutex{}
	err := util.RunAllAsync(len(targetObjs), func(i int) error {
		targetObj := targetObjs[i]
		key := GetTargetObjKey(a, targetObj, c.isNamespaced(targetObj.GroupVersionKind()))
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
					err = c.handleError(targetObj.GroupVersionKind(), existingObj.ref.Namespace, existingObj.ref.Name, err)
					if err != nil && !errors.IsNotFound(err) {
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
	err = c.handleError(obj.GroupVersionKind(), obj.GetNamespace(), obj.GetName(), err)
	if err != nil && errors.IsNotFound(err) {
		err = nil
	}
	return err
}

func (c *clusterInfo) handleError(gvk schema.GroupVersionKind, namespace string, name string, err error) error {
	if err != nil && errors.IsNotFound(err) {
		c.lock.Lock()
		defer c.lock.Unlock()
		if _, ok := c.nodes[kube.NewResourceKey(gvk.Group, gvk.Kind, namespace, name)]; ok {
			if c.syncTime != nil {
				c.log.Warn("Dropped stale cache")
				c.syncTime = nil
			}
		}
	}
	return err
}

func (c *clusterInfo) processEvent(event watch.EventType, un *unstructured.Unstructured) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	key := kube.GetResourceKey(un)
	existingNode, exists := c.nodes[key]
	if event == watch.Deleted {
		if exists {
			c.removeNode(key)
			if existingNode.appName != "" {
				c.onAppUpdated(existingNode.appName)
			}
		}
	} else if event != watch.Deleted {
		nodes := make([]*node, 0)
		if exists {
			nodes = append(nodes, existingNode)
		}
		newObj := createObjInfo(un)
		c.setNode(newObj)
		nodes = append(nodes, newObj)

		toNotify := make(map[string]bool)
		for i := range nodes {
			n := nodes[i]
			if ns, ok := c.nsIndex[n.ref.Namespace]; ok {
				app := n.getApp(ns)
				if app != "" {
					toNotify[app] = true
				}
			}
		}

		for name := range toNotify {
			c.onAppUpdated(name)
		}
	}

	return nil
}
