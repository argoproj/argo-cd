package cache

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"

	"github.com/argoproj/argo-cd/controller/metrics"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/kube"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/pager"
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
	syncTime      *time.Time
	syncError     error
	apisMeta      map[schema.GroupKind]*apiMeta
	serverVersion string
	apiGroups     []metav1.APIGroup
	// namespacedResources is a simple map which indicates a groupKind is namespaced
	namespacedResources map[schema.GroupKind]bool

	// lock is a rw lock which protects the fields of clusterInfo
	lock    *sync.RWMutex
	nodes   map[kube.ResourceKey]*node
	nsIndex map[string]map[kube.ResourceKey]*node

	onObjectUpdated  ObjectUpdatedHandler
	onEventReceived  func(event watch.EventType, un *unstructured.Unstructured)
	kubectl          kube.Kubectl
	cluster          *appv1.Cluster
	log              *log.Entry
	cacheSettingsSrc func() *cacheSettings
	metricsServer    *metrics.MetricsServer
}

func (c *clusterInfo) replaceResourceCache(gk schema.GroupKind, resourceVersion string, objs []unstructured.Unstructured, ns string) {
	info, ok := c.apisMeta[gk]
	if ok {
		objByKey := make(map[kube.ResourceKey]*unstructured.Unstructured)
		for i := range objs {
			objByKey[kube.GetResourceKey(&objs[i])] = &objs[i]
		}

		// update existing nodes
		for i := range objs {
			obj := &objs[i]
			key := kube.GetResourceKey(&objs[i])
			existingNode, exists := c.nodes[key]
			c.onNodeUpdated(exists, existingNode, obj)
		}

		// remove existing nodes that a no longer exist
		for key, existingNode := range c.nodes {
			if key.Kind != gk.Kind || key.Group != gk.Group || ns != "" && key.Namespace != ns {
				continue
			}

			if _, ok := objByKey[key]; !ok {
				c.onNodeRemoved(key, existingNode)
			}
		}
		info.resourceVersion = resourceVersion
	}
}

func isServiceAccountTokenSecret(un *unstructured.Unstructured) (bool, metav1.OwnerReference) {
	ref := metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       kube.ServiceAccountKind,
	}
	if un.GetKind() != kube.SecretKind || un.GroupVersionKind().Group != "" {
		return false, ref
	}

	if typeVal, ok, err := unstructured.NestedString(un.Object, "type"); !ok || err != nil || typeVal != "kubernetes.io/service-account-token" {
		return false, ref
	}

	annotations := un.GetAnnotations()
	if annotations == nil {
		return false, ref
	}

	id, okId := annotations["kubernetes.io/service-account.uid"]
	name, okName := annotations["kubernetes.io/service-account.name"]
	if okId && okName {
		ref.Name = name
		ref.UID = types.UID(id)
	}
	return ref.Name != "" && ref.UID != "", ref
}

