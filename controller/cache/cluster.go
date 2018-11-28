package cache

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/kube"

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
	nodes        map[kube.ResourceKey]*node
	lock         *sync.Mutex
	onAppUpdated func(appName string)
	kubectl      kube.Kubectl
	cluster      *appv1.Cluster
	syncLock     *sync.Mutex
	syncTime     *time.Time
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
		parents:   make(map[kube.ResourceKey]*node),
		children:  make(map[kube.ResourceKey]*node),
		tags:      getTags(un),
	}
	if labels := un.GetLabels(); len(ownerRefs) == 0 && labels != nil && labels[common.LabelApplicationName] != "" {
		info.appName = labels[common.LabelApplicationName]
		info.resource = un
	}
	return info
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
	log.Infof("Start syncing cluster %s", c.cluster.Server)
	c.nodes = make(map[kube.ResourceKey]*node)
	resources, err := c.kubectl.GetResources(c.cluster.RESTConfig(), "")
	if err != nil {
		log.Errorf("Failed to sync cluster %s: %v", c.cluster.Server, err)
		return err
	}

	for i := range resources {
		c.nodes[kube.GetResourceKey(resources[i])] = createObjInfo(resources[i])
	}

	nodes := make(map[kube.ResourceKey]*node)
	for k, v := range c.nodes {
		nodes[k] = v
	}
	for _, obj := range c.nodes {
		if len(obj.ownerRefs) == 0 {
			obj.fillChildren(nodes)
		}
	}
	resyncTime := time.Now()
	c.syncTime = &resyncTime
	log.Infof("Cluster %s successfully synced", c.cluster.Server)
	return nil
}

func (c *clusterInfo) getChildren(obj *unstructured.Unstructured) []appv1.ResourceNode {
	c.lock.Lock()
	defer c.lock.Unlock()
	children := make([]appv1.ResourceNode, 0)
	if objInfo, ok := c.nodes[kube.GetResourceKey(obj)]; ok {
		for _, child := range objInfo.children {
			children = append(children, child.childResourceNodes())
		}
	}
	return children
}

func (c *clusterInfo) getControlledLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	controlledObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
	for key, o := range c.nodes {
		if o.appName == a.Name && o.resource != nil && len(o.parents) == 0 {
			controlledObjs[key] = o.resource
		}
	}
	lock := &sync.Mutex{}
	err := util.RunAllAsync(len(targetObjs), func(i int) error {
		targetObj := targetObjs[i]
		key := kube.GetResourceKeyNS(targetObj, util.FirstNonEmpty(targetObj.GetNamespace(), a.Spec.Destination.Namespace))
		lock.Lock()
		controlledObj := controlledObjs[key]
		lock.Unlock()

		if controlledObj == nil {
			if existingObj, exists := c.nodes[key]; exists {
				if existingObj.resource != nil {
					controlledObj = existingObj.resource
				} else {
					var err error
					controlledObj, err = c.kubectl.GetResource(c.cluster.RESTConfig(), targetObj.GroupVersionKind(), existingObj.ref.Name, existingObj.ref.Namespace)
					if err != nil && !errors.IsNotFound(err) {
						return err
					}
				}
			}
		}

		if controlledObj != nil {
			controlledObj, err := c.kubectl.ConvertToVersion(controlledObj, targetObj.GroupVersionKind().Group, targetObj.GroupVersionKind().Version)
			if err != nil {
				return err
			}
			lock.Lock()
			controlledObjs[key] = controlledObj
			lock.Unlock()
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return controlledObjs, nil
}

func ownerRefGV(ownerRef metav1.OwnerReference) schema.GroupVersion {
	gv, err := schema.ParseGroupVersion(ownerRef.APIVersion)
	if err != nil {
		gv = schema.GroupVersion{}
	}
	return gv
}

func (c *clusterInfo) processEvent(event watch.EventType, un *unstructured.Unstructured) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	obj, exists := c.nodes[kube.GetResourceKey(un)]
	if exists && event == watch.Deleted {
		for i := range obj.parents {
			delete(obj.parents[i].children, obj.resourceKey())
		}
		for i := range obj.children {
			delete(obj.children[i].parents, obj.resourceKey())
		}
		delete(c.nodes, kube.GetResourceKey(un))
		if obj.appName != "" {
			c.onAppUpdated(obj.appName)
		}
	} else if !exists && event != watch.Deleted {
		newObj := createObjInfo(un)
		c.nodes[newObj.resourceKey()] = newObj
		if len(newObj.ownerRefs) > 0 {
			sameNamespace := make(map[kube.ResourceKey]*node)
			for k := range c.nodes {
				if c.nodes[k].ref.Namespace == un.GetNamespace() {
					sameNamespace[k] = c.nodes[k]
				}
			}
			for _, ownerRef := range newObj.ownerRefs {
				if owner, ok := sameNamespace[kube.NewResourceKey(ownerRefGV(ownerRef).Group, ownerRef.Kind, un.GetNamespace(), ownerRef.Name)]; ok {
					owner.fillChildren(sameNamespace)
				}
			}
		}
		if newObj.appName != "" {
			c.onAppUpdated(newObj.appName)
		}
	} else if exists {
		obj.resourceVersion = un.GetResourceVersion()
		toNotify := make([]string, 0)
		if obj.appName != "" {
			toNotify = append(toNotify, obj.appName)
		}

		if len(obj.ownerRefs) == 0 {
			newAppName := ""
			if un.GetLabels() != nil {
				newAppName = un.GetLabels()[common.LabelApplicationName]
			}
			if newAppName != obj.appName {
				obj.setAppName(newAppName)
				if newAppName != "" {
					toNotify = append(toNotify, newAppName)
				}
			}
		}

		if len(obj.parents) == 0 && obj.appName != "" {
			obj.resource = un
		} else {
			obj.resource = nil
		}
		obj.tags = getTags(un)

		for _, name := range toNotify {
			c.onAppUpdated(name)
		}
	}

	return nil
}