func (c *clusterInfo) createObjInfo(un *unstructured.Unstructured, appInstanceLabel string) *node {
	ownerRefs := un.GetOwnerReferences()
	gvk := un.GroupVersionKind()
	// Special case for endpoint. Remove after https://github.com/kubernetes/kubernetes/issues/28483 is fixed
	if gvk.Group == "" && gvk.Kind == kube.EndpointsKind && len(un.GetOwnerReferences()) == 0 {
		ownerRefs = append(ownerRefs, metav1.OwnerReference{
			Name:       un.GetName(),
			Kind:       kube.ServiceKind,
			APIVersion: "v1",
		})
	}

	// Special case for Operator Lifecycle Manager ClusterServiceVersion:
	if un.GroupVersionKind().Group == "operators.coreos.com" && un.GetKind() == "ClusterServiceVersion" {
		if un.GetAnnotations()["olm.operatorGroup"] != "" {
			ownerRefs = append(ownerRefs, metav1.OwnerReference{
				Name:       un.GetAnnotations()["olm.operatorGroup"],
				Kind:       "OperatorGroup",
				APIVersion: "operators.coreos.com/v1",
			})
		}
	}

	// edge case. Consider auto-created service account tokens as a child of service account objects
	if yes, ref := isServiceAccountTokenSecret(un); yes {
		ownerRefs = append(ownerRefs, ref)
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
	} else {
		// edge case. we do not label CRDs, so they miss the tracking label we inject. But we still
		// want the full resource to be available in our cache (to diff), so we store all CRDs
		switch gvk.Kind {
		case kube.CustomResourceDefinitionKind:
			nodeInfo.resource = un
		}
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
	c.lock.Lock()
	defer c.lock.Unlock()
	c.syncTime = nil
	for i := range c.apisMeta {
		c.apisMeta[i].watchCancel()
	}
	c.apisMeta = nil
	c.namespacedResources = nil
	c.log.Warnf("invalidated cluster")
}

func (c *clusterInfo) synced() bool {
	syncTime := c.syncTime
	if syncTime == nil {
		return false
	}
	if c.syncError != nil {
		return time.Now().Before(syncTime.Add(clusterRetryTimeout))
	}
	return time.Now().Before(syncTime.Add(clusterSyncTimeout))
}

func (c *clusterInfo) stopWatching(gk schema.GroupKind, ns string) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if info, ok := c.apisMeta[gk]; ok {
		info.watchCancel()
		delete(c.apisMeta, gk)
		c.replaceResourceCache(gk, "", []unstructured.Unstructured{}, ns)
		c.log.Warnf("Stop watching: %s not found", gk)
	}
}

// startMissingWatches lists supported cluster resources and start watching for changes unless watch is already running
func (c *clusterInfo) startMissingWatches() error {
	config := c.cluster.RESTConfig()

	apis, err := c.kubectl.GetAPIResources(config, c.cacheSettingsSrc().ResourcesFilter)
	if err != nil {
		return err
	}
	client, err := c.kubectl.NewDynamicClient(config)
	if err != nil {
		return err
	}
	namespacedResources := make(map[schema.GroupKind]bool)
	for i := range apis {
		api := apis[i]
		namespacedResources[api.GroupKind] = api.Meta.Namespaced
		if _, ok := c.apisMeta[api.GroupKind]; !ok {
			ctx, cancel := context.WithCancel(context.Background())
			info := &apiMeta{namespaced: api.Meta.Namespaced, watchCancel: cancel}
			c.apisMeta[api.GroupKind] = info

			err = c.processApi(client, api, func(resClient dynamic.ResourceInterface, ns string) error {
				go c.watchEvents(ctx, api, info, resClient, ns)
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	c.namespacedResources = namespacedResources
	return nil
}

func runSynced(lock sync.Locker, action func() error) error {
	lock.Lock()
	defer lock.Unlock()
	return action()
}

func (c *clusterInfo) watchEvents(ctx context.Context, api kube.APIResourceInfo, info *apiMeta, resClient dynamic.ResourceInterface, ns string) {
	util.RetryUntilSucceed(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
			}
		}()

		err = runSynced(c.lock, func() error {
			if info.resourceVersion == "" {
				listPager := pager.New(func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
					res, err := resClient.List(opts)
					if err == nil {
						info.resourceVersion = res.GetResourceVersion()
					}
					return res, err
				})
				var items []unstructured.Unstructured
				err = listPager.EachListItem(ctx, metav1.ListOptions{}, func(obj runtime.Object) error {
					if un, ok := obj.(*unstructured.Unstructured); !ok {
						return fmt.Errorf("object %s/%s has an unexpected type", un.GroupVersionKind().String(), un.GetName())
					} else {
						items = append(items, *un)
					}
					return nil
				})
				if err != nil {
					return fmt.Errorf("failed to load initial state of resource %s: %v", api.GroupKind.String(), err)
				}
				c.replaceResourceCache(api.GroupKind, info.resourceVersion, items, ns)
			}
			return nil
		})

		if err != nil {
			return err
		}

		w, err := resClient.Watch(metav1.ListOptions{ResourceVersion: info.resourceVersion})
		if errors.IsNotFound(err) {
			c.stopWatching(api.GroupKind, ns)
			return nil
		}
		if errors.IsGone(err) {
			info.resourceVersion = ""
			c.log.Warnf("Resource version of %s is too old", api.GroupKind)
		}
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
								c.stopWatching(gk, ns)
							}
						} else {
							err = runSynced(c.lock, func() error {
								return c.startMissingWatches()
							})

						}
					}
					if err != nil {
						c.log.Warnf("Failed to start missing watch: %v", err)
					}
				} else {
					return fmt.Errorf("Watch %s on %s has closed", api.GroupKind, c.cluster.Server)
				}
			}
		}

	}, fmt.Sprintf("watch %s on %s", api.GroupKind, c.cluster.Server), ctx, watchResourcesRetryTimeout)
}

func (c *clusterInfo) processApi(client dynamic.Interface, api kube.APIResourceInfo, callback func(resClient dynamic.ResourceInterface, ns string) error) error {
	resClient := client.Resource(api.GroupVersionResource)
	if len(c.cluster.Namespaces) == 0 {
		return callback(resClient, "")
	}

	if !api.Meta.Namespaced {
		return nil
	}

	for _, ns := range c.cluster.Namespaces {
		err := callback(resClient.Namespace(ns), ns)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *clusterInfo) sync() (err error) {

	c.log.Info("Start syncing cluster")

	for i := range c.apisMeta {
		c.apisMeta[i].watchCancel()
	}
	c.apisMeta = make(map[schema.GroupKind]*apiMeta)
	c.nodes = make(map[kube.ResourceKey]*node)
	c.namespacedResources = make(map[schema.GroupKind]bool)
	config := c.cluster.RESTConfig()
	version, err := c.kubectl.GetServerVersion(config)
	if err != nil {
		return err
	}
	c.serverVersion = version
	groups, err := c.kubectl.GetAPIGroups(config)
	if err != nil {
		return err
	}
	c.apiGroups = groups

	apis, err := c.kubectl.GetAPIResources(config, c.cacheSettingsSrc().ResourcesFilter)
	if err != nil {
		return err
	}
	client, err := c.kubectl.NewDynamicClient(config)
	if err != nil {
		return err
	}
	lock := sync.Mutex{}
	err = util.RunAllAsync(len(apis), func(i int) error {
		api := apis[i]

		lock.Lock()
		ctx, cancel := context.WithCancel(context.Background())
		info := &apiMeta{namespaced: api.Meta.Namespaced, watchCancel: cancel}
		c.apisMeta[api.GroupKind] = info
		c.namespacedResources[api.GroupKind] = api.Meta.Namespaced
		lock.Unlock()

		return c.processApi(client, api, func(resClient dynamic.ResourceInterface, ns string) error {

			listPager := pager.New(func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
				res, err := resClient.List(opts)
				if err == nil {
					lock.Lock()
					info.resourceVersion = res.GetResourceVersion()
					lock.Unlock()
				}
				return res, err
			})

			err = listPager.EachListItem(context.Background(), metav1.ListOptions{}, func(obj runtime.Object) error {
				if un, ok := obj.(*unstructured.Unstructured); !ok {
					return fmt.Errorf("object %s/%s has an unexpected type", un.GroupVersionKind().String(), un.GetName())
				} else {
					lock.Lock()
					c.setNode(c.createObjInfo(un, c.cacheSettingsSrc().AppInstanceLabelKey))
					lock.Unlock()
				}
				return nil
			})

			if err != nil {
				return fmt.Errorf("failed to load initial state of resource %s: %v", api.GroupKind.String(), err)
			}

			go c.watchEvents(ctx, api, info, resClient, ns)
			return nil
		})
	})

	if err != nil {
		log.Errorf("Failed to sync cluster %s: %v", c.cluster.Server, err)
		return err
	}

	c.log.Info("Cluster successfully synced")
	return nil
}

func (c *clusterInfo) ensureSynced() error {
	// first check if cluster is synced *without lock*
	if c.synced() {
		return c.syncError
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	// before doing any work, check once again now that we have the lock, to see if it got
	// synced between the first check and now
	if c.synced() {
		return c.syncError
	}
	err := c.sync()
	syncTime := time.Now()
	c.syncTime = &syncTime
	c.syncError = err
	return c.syncError
}

func (c *clusterInfo) getNamespaceTopLevelResources(namespace string) map[kube.ResourceKey]appv1.ResourceNode {
	c.lock.RLock()
	defer c.lock.RUnlock()
	nodes := make(map[kube.ResourceKey]appv1.ResourceNode)
	for _, node := range c.nsIndex[namespace] {
		if len(node.ownerRefs) == 0 {
			nodes[node.resourceKey()] = node.asResourceNode()
		}
	}
	return nodes
}

func (c *clusterInfo) iterateHierarchy(key kube.ResourceKey, action func(child appv1.ResourceNode, appName string)) {
	c.lock.Lock()
	defer c.lock.Unlock()
	if objInfo, ok := c.nodes[key]; ok {
		nsNodes := c.nsIndex[key.Namespace]
		action(objInfo.asResourceNode(), objInfo.getApp(nsNodes))
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
				action(child.asResourceNode(), child.getApp(nsNodes))
				child.iterateChildren(nsNodes, map[kube.ResourceKey]bool{objInfo.resourceKey(): true}, action)
			}
		}
	}
}

func (c *clusterInfo) isNamespaced(gk schema.GroupKind) bool {
	// this is safe to access without a lock since we always replace the entire map instead of mutating keys
	if isNamespaced, ok := c.namespacedResources[gk]; ok {
		return isNamespaced
	}
	log.Warnf("group/kind %s scope is unknown (known objects: %d). assuming namespaced object", gk, len(c.namespacedResources))
	return true
}

func (c *clusterInfo) getManagedLiveObjs(a *appv1.Application, targetObjs []*unstructured.Unstructured) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	managedObjs := make(map[kube.ResourceKey]*unstructured.Unstructured)
	// iterate all objects in live state cache to find ones associated with app
	for key, o := range c.nodes {
		if o.appName == a.Name && o.resource != nil && len(o.ownerRefs) == 0 {
			managedObjs[key] = o.resource
		}
	}
	config := metrics.AddMetricsTransportWrapper(c.metricsServer, a, c.cluster.RESTConfig())
	// iterate target objects and identify ones that already exist in the cluster,
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
	if c.onEventReceived != nil {
		c.onEventReceived(event, un)
	}
	key := kube.GetResourceKey(un)
	if event == watch.Modified && skipAppRequeing(key) {
		return
	}
	c.lock.Lock()
	defer c.lock.Unlock()
	existingNode, exists := c.nodes[key]
	if event == watch.Deleted {
		if exists {
			c.onNodeRemoved(key, existingNode)
		}
	} else if event != watch.Deleted {
		c.onNodeUpdated(exists, existingNode, un)
	}
}

func (c *clusterInfo) onNodeUpdated(exists bool, existingNode *node, un *unstructured.Unstructured) {
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
			if app == "" {
				continue
			}
			toNotify[app] = n.isRootAppNode() || toNotify[app]
		}
	}
	c.onObjectUpdated(toNotify, newObj.ref)
}

func (c *clusterInfo) onNodeRemoved(key kube.ResourceKey, n *node) {
	appName := n.appName
	if ns, ok := c.nsIndex[key.Namespace]; ok {
		appName = n.getApp(ns)
	}

	c.removeNode(key)
	managedByApp := make(map[string]bool)
	if appName != "" {
		managedByApp[appName] = n.isRootAppNode()
	}
	c.onObjectUpdated(managedByApp, n.ref)
}

var (
	ignoredRefreshResources = map[string]bool{
		"/" + kube.EndpointsKind: true,
	}
)

func (c *clusterInfo) getClusterInfo() metrics.ClusterInfo {
	c.lock.RLock()
	defer c.lock.RUnlock()
	return metrics.ClusterInfo{
		APIsCount:         len(c.apisMeta),
		K8SVersion:        c.serverVersion,
		ResourcesCount:    len(c.nodes),
		Server:            c.cluster.Server,
		LastCacheSyncTime: c.syncTime,
	}
}

// skipAppRequeing checks if the object is an API type which we want to skip requeuing against.
// We ignore API types which have a high churn rate, and/or whose updates are irrelevant to the app
func skipAppRequeing(key kube.ResourceKey) bool {
	return ignoredRefreshResources[key.Group+"/"+key.Kind]
}
