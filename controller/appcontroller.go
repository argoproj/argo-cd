package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"reflect"
	"runtime/debug"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	clustercache "github.com/argoproj/gitops-engine/pkg/cache"
	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	resourceutil "github.com/argoproj/gitops-engine/pkg/sync/resource"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	jsonpatch "github.com/evanphx/json-patch"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/semaphore"
	v1 "k8s.io/api/core/v1"
	apierr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/v2/common"
	statecache "github.com/argoproj/argo-cd/v2/controller/cache"
	"github.com/argoproj/argo-cd/v2/controller/metrics"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
	argodiff "github.com/argoproj/argo-cd/v2/util/argo/diff"
	appstatecache "github.com/argoproj/argo-cd/v2/util/cache/appstate"
	"github.com/argoproj/argo-cd/v2/util/db"
	"github.com/argoproj/argo-cd/v2/util/errors"
	"github.com/argoproj/argo-cd/v2/util/glob"
	"github.com/argoproj/argo-cd/v2/util/helm"
	logutils "github.com/argoproj/argo-cd/v2/util/log"
	settings_util "github.com/argoproj/argo-cd/v2/util/settings"
)

const (
	updateOperationStateTimeout = 1 * time.Second
	// orphanedIndex contains application which monitor orphaned resources by namespace
	orphanedIndex = "orphaned"
)

type CompareWith int

const (
	// Compare live application state against state defined in latest git revision with no resolved revision caching.
	CompareWithLatestForceResolve CompareWith = 3
	// Compare live application state against state defined in latest git revision.
	CompareWithLatest CompareWith = 2
	// Compare live application state against state defined using revision of most recent comparison.
	CompareWithRecent CompareWith = 1
	// Skip comparison and only refresh application resources tree
	ComparisonWithNothing CompareWith = 0
)

func (a CompareWith) Max(b CompareWith) CompareWith {
	return CompareWith(math.Max(float64(a), float64(b)))
}

func (a CompareWith) Pointer() *CompareWith {
	return &a
}

// ApplicationController is the controller for application resources.
type ApplicationController struct {
	cache                *appstatecache.Cache
	namespace            string
	kubeClientset        kubernetes.Interface
	kubectl              kube.Kubectl
	applicationClientset appclientset.Interface
	auditLogger          *argo.AuditLogger
	// queue contains app namespace/name
	appRefreshQueue workqueue.RateLimitingInterface
	// queue contains app namespace/name/comparisonType and used to request app refresh with the predefined comparison type
	appComparisonTypeRefreshQueue workqueue.RateLimitingInterface
	appOperationQueue             workqueue.RateLimitingInterface
	projectRefreshQueue           workqueue.RateLimitingInterface
	appInformer                   cache.SharedIndexInformer
	appLister                     applisters.ApplicationLister
	projInformer                  cache.SharedIndexInformer
	appStateManager               AppStateManager
	stateCache                    statecache.LiveStateCache
	statusRefreshTimeout          time.Duration
	statusHardRefreshTimeout      time.Duration
	selfHealTimeout               time.Duration
	repoClientset                 apiclient.Clientset
	db                            db.ArgoDB
	settingsMgr                   *settings_util.SettingsManager
	refreshRequestedApps          map[string]CompareWith
	refreshRequestedAppsMutex     *sync.Mutex
	metricsServer                 *metrics.MetricsServer
	kubectlSemaphore              *semaphore.Weighted
	clusterFilter                 func(cluster *appv1.Cluster) bool
	projByNameCache               sync.Map
	applicationNamespaces         []string
}

// NewApplicationController creates new instance of ApplicationController.
func NewApplicationController(
	namespace string,
	settingsMgr *settings_util.SettingsManager,
	kubeClientset kubernetes.Interface,
	applicationClientset appclientset.Interface,
	repoClientset apiclient.Clientset,
	argoCache *appstatecache.Cache,
	kubectl kube.Kubectl,
	appResyncPeriod time.Duration,
	appHardResyncPeriod time.Duration,
	selfHealTimeout time.Duration,
	metricsPort int,
	metricsCacheExpiration time.Duration,
	metricsApplicationLabels []string,
	kubectlParallelismLimit int64,
	persistResourceHealth bool,
	clusterFilter func(cluster *appv1.Cluster) bool,
	applicationNamespaces []string,
) (*ApplicationController, error) {
	log.Infof("appResyncPeriod=%v, appHardResyncPeriod=%v", appResyncPeriod, appHardResyncPeriod)
	db := db.NewDB(namespace, settingsMgr, kubeClientset)
	ctrl := ApplicationController{
		cache:                         argoCache,
		namespace:                     namespace,
		kubeClientset:                 kubeClientset,
		kubectl:                       kubectl,
		applicationClientset:          applicationClientset,
		repoClientset:                 repoClientset,
		appRefreshQueue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "app_reconciliation_queue"),
		appOperationQueue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "app_operation_processing_queue"),
		projectRefreshQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "project_reconciliation_queue"),
		appComparisonTypeRefreshQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		db:                            db,
		statusRefreshTimeout:          appResyncPeriod,
		statusHardRefreshTimeout:      appHardResyncPeriod,
		refreshRequestedApps:          make(map[string]CompareWith),
		refreshRequestedAppsMutex:     &sync.Mutex{},
		auditLogger:                   argo.NewAuditLogger(namespace, kubeClientset, "argocd-application-controller"),
		settingsMgr:                   settingsMgr,
		selfHealTimeout:               selfHealTimeout,
		clusterFilter:                 clusterFilter,
		projByNameCache:               sync.Map{},
		applicationNamespaces:         applicationNamespaces,
	}
	if kubectlParallelismLimit > 0 {
		ctrl.kubectlSemaphore = semaphore.NewWeighted(kubectlParallelismLimit)
	}
	kubectl.SetOnKubectlRun(ctrl.onKubectlRun)
	appInformer, appLister := ctrl.newApplicationInformerAndLister()
	indexers := cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}
	projInformer := v1alpha1.NewAppProjectInformer(applicationClientset, namespace, appResyncPeriod, indexers)
	projInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(obj); err == nil {
				ctrl.projectRefreshQueue.Add(key)
				if projMeta, ok := obj.(metav1.Object); ok {
					ctrl.InvalidateProjectsCache(projMeta.GetName())
				}

			}
		},
		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				ctrl.projectRefreshQueue.Add(key)
				if projMeta, ok := new.(metav1.Object); ok {
					ctrl.InvalidateProjectsCache(projMeta.GetName())
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
				ctrl.projectRefreshQueue.Add(key)
				if projMeta, ok := obj.(metav1.Object); ok {
					ctrl.InvalidateProjectsCache(projMeta.GetName())
				}
			}
		},
	})
	metricsAddr := fmt.Sprintf("0.0.0.0:%d", metricsPort)
	var err error
	ctrl.metricsServer, err = metrics.NewMetricsServer(metricsAddr, appLister, ctrl.canProcessApp, func(r *http.Request) error {
		return nil
	}, metricsApplicationLabels)
	if err != nil {
		return nil, err
	}
	if metricsCacheExpiration.Seconds() != 0 {
		err = ctrl.metricsServer.SetExpiration(metricsCacheExpiration)
		if err != nil {
			return nil, err
		}
	}
	stateCache := statecache.NewLiveStateCache(db, appInformer, ctrl.settingsMgr, kubectl, ctrl.metricsServer, ctrl.handleObjectUpdated, clusterFilter, argo.NewResourceTracking())
	appStateManager := NewAppStateManager(db, applicationClientset, repoClientset, namespace, kubectl, ctrl.settingsMgr, stateCache, projInformer, ctrl.metricsServer, argoCache, ctrl.statusRefreshTimeout, argo.NewResourceTracking(), persistResourceHealth)
	ctrl.appInformer = appInformer
	ctrl.appLister = appLister
	ctrl.projInformer = projInformer
	ctrl.appStateManager = appStateManager
	ctrl.stateCache = stateCache

	return &ctrl, nil
}

func (ctrl *ApplicationController) InvalidateProjectsCache(names ...string) {
	if len(names) > 0 {
		for _, name := range names {
			ctrl.projByNameCache.Delete(name)
		}
	} else {
		ctrl.projByNameCache.Range(func(key, _ interface{}) bool {
			ctrl.projByNameCache.Delete(key)
			return true
		})
	}
}

func (ctrl *ApplicationController) GetMetricsServer() *metrics.MetricsServer {
	return ctrl.metricsServer
}

func (ctrl *ApplicationController) onKubectlRun(command string) (kube.CleanupFunc, error) {
	ctrl.metricsServer.IncKubectlExec(command)
	if ctrl.kubectlSemaphore != nil {
		if err := ctrl.kubectlSemaphore.Acquire(context.Background(), 1); err != nil {
			return nil, err
		}
		ctrl.metricsServer.IncKubectlExecPending(command)
	}
	return func() {
		if ctrl.kubectlSemaphore != nil {
			ctrl.kubectlSemaphore.Release(1)
			ctrl.metricsServer.DecKubectlExecPending(command)
		}
	}, nil
}

func isSelfReferencedApp(app *appv1.Application, ref v1.ObjectReference) bool {
	gvk := ref.GroupVersionKind()
	return ref.UID == app.UID &&
		ref.Name == app.Name &&
		ref.Namespace == app.Namespace &&
		gvk.Group == application.Group &&
		gvk.Kind == application.ApplicationKind
}

func (ctrl *ApplicationController) newAppProjCache(name string) *appProjCache {
	return &appProjCache{name: name, ctrl: ctrl}
}

type appProjCache struct {
	name string
	ctrl *ApplicationController

	lock    sync.Mutex
	appProj *appv1.AppProject
}

// GetAppProject gets an AppProject from the cache. If the AppProject is not
// yet cached, retrieves the AppProject from the K8s control plane and stores
// in the cache.
func (projCache *appProjCache) GetAppProject(ctx context.Context) (*appv1.AppProject, error) {
	projCache.lock.Lock()
	defer projCache.lock.Unlock()
	if projCache.appProj != nil {
		return projCache.appProj, nil
	}
	proj, err := argo.GetAppProjectByName(projCache.name, applisters.NewAppProjectLister(projCache.ctrl.projInformer.GetIndexer()), projCache.ctrl.namespace, projCache.ctrl.settingsMgr, projCache.ctrl.db, ctx)
	if err != nil {
		return nil, err
	}
	projCache.appProj = proj
	return projCache.appProj, nil
}

// getAppProj gets the AppProject for the given Application app.
func (ctrl *ApplicationController) getAppProj(app *appv1.Application) (*appv1.AppProject, error) {
	projCache, _ := ctrl.projByNameCache.LoadOrStore(app.Spec.GetProject(), ctrl.newAppProjCache(app.Spec.GetProject()))
	proj, err := projCache.(*appProjCache).GetAppProject(context.TODO())
	if err != nil {
		if apierr.IsNotFound(err) {
			return nil, err
		} else {
			return nil, fmt.Errorf("could not retrieve AppProject '%s' from cache: %v", app.Spec.Project, err)
		}
	}
	if !proj.IsAppNamespacePermitted(app, ctrl.namespace) {
		return nil, argo.ErrProjectNotPermitted(app.GetName(), app.GetNamespace(), proj.GetName())
	}
	return proj, nil
}

func (ctrl *ApplicationController) handleObjectUpdated(managedByApp map[string]bool, ref v1.ObjectReference) {
	// if namespaced resource is not managed by any app it might be orphaned resource of some other apps
	if len(managedByApp) == 0 && ref.Namespace != "" {
		// retrieve applications which monitor orphaned resources in the same namespace and refresh them unless resource is denied in app project
		if objs, err := ctrl.appInformer.GetIndexer().ByIndex(orphanedIndex, ref.Namespace); err == nil {
			for i := range objs {
				app, ok := objs[i].(*appv1.Application)
				if !ok {
					continue
				}

				managedByApp[app.InstanceName(ctrl.namespace)] = true
			}
		}
	}
	for appName, isManagedResource := range managedByApp {
		// The appName is given as <namespace>_<name>, but the indexer needs it
		// format <namespace>/<name>
		appKey := ctrl.toAppKey(appName)
		obj, exists, err := ctrl.appInformer.GetIndexer().GetByKey(appKey)
		app, ok := obj.(*appv1.Application)
		if exists && err == nil && ok && isSelfReferencedApp(app, ref) {
			// Don't force refresh app if related resource is application itself. This prevents infinite reconciliation loop.
			continue
		}

		if !ctrl.canProcessApp(obj) {
			// Don't force refresh app if app belongs to a different controller shard or is outside the allowed namespaces.
			continue
		}

		// Enforce application's permission for the source namespace
		_, err = ctrl.getAppProj(app)
		if err != nil {
			log.Errorf("Unable to determine project for app '%s': %v", app.QualifiedName(), err)
			continue
		}

		level := ComparisonWithNothing
		if isManagedResource {
			level = CompareWithRecent
		}

		// Additional check for debug level so we don't need to evaluate the
		// format string in case of non-debug scenarios
		if log.GetLevel() >= log.DebugLevel {
			var resKey string
			if ref.Namespace != "" {
				resKey = ref.Namespace + "/" + ref.Name
			} else {
				resKey = "(cluster-scoped)/" + ref.Name
			}
			log.Debugf("Refreshing app %s for change in cluster of object %s of type %s/%s", appKey, resKey, ref.APIVersion, ref.Kind)
		}

		ctrl.requestAppRefresh(app.QualifiedName(), &level, nil)
	}
}

// setAppManagedResources will build a list of ResourceDiff based on the provided comparisonResult
// and persist app resources related data in the cache. Will return the persisted ApplicationTree.
func (ctrl *ApplicationController) setAppManagedResources(a *appv1.Application, comparisonResult *comparisonResult) (*appv1.ApplicationTree, error) {
	managedResources, err := ctrl.hideSecretData(a, comparisonResult)
	if err != nil {
		return nil, fmt.Errorf("error getting managed resources: %s", err)
	}
	tree, err := ctrl.getResourceTree(a, managedResources)
	if err != nil {
		return nil, fmt.Errorf("error getting resource tree: %s", err)
	}
	err = ctrl.cache.SetAppResourcesTree(a.InstanceName(ctrl.namespace), tree)
	if err != nil {
		return nil, fmt.Errorf("error setting app resource tree: %s", err)
	}
	err = ctrl.cache.SetAppManagedResources(a.InstanceName(ctrl.namespace), managedResources)
	if err != nil {
		return nil, fmt.Errorf("error setting app managed resources: %s", err)
	}
	return tree, nil
}

// returns true of given resources exist in the namespace by default and not managed by the user
func isKnownOrphanedResourceExclusion(key kube.ResourceKey, proj *appv1.AppProject) bool {
	if key.Namespace == "default" && key.Group == "" && key.Kind == kube.ServiceKind && key.Name == "kubernetes" {
		return true
	}
	if key.Group == "" && key.Kind == kube.ServiceAccountKind && key.Name == "default" {
		return true
	}
	if key.Group == "" && key.Kind == "ConfigMap" && key.Name == "kube-root-ca.crt" {
		return true
	}
	list := proj.Spec.OrphanedResources.Ignore
	for _, item := range list {
		if item.Kind == "" || glob.Match(item.Kind, key.Kind) {
			if glob.Match(item.Group, key.Group) {
				if item.Name == "" || glob.Match(item.Name, key.Name) {
					return true
				}
			}
		}
	}
	return false
}

func (ctrl *ApplicationController) getResourceTree(a *appv1.Application, managedResources []*appv1.ResourceDiff) (*appv1.ApplicationTree, error) {
	nodes := make([]appv1.ResourceNode, 0)
	proj, err := ctrl.getAppProj(a)
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	orphanedNodesMap := make(map[kube.ResourceKey]appv1.ResourceNode)
	warnOrphaned := true
	if proj.Spec.OrphanedResources != nil {
		orphanedNodesMap, err = ctrl.stateCache.GetNamespaceTopLevelResources(a.Spec.Destination.Server, a.Spec.Destination.Namespace)
		if err != nil {
			return nil, fmt.Errorf("failed to get namespace top-level resources: %w", err)
		}
		warnOrphaned = proj.Spec.OrphanedResources.IsWarn()
	}
	for i := range managedResources {
		managedResource := managedResources[i]
		delete(orphanedNodesMap, kube.NewResourceKey(managedResource.Group, managedResource.Kind, managedResource.Namespace, managedResource.Name))
		var live = &unstructured.Unstructured{}
		err := json.Unmarshal([]byte(managedResource.LiveState), &live)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal live state of managed resources: %w", err)
		}
		var target = &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(managedResource.TargetState), &target)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal target state of managed resources: %w", err)
		}

		if live == nil {
			nodes = append(nodes, appv1.ResourceNode{
				ResourceRef: appv1.ResourceRef{
					Version:   target.GroupVersionKind().Version,
					Name:      managedResource.Name,
					Kind:      managedResource.Kind,
					Group:     managedResource.Group,
					Namespace: managedResource.Namespace,
				},
			})
		} else {
			err := ctrl.stateCache.IterateHierarchy(a.Spec.Destination.Server, kube.GetResourceKey(live), func(child appv1.ResourceNode, appName string) bool {
				permitted, _ := proj.IsResourcePermitted(schema.GroupKind{Group: child.ResourceRef.Group, Kind: child.ResourceRef.Kind}, child.Namespace, a.Spec.Destination, func(project string) ([]*appv1.Cluster, error) {
					clusters, err := ctrl.db.GetProjectClusters(context.TODO(), project)
					if err != nil {
						return nil, fmt.Errorf("failed to get project clusters: %w", err)
					}
					return clusters, nil
				})
				if !permitted {
					return false
				}
				nodes = append(nodes, child)
				return true
			})
			if err != nil {
				return nil, fmt.Errorf("failed to iterate resource hierarchy: %w", err)
			}
		}
	}
	orphanedNodes := make([]appv1.ResourceNode, 0)
	for k := range orphanedNodesMap {
		if k.Namespace != "" && proj.IsGroupKindPermitted(k.GroupKind(), true) && !isKnownOrphanedResourceExclusion(k, proj) {
			err := ctrl.stateCache.IterateHierarchy(a.Spec.Destination.Server, k, func(child appv1.ResourceNode, appName string) bool {
				belongToAnotherApp := false
				if appName != "" {
					appKey := ctrl.toAppKey(appName)
					if _, exists, err := ctrl.appInformer.GetIndexer().GetByKey(appKey); exists && err == nil {
						belongToAnotherApp = true
					}
				}

				if belongToAnotherApp {
					return false
				}

				permitted, _ := proj.IsResourcePermitted(schema.GroupKind{Group: child.ResourceRef.Group, Kind: child.ResourceRef.Kind}, child.Namespace, a.Spec.Destination, func(project string) ([]*appv1.Cluster, error) {
					return ctrl.db.GetProjectClusters(context.TODO(), project)
				})

				if !permitted {
					return false
				}
				orphanedNodes = append(orphanedNodes, child)
				return true
			})
			if err != nil {
				return nil, err
			}
		}
	}
	var conditions []appv1.ApplicationCondition
	if len(orphanedNodes) > 0 && warnOrphaned {
		conditions = []appv1.ApplicationCondition{{
			Type:    appv1.ApplicationConditionOrphanedResourceWarning,
			Message: fmt.Sprintf("Application has %d orphaned resources", len(orphanedNodes)),
		}}
	}
	a.Status.SetConditions(conditions, map[appv1.ApplicationConditionType]bool{appv1.ApplicationConditionOrphanedResourceWarning: true})
	sort.Slice(orphanedNodes, func(i, j int) bool {
		return orphanedNodes[i].ResourceRef.String() < orphanedNodes[j].ResourceRef.String()
	})

	hosts, err := ctrl.getAppHosts(a, nodes)
	if err != nil {
		return nil, fmt.Errorf("failed to get app hosts: %w", err)
	}
	return &appv1.ApplicationTree{Nodes: nodes, OrphanedNodes: orphanedNodes, Hosts: hosts}, nil
}

func (ctrl *ApplicationController) getAppHosts(a *appv1.Application, appNodes []appv1.ResourceNode) ([]appv1.HostInfo, error) {
	supportedResourceNames := map[v1.ResourceName]bool{
		v1.ResourceCPU:     true,
		v1.ResourceStorage: true,
		v1.ResourceMemory:  true,
	}
	appPods := map[kube.ResourceKey]bool{}
	for _, node := range appNodes {
		if node.Group == "" && node.Kind == kube.PodKind {
			appPods[kube.NewResourceKey(node.Group, node.Kind, node.Namespace, node.Name)] = true
		}
	}

	allNodesInfo := map[string]statecache.NodeInfo{}
	allPodsByNode := map[string][]statecache.PodInfo{}
	appPodsByNode := map[string][]statecache.PodInfo{}
	err := ctrl.stateCache.IterateResources(a.Spec.Destination.Server, func(res *clustercache.Resource, info *statecache.ResourceInfo) {
		key := res.ResourceKey()

		switch {
		case info.NodeInfo != nil && key.Group == "" && key.Kind == "Node":
			allNodesInfo[key.Name] = *info.NodeInfo
		case info.PodInfo != nil && key.Group == "" && key.Kind == kube.PodKind:
			if appPods[key] {
				appPodsByNode[info.PodInfo.NodeName] = append(appPodsByNode[info.PodInfo.NodeName], *info.PodInfo)
			} else {
				allPodsByNode[info.PodInfo.NodeName] = append(allPodsByNode[info.PodInfo.NodeName], *info.PodInfo)
			}
		}
	})
	if err != nil {
		return nil, err
	}

	var hosts []appv1.HostInfo
	for nodeName, appPods := range appPodsByNode {
		node, ok := allNodesInfo[nodeName]
		if !ok {
			continue
		}

		neighbors := allPodsByNode[nodeName]

		resources := map[v1.ResourceName]appv1.HostResourceInfo{}
		for name, resource := range node.Capacity {
			info := resources[name]
			info.ResourceName = name
			info.Capacity += resource.MilliValue()
			resources[name] = info
		}

		for _, pod := range appPods {
			for name, resource := range pod.ResourceRequests {
				if !supportedResourceNames[name] {
					continue
				}

				info := resources[name]
				info.RequestedByApp += resource.MilliValue()
				resources[name] = info
			}
		}

		for _, pod := range neighbors {
			for name, resource := range pod.ResourceRequests {
				if !supportedResourceNames[name] || pod.Phase == v1.PodSucceeded || pod.Phase == v1.PodFailed {
					continue
				}
				info := resources[name]
				info.RequestedByNeighbors += resource.MilliValue()
				resources[name] = info
			}
		}

		var resourcesInfo []appv1.HostResourceInfo
		for _, info := range resources {
			if supportedResourceNames[info.ResourceName] && info.Capacity > 0 {
				resourcesInfo = append(resourcesInfo, info)
			}
		}
		sort.Slice(resourcesInfo, func(i, j int) bool {
			return resourcesInfo[i].ResourceName < resourcesInfo[j].ResourceName
		})
		hosts = append(hosts, appv1.HostInfo{Name: nodeName, SystemInfo: node.SystemInfo, ResourcesInfo: resourcesInfo})
	}
	return hosts, nil
}

func (ctrl *ApplicationController) hideSecretData(app *appv1.Application, comparisonResult *comparisonResult) ([]*appv1.ResourceDiff, error) {
	items := make([]*appv1.ResourceDiff, len(comparisonResult.managedResources))
	for i := range comparisonResult.managedResources {
		res := comparisonResult.managedResources[i]
		item := appv1.ResourceDiff{
			Namespace:       res.Namespace,
			Name:            res.Name,
			Group:           res.Group,
			Kind:            res.Kind,
			Hook:            res.Hook,
			ResourceVersion: res.ResourceVersion,
		}

		target := res.Target
		live := res.Live
		resDiff := res.Diff
		if res.Kind == kube.SecretKind && res.Group == "" {
			var err error
			target, live, err = diff.HideSecretData(res.Target, res.Live)
			if err != nil {
				return nil, fmt.Errorf("error hiding secret data: %s", err)
			}
			compareOptions, err := ctrl.settingsMgr.GetResourceCompareOptions()
			if err != nil {
				return nil, fmt.Errorf("error getting resource compare options: %s", err)
			}
			resourceOverrides, err := ctrl.settingsMgr.GetResourceOverrides()
			if err != nil {
				return nil, fmt.Errorf("error getting resource overrides: %s", err)
			}
			appLabelKey, err := ctrl.settingsMgr.GetAppInstanceLabelKey()
			if err != nil {
				return nil, fmt.Errorf("error getting app instance label key: %s", err)
			}
			trackingMethod, err := ctrl.settingsMgr.GetTrackingMethod()
			if err != nil {
				return nil, fmt.Errorf("error getting tracking method: %s", err)
			}

			clusterCache, err := ctrl.stateCache.GetClusterCache(app.Spec.Destination.Server)
			if err != nil {
				return nil, fmt.Errorf("error getting cluster cache: %s", err)
			}
			diffConfig, err := argodiff.NewDiffConfigBuilder().
				WithDiffSettings(app.Spec.IgnoreDifferences, resourceOverrides, compareOptions.IgnoreAggregatedRoles).
				WithTracking(appLabelKey, trackingMethod).
				WithNoCache().
				WithLogger(logutils.NewLogrusLogger(logutils.NewWithCurrentConfig())).
				WithGVKParser(clusterCache.GetGVKParser()).
				Build()
			if err != nil {
				return nil, fmt.Errorf("appcontroller error building diff config: %s", err)
			}

			diffResult, err := argodiff.StateDiff(live, target, diffConfig)
			if err != nil {
				return nil, fmt.Errorf("error applying diff: %s", err)
			}
			resDiff = diffResult
		}

		if live != nil {
			data, err := json.Marshal(live)
			if err != nil {
				return nil, fmt.Errorf("error marshaling live json: %s", err)
			}
			item.LiveState = string(data)
		} else {
			item.LiveState = "null"
		}

		if target != nil {
			data, err := json.Marshal(target)
			if err != nil {
				return nil, fmt.Errorf("error marshaling target json: %s", err)
			}
			item.TargetState = string(data)
		} else {
			item.TargetState = "null"
		}
		item.PredictedLiveState = string(resDiff.PredictedLive)
		item.NormalizedLiveState = string(resDiff.NormalizedLive)
		item.Modified = resDiff.Modified

		items[i] = &item
	}
	return items, nil
}

// Run starts the Application CRD controller.
func (ctrl *ApplicationController) Run(ctx context.Context, statusProcessors int, operationProcessors int) {
	defer runtime.HandleCrash()
	defer ctrl.appRefreshQueue.ShutDown()
	defer ctrl.appComparisonTypeRefreshQueue.ShutDown()
	defer ctrl.appOperationQueue.ShutDown()
	defer ctrl.projectRefreshQueue.ShutDown()

	ctrl.metricsServer.RegisterClustersInfoSource(ctx, ctrl.stateCache)
	ctrl.RegisterClusterSecretUpdater(ctx)

	go ctrl.appInformer.Run(ctx.Done())
	go ctrl.projInformer.Run(ctx.Done())

	errors.CheckError(ctrl.stateCache.Init())

	if !cache.WaitForCacheSync(ctx.Done(), ctrl.appInformer.HasSynced, ctrl.projInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}

	go func() { errors.CheckError(ctrl.stateCache.Run(ctx)) }()
	go func() { errors.CheckError(ctrl.metricsServer.ListenAndServe()) }()

	for i := 0; i < statusProcessors; i++ {
		go wait.Until(func() {
			for ctrl.processAppRefreshQueueItem() {
			}
		}, time.Second, ctx.Done())
	}

	for i := 0; i < operationProcessors; i++ {
		go wait.Until(func() {
			for ctrl.processAppOperationQueueItem() {
			}
		}, time.Second, ctx.Done())
	}

	go wait.Until(func() {
		for ctrl.processAppComparisonTypeQueueItem() {
		}
	}, time.Second, ctx.Done())

	go wait.Until(func() {
		for ctrl.processProjectQueueItem() {
		}
	}, time.Second, ctx.Done())
	<-ctx.Done()
}

// requestAppRefresh adds a request for given app to the refresh queue. appName
// needs to be the qualified name of the application, i.e. <namespace>/<name>.
func (ctrl *ApplicationController) requestAppRefresh(appName string, compareWith *CompareWith, after *time.Duration) {
	key := ctrl.toAppKey(appName)

	if compareWith != nil && after != nil {
		ctrl.appComparisonTypeRefreshQueue.AddAfter(fmt.Sprintf("%s/%d", key, compareWith), *after)
	} else {
		if compareWith != nil {
			ctrl.refreshRequestedAppsMutex.Lock()
			ctrl.refreshRequestedApps[key] = compareWith.Max(ctrl.refreshRequestedApps[key])
			ctrl.refreshRequestedAppsMutex.Unlock()
		}
		if after != nil {
			ctrl.appRefreshQueue.AddAfter(key, *after)
			ctrl.appOperationQueue.AddAfter(key, *after)
		} else {
			ctrl.appRefreshQueue.Add(key)
			ctrl.appOperationQueue.Add(key)
		}
	}
}

func (ctrl *ApplicationController) isRefreshRequested(appName string) (bool, CompareWith) {
	ctrl.refreshRequestedAppsMutex.Lock()
	defer ctrl.refreshRequestedAppsMutex.Unlock()
	level, ok := ctrl.refreshRequestedApps[appName]
	if ok {
		delete(ctrl.refreshRequestedApps, appName)
	}
	return ok, level
}

func (ctrl *ApplicationController) processAppOperationQueueItem() (processNext bool) {
	appKey, shutdown := ctrl.appOperationQueue.Get()
	if shutdown {
		processNext = false
		return
	}
	processNext = true
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
		ctrl.appOperationQueue.Done(appKey)
	}()

	obj, exists, err := ctrl.appInformer.GetIndexer().GetByKey(appKey.(string))
	if err != nil {
		log.Errorf("Failed to get application '%s' from informer index: %+v", appKey, err)
		return
	}
	if !exists {
		// This happens after app was deleted, but the work queue still had an entry for it.
		return
	}
	origApp, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}
	app := origApp.DeepCopy()

	if app.Operation != nil {
		// If we get here, we are about to process an operation, but we cannot rely on informer since it might have stale data.
		// So always retrieve the latest version to ensure it is not stale to avoid unnecessary syncing.
		// We cannot rely on informer since applications might be updated by both application controller and api server.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.ObjectMeta.Namespace).Get(context.Background(), app.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Failed to retrieve latest application state: %v", err)
			return
		}
		app = freshApp
	}

	if app.Operation != nil {
		ctrl.processRequestedAppOperation(app)
	} else if app.DeletionTimestamp != nil && app.CascadedDeletion() {
		_, err = ctrl.finalizeApplicationDeletion(app, func(project string) ([]*appv1.Cluster, error) {
			return ctrl.db.GetProjectClusters(context.Background(), project)
		})
		if err != nil {
			ctrl.setAppCondition(app, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionDeletionError,
				Message: err.Error(),
			})
			message := fmt.Sprintf("Unable to delete application resources: %v", err.Error())
			ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonStatusRefreshed, Type: v1.EventTypeWarning}, message, "")
		}
	}
	return
}

func (ctrl *ApplicationController) processAppComparisonTypeQueueItem() (processNext bool) {
	key, shutdown := ctrl.appComparisonTypeRefreshQueue.Get()
	processNext = true

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
		ctrl.appComparisonTypeRefreshQueue.Done(key)
	}()
	if shutdown {
		processNext = false
		return
	}

	if parts := strings.Split(key.(string), "/"); len(parts) != 3 {
		log.Warnf("Unexpected key format in appComparisonTypeRefreshTypeQueue. Key should consists of namespace/name/comparisonType but got: %s", key.(string))
	} else {
		if compareWith, err := strconv.Atoi(parts[2]); err != nil {
			log.Warnf("Unable to parse comparison type: %v", err)
			return
		} else {
			ctrl.requestAppRefresh(ctrl.toAppQualifiedName(parts[1], parts[0]), CompareWith(compareWith).Pointer(), nil)
		}
	}
	return
}

func (ctrl *ApplicationController) processProjectQueueItem() (processNext bool) {
	key, shutdown := ctrl.projectRefreshQueue.Get()
	processNext = true

	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
		ctrl.projectRefreshQueue.Done(key)
	}()
	if shutdown {
		processNext = false
		return
	}
	obj, exists, err := ctrl.projInformer.GetIndexer().GetByKey(key.(string))
	if err != nil {
		log.Errorf("Failed to get project '%s' from informer index: %+v", key, err)
		return
	}
	if !exists {
		// This happens after appproj was deleted, but the work queue still had an entry for it.
		return
	}
	origProj, ok := obj.(*appv1.AppProject)
	if !ok {
		log.Warnf("Key '%s' in index is not an appproject", key)
		return
	}

	if origProj.DeletionTimestamp != nil && origProj.HasFinalizer() {
		if err := ctrl.finalizeProjectDeletion(origProj.DeepCopy()); err != nil {
			log.Warnf("Failed to finalize project deletion: %v", err)
		}
	}
	return
}

func (ctrl *ApplicationController) finalizeProjectDeletion(proj *appv1.AppProject) error {
	apps, err := ctrl.appLister.Applications(ctrl.namespace).List(labels.Everything())
	if err != nil {
		return fmt.Errorf("error listing applications: %w", err)
	}
	appsCount := 0
	for i := range apps {
		if apps[i].Spec.GetProject() == proj.Name {
			appsCount++
		}
	}
	if appsCount == 0 {
		return ctrl.removeProjectFinalizer(proj)
	} else {
		log.Infof("Cannot remove project '%s' finalizer as is referenced by %d applications", proj.Name, appsCount)
	}
	return nil
}

func (ctrl *ApplicationController) removeProjectFinalizer(proj *appv1.AppProject) error {
	proj.RemoveFinalizer()
	var patch []byte
	patch, _ = json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": proj.Finalizers,
		},
	})
	_, err := ctrl.applicationClientset.ArgoprojV1alpha1().AppProjects(ctrl.namespace).Patch(context.Background(), proj.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

// shouldBeDeleted returns whether a given resource obj should be deleted on cascade delete of application app
func (ctrl *ApplicationController) shouldBeDeleted(app *appv1.Application, obj *unstructured.Unstructured) bool {
	return !kube.IsCRD(obj) && !isSelfReferencedApp(app, kube.GetObjectRef(obj)) &&
		!resourceutil.HasAnnotationOption(obj, synccommon.AnnotationSyncOptions, synccommon.SyncOptionDisableDeletion) &&
		!resourceutil.HasAnnotationOption(obj, helm.ResourcePolicyAnnotation, helm.ResourcePolicyKeep)
}

func (ctrl *ApplicationController) getPermittedAppLiveObjects(app *appv1.Application, proj *appv1.AppProject, projectClusters func(project string) ([]*appv1.Cluster, error)) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	objsMap, err := ctrl.stateCache.GetManagedLiveObjs(app, []*unstructured.Unstructured{})
	if err != nil {
		return nil, err
	}
	// Don't delete live resources which are not permitted in the app project
	for k, v := range objsMap {
		permitted, err := proj.IsLiveResourcePermitted(v, app.Spec.Destination.Server, app.Spec.Destination.Name, projectClusters)

		if err != nil {
			return nil, err
		}

		if !permitted {
			delete(objsMap, k)
		}
	}
	return objsMap, nil
}

func (ctrl *ApplicationController) finalizeApplicationDeletion(app *appv1.Application, projectClusters func(project string) ([]*appv1.Cluster, error)) ([]*unstructured.Unstructured, error) {
	logCtx := log.WithField("application", app.QualifiedName())
	logCtx.Infof("Deleting resources")
	// Get refreshed application info, since informer app copy might be stale
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(context.Background(), app.Name, metav1.GetOptions{})
	if err != nil {
		if !apierr.IsNotFound(err) {
			logCtx.Errorf("Unable to get refreshed application info prior deleting resources: %v", err)
		}
		return nil, nil
	}
	proj, err := ctrl.getAppProj(app)
	if err != nil {
		return nil, err
	}

	// validDestination is true if the Application destination points to a cluster that is managed by Argo CD
	// (and thus either a cluster secret exists for it, or it's local); validDestination is false otherwise.
	validDestination := true

	// Validate the cluster using the Application destination's `name` field, if applicable,
	// and set the Server field, if needed.
	if err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, ctrl.db); err != nil {
		log.Warnf("Unable to validate destination of the Application being deleted: %v", err)
		validDestination = false
	}

	objs := make([]*unstructured.Unstructured, 0)
	var cluster *appv1.Cluster

	// Attempt to validate the destination via its URL
	if validDestination {
		if cluster, err = ctrl.db.GetCluster(context.Background(), app.Spec.Destination.Server); err != nil {
			log.Warnf("Unable to locate cluster URL for Application being deleted: %v", err)
			validDestination = false
		}
	}

	if validDestination {
		// ApplicationDestination points to a valid cluster, so we may clean up the live objects

		objsMap, err := ctrl.getPermittedAppLiveObjects(app, proj, projectClusters)
		if err != nil {
			return nil, err
		}

		for k := range objsMap {
			// Wait for objects pending deletion to complete before proceeding with next sync wave
			if objsMap[k].GetDeletionTimestamp() != nil {
				logCtx.Infof("%d objects remaining for deletion", len(objsMap))
				return objs, nil
			}

			if ctrl.shouldBeDeleted(app, objsMap[k]) {
				objs = append(objs, objsMap[k])
			}
		}

		config := metrics.AddMetricsTransportWrapper(ctrl.metricsServer, app, cluster.RESTConfig())

		filteredObjs := FilterObjectsForDeletion(objs)

		propagationPolicy := metav1.DeletePropagationForeground
		if app.GetPropagationPolicy() == appv1.BackgroundPropagationPolicyFinalizer {
			propagationPolicy = metav1.DeletePropagationBackground
		}
		logCtx.Infof("Deleting application's resources with %s propagation policy", propagationPolicy)

		err = kube.RunAllAsync(len(filteredObjs), func(i int) error {
			obj := filteredObjs[i]
			return ctrl.kubectl.DeleteResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), metav1.DeleteOptions{PropagationPolicy: &propagationPolicy})
		})
		if err != nil {
			return objs, err
		}

		objsMap, err = ctrl.getPermittedAppLiveObjects(app, proj, projectClusters)
		if err != nil {
			return nil, err
		}

		for k, obj := range objsMap {
			if !ctrl.shouldBeDeleted(app, obj) {
				delete(objsMap, k)
			}
		}
		if len(objsMap) > 0 {
			logCtx.Infof("%d objects remaining for deletion", len(objsMap))
			return objs, nil
		}
	}

	if err := ctrl.cache.SetAppManagedResources(app.Name, nil); err != nil {
		return objs, err
	}

	if err := ctrl.cache.SetAppResourcesTree(app.Name, nil); err != nil {
		return objs, err
	}

	if err := ctrl.removeCascadeFinalizer(app); err != nil {
		return objs, err
	}

	if validDestination {
		logCtx.Infof("Successfully deleted %d resources", len(objs))
	} else {
		logCtx.Infof("Resource entries removed from undefined cluster")
	}

	ctrl.projectRefreshQueue.Add(fmt.Sprintf("%s/%s", ctrl.namespace, app.Spec.GetProject()))
	return objs, nil
}

func (ctrl *ApplicationController) removeCascadeFinalizer(app *appv1.Application) error {
	_, err := ctrl.getAppProj(app)
	if err != nil {
		return fmt.Errorf("error getting project: %w", err)
	}
	app.UnSetCascadedDeletion()
	var patch []byte
	patch, _ = json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": app.Finalizers,
		},
	})

	_, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func (ctrl *ApplicationController) setAppCondition(app *appv1.Application, condition appv1.ApplicationCondition) {
	// do nothing if app already has same condition
	for _, c := range app.Status.Conditions {
		if c.Message == condition.Message && c.Type == condition.Type {
			return
		}
	}

	app.Status.SetConditions([]appv1.ApplicationCondition{condition}, map[appv1.ApplicationConditionType]bool{condition.Type: true})

	var patch []byte
	patch, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": app.Status.Conditions,
		},
	})
	if err == nil {
		_, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	}
	if err != nil {
		log.Errorf("Unable to set application condition: %v", err)
	}
}

func (ctrl *ApplicationController) processRequestedAppOperation(app *appv1.Application) {
	logCtx := log.WithField("application", app.QualifiedName())
	var state *appv1.OperationState
	// Recover from any unexpected panics and automatically set the status to be failed
	defer func() {
		if r := recover(); r != nil {
			logCtx.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
			state.Phase = synccommon.OperationError
			if rerr, ok := r.(error); ok {
				state.Message = rerr.Error()
			} else {
				state.Message = fmt.Sprintf("%v", r)
			}
			ctrl.setOperationState(app, state)
		}
	}()
	terminating := false
	if isOperationInProgress(app) {
		state = app.Status.OperationState.DeepCopy()
		terminating = state.Phase == synccommon.OperationTerminating
		// Failed  operation with retry strategy might have be in-progress and has completion time
		if state.FinishedAt != nil && !terminating {
			retryAt, err := app.Status.OperationState.Operation.Retry.NextRetryAt(state.FinishedAt.Time, state.RetryCount)
			if err != nil {
				state.Phase = synccommon.OperationFailed
				state.Message = err.Error()
				ctrl.setOperationState(app, state)
				return
			}
			retryAfter := time.Until(retryAt)
			if retryAfter > 0 {
				logCtx.Infof("Skipping retrying in-progress operation. Attempting again at: %s", retryAt.Format(time.RFC3339))
				ctrl.requestAppRefresh(app.QualifiedName(), CompareWithLatest.Pointer(), &retryAfter)
				return
			} else {
				// retrying operation. remove previous failure time in app since it is used as a trigger
				// that previous failed and operation should be retried
				state.FinishedAt = nil
				ctrl.setOperationState(app, state)
				// Get rid of sync results and null out previous operation completion time
				state.SyncResult = nil
			}
		} else {
			logCtx.Infof("Resuming in-progress operation. phase: %s, message: %s", state.Phase, state.Message)
		}
	} else {
		state = &appv1.OperationState{Phase: synccommon.OperationRunning, Operation: *app.Operation, StartedAt: metav1.Now()}
		ctrl.setOperationState(app, state)
		logCtx.Infof("Initialized new operation: %v", *app.Operation)
	}

	if err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, ctrl.db); err != nil {
		state.Phase = synccommon.OperationFailed
		state.Message = err.Error()
	} else {
		ctrl.appStateManager.SyncAppState(app, state)
	}

	// Check whether application is allowed to use project
	_, err := ctrl.getAppProj(app)
	if err != nil {
		state.Phase = synccommon.OperationError
		state.Message = err.Error()
	}

	if state.Phase == synccommon.OperationRunning {
		// It's possible for an app to be terminated while we were operating on it. We do not want
		// to clobber the Terminated state with Running. Get the latest app state to check for this.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(context.Background(), app.ObjectMeta.Name, metav1.GetOptions{})
		if err == nil {
			// App may have lost permissions to use the project meanwhile.
			_, err = ctrl.getAppProj(freshApp)
			if err != nil {
				state.Phase = synccommon.OperationFailed
				state.Message = fmt.Sprintf("operation not allowed: %v", err)
			}
			if freshApp.Status.OperationState != nil && freshApp.Status.OperationState.Phase == synccommon.OperationTerminating {
				state.Phase = synccommon.OperationTerminating
				state.Message = "operation is terminating"
				// after this, we will get requeued to the workqueue, but next time the
				// SyncAppState will operate in a Terminating phase, allowing the worker to perform
				// cleanup (e.g. delete jobs, workflows, etc...)
			}
		}
	} else if state.Phase == synccommon.OperationFailed || state.Phase == synccommon.OperationError {
		if !terminating && (state.RetryCount < state.Operation.Retry.Limit || state.Operation.Retry.Limit < 0) {
			now := metav1.Now()
			state.FinishedAt = &now
			if retryAt, err := state.Operation.Retry.NextRetryAt(now.Time, state.RetryCount); err != nil {
				state.Phase = synccommon.OperationFailed
				state.Message = fmt.Sprintf("%s (failed to retry: %v)", state.Message, err)
			} else {
				state.Phase = synccommon.OperationRunning
				state.RetryCount++
				state.Message = fmt.Sprintf("%s. Retrying attempt #%d at %s.", state.Message, state.RetryCount, retryAt.Format(time.Kitchen))
			}
		} else if state.RetryCount > 0 {
			state.Message = fmt.Sprintf("%s (retried %d times).", state.Message, state.RetryCount)
		}

	}

	ctrl.setOperationState(app, state)
	if state.Phase.Completed() && (app.Operation.Sync != nil && !app.Operation.Sync.DryRun) {
		// if we just completed an operation, force a refresh so that UI will report up-to-date
		// sync/health information
		if _, err := cache.MetaNamespaceKeyFunc(app); err == nil {
			// force app refresh with using CompareWithLatest comparison type and trigger app reconciliation loop
			ctrl.requestAppRefresh(app.QualifiedName(), CompareWithLatest.Pointer(), nil)
		} else {
			logCtx.Warnf("Fails to requeue application: %v", err)
		}
	}
}

func (ctrl *ApplicationController) setOperationState(app *appv1.Application, state *appv1.OperationState) {
	kube.RetryUntilSucceed(context.Background(), updateOperationStateTimeout, "Update application operation state", logutils.NewLogrusLogger(logutils.NewWithCurrentConfig()), func() error {
		if state.Phase == "" {
			// expose any bugs where we neglect to set phase
			panic("no phase was set")
		}
		if state.Phase.Completed() {
			now := metav1.Now()
			state.FinishedAt = &now
		}
		patch := map[string]interface{}{
			"status": map[string]interface{}{
				"operationState": state,
			},
		}
		if state.Phase.Completed() {
			// If operation is completed, clear the operation field to indicate no operation is
			// in progress.
			patch["operation"] = nil
		}
		if reflect.DeepEqual(app.Status.OperationState, state) {
			log.Infof("No operation updates necessary to '%s'. Skipping patch", app.QualifiedName())
			return nil
		}
		patchJSON, err := json.Marshal(patch)
		if err != nil {
			return fmt.Errorf("error marshaling json: %w", err)
		}
		if app.Status.OperationState != nil && app.Status.OperationState.FinishedAt != nil && state.FinishedAt == nil {
			patchJSON, err = jsonpatch.MergeMergePatches(patchJSON, []byte(`{"status": {"operationState": {"finishedAt": null}}}`))
			if err != nil {
				return fmt.Errorf("error merging operation state patch: %w", err)
			}
		}

		appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
		_, err = appClient.Patch(context.Background(), app.Name, types.MergePatchType, patchJSON, metav1.PatchOptions{})
		if err != nil {
			// Stop retrying updating deleted application
			if apierr.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("error patching application with operation state: %w", err)
		}
		log.Infof("updated '%s' operation (phase: %s)", app.QualifiedName(), state.Phase)
		if state.Phase.Completed() {
			eventInfo := argo.EventInfo{Reason: argo.EventReasonOperationCompleted}
			var messages []string
			if state.Operation.Sync != nil && len(state.Operation.Sync.Resources) > 0 {
				messages = []string{"Partial sync operation"}
			} else {
				messages = []string{"Sync operation"}
			}
			if state.SyncResult != nil {
				messages = append(messages, "to", state.SyncResult.Revision)
			}
			if state.Phase.Successful() {
				eventInfo.Type = v1.EventTypeNormal
				messages = append(messages, "succeeded")
			} else {
				eventInfo.Type = v1.EventTypeWarning
				messages = append(messages, "failed:", state.Message)
			}
			ctrl.auditLogger.LogAppEvent(app, eventInfo, strings.Join(messages, " "), "")
			ctrl.metricsServer.IncSync(app, state)
		}
		return nil
	})
}

func (ctrl *ApplicationController) processAppRefreshQueueItem() (processNext bool) {
	appKey, shutdown := ctrl.appRefreshQueue.Get()
	if shutdown {
		processNext = false
		return
	}
	processNext = true
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
		}
		ctrl.appRefreshQueue.Done(appKey)
	}()
	obj, exists, err := ctrl.appInformer.GetIndexer().GetByKey(appKey.(string))
	if err != nil {
		log.Errorf("Failed to get application '%s' from informer index: %+v", appKey, err)
		return
	}
	if !exists {
		// This happens after app was deleted, but the work queue still had an entry for it.
		return
	}
	origApp, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}
	origApp = origApp.DeepCopy()
	needRefresh, refreshType, comparisonLevel := ctrl.needRefreshAppStatus(origApp, ctrl.statusRefreshTimeout, ctrl.statusHardRefreshTimeout)

	if !needRefresh {
		return
	}
	app := origApp.DeepCopy()
	logCtx := log.WithFields(log.Fields{"application": app.QualifiedName()})

	startTime := time.Now()
	defer func() {
		reconcileDuration := time.Since(startTime)
		ctrl.metricsServer.IncReconcile(origApp, reconcileDuration)
		logCtx.WithFields(log.Fields{
			"time_ms":        reconcileDuration.Milliseconds(),
			"level":          comparisonLevel,
			"dest-server":    origApp.Spec.Destination.Server,
			"dest-name":      origApp.Spec.Destination.Name,
			"dest-namespace": origApp.Spec.Destination.Namespace,
		}).Info("Reconciliation completed")
	}()

	if comparisonLevel == ComparisonWithNothing {
		managedResources := make([]*appv1.ResourceDiff, 0)
		if err := ctrl.cache.GetAppManagedResources(app.InstanceName(ctrl.namespace), &managedResources); err != nil {
			logCtx.Warnf("Failed to get cached managed resources for tree reconciliation, fall back to full reconciliation")
		} else {
			var tree *appv1.ApplicationTree
			if tree, err = ctrl.getResourceTree(app, managedResources); err == nil {
				app.Status.Summary = tree.GetSummary(app)
				if err := ctrl.cache.SetAppResourcesTree(app.InstanceName(ctrl.namespace), tree); err != nil {
					logCtx.Errorf("Failed to cache resources tree: %v", err)
					return
				}
			}

			ctrl.persistAppStatus(origApp, &app.Status)
			return
		}
	}

	project, hasErrors := ctrl.refreshAppConditions(app)
	if hasErrors {
		app.Status.Sync.Status = appv1.SyncStatusCodeUnknown
		app.Status.Health.Status = health.HealthStatusUnknown
		ctrl.persistAppStatus(origApp, &app.Status)

		if err := ctrl.cache.SetAppResourcesTree(app.InstanceName(ctrl.namespace), &appv1.ApplicationTree{}); err != nil {
			log.Warnf("failed to set app resource tree: %v", err)
		}
		if err := ctrl.cache.SetAppManagedResources(app.InstanceName(ctrl.namespace), nil); err != nil {
			log.Warnf("failed to set app managed resources tree: %v", err)
		}
		return
	}

	var localManifests []string
	if opState := app.Status.OperationState; opState != nil && opState.Operation.Sync != nil {
		localManifests = opState.Operation.Sync.Manifests
	}

	revisions := make([]string, 0)
	sources := make([]appv1.ApplicationSource, 0)

	hasMultipleSources := app.Spec.HasMultipleSources()

	// If we have multiple sources, we use all the sources under `sources` field and ignore source under `source` field.
	// else we use the source under the source field.
	if hasMultipleSources {
		for _, source := range app.Spec.Sources {
			// We do not perform any filtering of duplicate sources.
			// Argo CD will apply and update the resources generated from the sources automatically
			// based on the order in which manifests were generated
			sources = append(sources, source)
			revisions = append(revisions, source.TargetRevision)
		}
		if comparisonLevel == CompareWithRecent {
			revisions = app.Status.Sync.Revisions
		}
	} else {
		revision := app.Spec.GetSource().TargetRevision
		if comparisonLevel == CompareWithRecent {
			revision = app.Status.Sync.Revision
		}
		revisions = append(revisions, revision)
		sources = append(sources, app.Spec.GetSource())
	}
	now := metav1.Now()

	compareResult := ctrl.appStateManager.CompareAppState(app, project, revisions, sources,
		refreshType == appv1.RefreshTypeHard,
		comparisonLevel == CompareWithLatestForceResolve, localManifests, hasMultipleSources)

	for k, v := range compareResult.timings {
		logCtx = logCtx.WithField(k, v.Milliseconds())
	}

	ctrl.normalizeApplication(origApp, app)

	tree, err := ctrl.setAppManagedResources(app, compareResult)
	if err != nil {
		logCtx.Errorf("Failed to cache app resources: %v", err)
	} else {
		app.Status.Summary = tree.GetSummary(app)
	}

	if project.Spec.SyncWindows.Matches(app).CanSync(false) {
		syncErrCond := ctrl.autoSync(app, compareResult.syncStatus, compareResult.resources)
		if syncErrCond != nil {
			app.Status.SetConditions(
				[]appv1.ApplicationCondition{*syncErrCond},
				map[appv1.ApplicationConditionType]bool{appv1.ApplicationConditionSyncError: true},
			)
		} else {
			app.Status.SetConditions(
				[]appv1.ApplicationCondition{},
				map[appv1.ApplicationConditionType]bool{appv1.ApplicationConditionSyncError: true},
			)
		}
	} else {
		logCtx.Info("Sync prevented by sync window")
	}

	if app.Status.ReconciledAt == nil || comparisonLevel >= CompareWithLatest {
		app.Status.ReconciledAt = &now
	}
	app.Status.Sync = *compareResult.syncStatus
	app.Status.Health = *compareResult.healthStatus
	app.Status.Resources = compareResult.resources
	sort.Slice(app.Status.Resources, func(i, j int) bool {
		return resourceStatusKey(app.Status.Resources[i]) < resourceStatusKey(app.Status.Resources[j])
	})
	app.Status.SourceType = compareResult.appSourceType
	app.Status.SourceTypes = compareResult.appSourceTypes
	ctrl.persistAppStatus(origApp, &app.Status)
	return
}

func resourceStatusKey(res appv1.ResourceStatus) string {
	return strings.Join([]string{res.Group, res.Kind, res.Namespace, res.Name}, "/")
}

func currentSourceEqualsSyncedSource(app *appv1.Application) bool {
	if app.Spec.HasMultipleSources() {
		return app.Spec.Sources.Equals(app.Status.Sync.ComparedTo.Sources)
	}
	return app.Spec.Source.Equals(&app.Status.Sync.ComparedTo.Source)
}

// needRefreshAppStatus answers if application status needs to be refreshed.
// Returns true if application never been compared, has changed or comparison result has expired.
// Additionally returns whether full refresh was requested or not.
// If full refresh is requested then target and live state should be reconciled, else only live state tree should be updated.
func (ctrl *ApplicationController) needRefreshAppStatus(app *appv1.Application, statusRefreshTimeout, statusHardRefreshTimeout time.Duration) (bool, appv1.RefreshType, CompareWith) {
	logCtx := log.WithFields(log.Fields{"application": app.QualifiedName()})
	var reason string
	compareWith := CompareWithLatest
	refreshType := appv1.RefreshTypeNormal
	softExpired := app.Status.ReconciledAt == nil || app.Status.ReconciledAt.Add(statusRefreshTimeout).Before(time.Now().UTC())
	hardExpired := (app.Status.ReconciledAt == nil || app.Status.ReconciledAt.Add(statusHardRefreshTimeout).Before(time.Now().UTC())) && statusHardRefreshTimeout.Seconds() != 0

	if requestedType, ok := app.IsRefreshRequested(); ok {
		compareWith = CompareWithLatestForceResolve
		// user requested app refresh.
		refreshType = requestedType
		reason = fmt.Sprintf("%s refresh requested", refreshType)
	} else {
		if !currentSourceEqualsSyncedSource(app) {
			reason = "spec.source differs"
			compareWith = CompareWithLatestForceResolve
			if app.Spec.HasMultipleSources() {
				reason = "at least one of the spec.sources differs"
			}
		} else if hardExpired || softExpired {
			// The commented line below mysteriously crashes if app.Status.ReconciledAt is nil
			// reason = fmt.Sprintf("comparison expired. reconciledAt: %v, expiry: %v", app.Status.ReconciledAt, statusRefreshTimeout)
			//TODO: find existing Golang bug or create a new one
			reconciledAtStr := "never"
			if app.Status.ReconciledAt != nil {
				reconciledAtStr = app.Status.ReconciledAt.String()
			}
			reason = fmt.Sprintf("comparison expired, requesting refresh. reconciledAt: %v, expiry: %v", reconciledAtStr, statusRefreshTimeout)
			if hardExpired {
				reason = fmt.Sprintf("comparison expired, requesting hard refresh. reconciledAt: %v, expiry: %v", reconciledAtStr, statusHardRefreshTimeout)
				refreshType = appv1.RefreshTypeHard
			}
		} else if !app.Spec.Destination.Equals(app.Status.Sync.ComparedTo.Destination) {
			reason = "spec.destination differs"
		} else if requested, level := ctrl.isRefreshRequested(app.QualifiedName()); requested {
			compareWith = level
			reason = "controller refresh requested"
		}
	}

	if reason != "" {
		logCtx.Infof("Refreshing app status (%s), level (%d)", reason, compareWith)
		return true, refreshType, compareWith
	}
	return false, refreshType, compareWith
}

func (ctrl *ApplicationController) refreshAppConditions(app *appv1.Application) (*appv1.AppProject, bool) {
	errorConditions := make([]appv1.ApplicationCondition, 0)
	proj, err := ctrl.getAppProj(app)
	if err != nil {
		errorConditions = append(errorConditions, ctrl.projectErrorToCondition(err, app))
	} else {
		specConditions, err := argo.ValidatePermissions(context.Background(), &app.Spec, proj, ctrl.db)
		if err != nil {
			errorConditions = append(errorConditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionUnknownError,
				Message: err.Error(),
			})
		} else {
			errorConditions = append(errorConditions, specConditions...)
		}
	}
	app.Status.SetConditions(errorConditions, map[appv1.ApplicationConditionType]bool{
		appv1.ApplicationConditionInvalidSpecError: true,
		appv1.ApplicationConditionUnknownError:     true,
	})
	return proj, len(errorConditions) > 0
}

// normalizeApplication normalizes an application.spec and additionally persists updates if it changed
func (ctrl *ApplicationController) normalizeApplication(orig, app *appv1.Application) {
	logCtx := log.WithFields(log.Fields{"application": app.QualifiedName()})
	app.Spec = *argo.NormalizeApplicationSpec(&app.Spec)

	patch, modified, err := diff.CreateTwoWayMergePatch(orig, app, appv1.Application{})

	if err != nil {
		logCtx.Errorf("error constructing app spec patch: %v", err)
	} else if modified {
		appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
		_, err = appClient.Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
		if err != nil {
			logCtx.Errorf("Error persisting normalized application spec: %v", err)
		} else {
			logCtx.Infof("Normalized app spec: %s", string(patch))
		}
	}
}

// persistAppStatus persists updates to application status. If no changes were made, it is a no-op
func (ctrl *ApplicationController) persistAppStatus(orig *appv1.Application, newStatus *appv1.ApplicationStatus) {
	logCtx := log.WithFields(log.Fields{"application": orig.QualifiedName()})
	if orig.Status.Sync.Status != newStatus.Sync.Status {
		message := fmt.Sprintf("Updated sync status: %s -> %s", orig.Status.Sync.Status, newStatus.Sync.Status)
		ctrl.auditLogger.LogAppEvent(orig, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Type: v1.EventTypeNormal}, message, "")
	}
	if orig.Status.Health.Status != newStatus.Health.Status {
		message := fmt.Sprintf("Updated health status: %s -> %s", orig.Status.Health.Status, newStatus.Health.Status)
		ctrl.auditLogger.LogAppEvent(orig, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Type: v1.EventTypeNormal}, message, "")
	}
	var newAnnotations map[string]string
	if orig.GetAnnotations() != nil {
		newAnnotations = make(map[string]string)
		for k, v := range orig.GetAnnotations() {
			newAnnotations[k] = v
		}
		delete(newAnnotations, appv1.AnnotationKeyRefresh)
	}
	patch, modified, err := diff.CreateTwoWayMergePatch(
		&appv1.Application{ObjectMeta: metav1.ObjectMeta{Annotations: orig.GetAnnotations()}, Status: orig.Status},
		&appv1.Application{ObjectMeta: metav1.ObjectMeta{Annotations: newAnnotations}, Status: *newStatus}, appv1.Application{})
	if err != nil {
		logCtx.Errorf("Error constructing app status patch: %v", err)
		return
	}
	if !modified {
		logCtx.Infof("No status changes. Skipping patch")
		return
	}
	appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(orig.Namespace)
	_, err = appClient.Patch(context.Background(), orig.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		logCtx.Warnf("Error updating application: %v", err)
	} else {
		logCtx.Infof("Update successful")
	}
}

// autoSync will initiate a sync operation for an application configured with automated sync
func (ctrl *ApplicationController) autoSync(app *appv1.Application, syncStatus *appv1.SyncStatus, resources []appv1.ResourceStatus) *appv1.ApplicationCondition {
	if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
		return nil
	}
	logCtx := log.WithFields(log.Fields{"application": app.QualifiedName()})

	if app.Operation != nil {
		logCtx.Infof("Skipping auto-sync: another operation is in progress")
		return nil
	}
	if app.DeletionTimestamp != nil && !app.DeletionTimestamp.IsZero() {
		logCtx.Infof("Skipping auto-sync: deletion in progress")
		return nil
	}

	// Only perform auto-sync if we detect OutOfSync status. This is to prevent us from attempting
	// a sync when application is already in a Synced or Unknown state
	if syncStatus.Status != appv1.SyncStatusCodeOutOfSync {
		logCtx.Infof("Skipping auto-sync: application status is %s", syncStatus.Status)
		return nil
	}

	if !app.Spec.SyncPolicy.Automated.Prune {
		requirePruneOnly := true
		for _, r := range resources {
			if r.Status != appv1.SyncStatusCodeSynced && !r.RequiresPruning {
				requirePruneOnly = false
				break
			}
		}
		if requirePruneOnly {
			logCtx.Infof("Skipping auto-sync: need to prune extra resources only but automated prune is disabled")
			return nil
		}
	}

	desiredCommitSHA := syncStatus.Revision
	desiredCommitSHAsMS := syncStatus.Revisions
	alreadyAttempted, attemptPhase := alreadyAttemptedSync(app, desiredCommitSHA, desiredCommitSHAsMS, app.Spec.HasMultipleSources())
	selfHeal := app.Spec.SyncPolicy.Automated.SelfHeal
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:    desiredCommitSHA,
			Prune:       app.Spec.SyncPolicy.Automated.Prune,
			SyncOptions: app.Spec.SyncPolicy.SyncOptions,
			Revisions:   desiredCommitSHAsMS,
		},
		InitiatedBy: appv1.OperationInitiator{Automated: true},
		Retry:       appv1.RetryStrategy{Limit: 5},
	}
	if app.Spec.SyncPolicy.Retry != nil {
		op.Retry = *app.Spec.SyncPolicy.Retry
	}
	// It is possible for manifests to remain OutOfSync even after a sync/kubectl apply (e.g.
	// auto-sync with pruning disabled). We need to ensure that we do not keep Syncing an
	// application in an infinite loop. To detect this, we only attempt the Sync if the revision
	// and parameter overrides are different from our most recent sync operation.
	if alreadyAttempted && (!selfHeal || !attemptPhase.Successful()) {
		if !attemptPhase.Successful() {
			logCtx.Warnf("Skipping auto-sync: failed previous sync attempt to %s", desiredCommitSHA)
			message := fmt.Sprintf("Failed sync attempt to %s: %s", desiredCommitSHA, app.Status.OperationState.Message)
			return &appv1.ApplicationCondition{Type: appv1.ApplicationConditionSyncError, Message: message}
		}
		logCtx.Infof("Skipping auto-sync: most recent sync already to %s", desiredCommitSHA)
		return nil
	} else if alreadyAttempted && selfHeal {
		if shouldSelfHeal, retryAfter := ctrl.shouldSelfHeal(app); shouldSelfHeal {
			for _, resource := range resources {
				if resource.Status != appv1.SyncStatusCodeSynced {
					op.Sync.Resources = append(op.Sync.Resources, appv1.SyncOperationResource{
						Kind:  resource.Kind,
						Group: resource.Group,
						Name:  resource.Name,
					})
				}
			}
		} else {
			logCtx.Infof("Skipping auto-sync: already attempted sync to %s with timeout %v (retrying in %v)", desiredCommitSHA, ctrl.selfHealTimeout, retryAfter)
			ctrl.requestAppRefresh(app.QualifiedName(), CompareWithLatest.Pointer(), &retryAfter)
			return nil
		}

	}

	if app.Spec.SyncPolicy.Automated.Prune && !app.Spec.SyncPolicy.Automated.AllowEmpty {
		bAllNeedPrune := true
		for _, r := range resources {
			if !r.RequiresPruning {
				bAllNeedPrune = false
			}
		}
		if bAllNeedPrune {
			message := fmt.Sprintf("Skipping sync attempt to %s: auto-sync will wipe out all resources", desiredCommitSHA)
			logCtx.Warnf(message)
			return &appv1.ApplicationCondition{Type: appv1.ApplicationConditionSyncError, Message: message}
		}
	}
	appIf := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
	_, err := argo.SetAppOperation(appIf, app.Name, &op)
	if err != nil {
		logCtx.Errorf("Failed to initiate auto-sync to %s: %v", desiredCommitSHA, err)
		return &appv1.ApplicationCondition{Type: appv1.ApplicationConditionSyncError, Message: err.Error()}
	}
	message := fmt.Sprintf("Initiated automated sync to '%s'", desiredCommitSHA)
	ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonOperationStarted, Type: v1.EventTypeNormal}, message, "")
	logCtx.Info(message)
	return nil
}

// alreadyAttemptedSync returns whether or not the most recent sync was performed against the
// commitSHA and with the same app source config which are currently set in the app
func alreadyAttemptedSync(app *appv1.Application, commitSHA string, commitSHAsMS []string, hasMultipleSources bool) (bool, synccommon.OperationPhase) {
	if app.Status.OperationState == nil || app.Status.OperationState.Operation.Sync == nil || app.Status.OperationState.SyncResult == nil {
		return false, ""
	}
	if hasMultipleSources {
		if !reflect.DeepEqual(app.Status.OperationState.SyncResult.Revisions, commitSHAsMS) {
			return false, ""
		}
	} else {
		if app.Status.OperationState.SyncResult.Revision != commitSHA {
			return false, ""
		}
	}

	if hasMultipleSources {
		// Ignore differences in target revision, since we already just verified commitSHAs are equal,
		// and we do not want to trigger auto-sync due to things like HEAD != master
		specSources := app.Spec.Sources.DeepCopy()
		syncSources := app.Status.OperationState.SyncResult.Sources.DeepCopy()
		for _, source := range specSources {
			source.TargetRevision = ""
		}
		for _, source := range syncSources {
			source.TargetRevision = ""
		}
		return reflect.DeepEqual(app.Spec.Sources, app.Status.OperationState.SyncResult.Sources), app.Status.OperationState.Phase
	} else {
		// Ignore differences in target revision, since we already just verified commitSHAs are equal,
		// and we do not want to trigger auto-sync due to things like HEAD != master
		specSource := app.Spec.Source.DeepCopy()
		specSource.TargetRevision = ""
		syncResSource := app.Status.OperationState.SyncResult.Source.DeepCopy()
		syncResSource.TargetRevision = ""
		return reflect.DeepEqual(app.Spec.GetSource(), app.Status.OperationState.SyncResult.Source), app.Status.OperationState.Phase
	}
}

func (ctrl *ApplicationController) shouldSelfHeal(app *appv1.Application) (bool, time.Duration) {
	if app.Status.OperationState == nil {
		return true, time.Duration(0)
	}

	var retryAfter time.Duration
	if app.Status.OperationState.FinishedAt == nil {
		retryAfter = ctrl.selfHealTimeout
	} else {
		retryAfter = ctrl.selfHealTimeout - time.Since(app.Status.OperationState.FinishedAt.Time)
	}
	return retryAfter <= 0, retryAfter
}

func (ctrl *ApplicationController) canProcessApp(obj interface{}) bool {
	app, ok := obj.(*appv1.Application)
	if !ok {
		return false
	}

	// Only process given app if it exists in a watched namespace, or in the
	// control plane's namespace.
	if app.Namespace != ctrl.namespace && !glob.MatchStringInList(ctrl.applicationNamespaces, app.Namespace, false) {
		return false
	}

	if annotations := app.GetAnnotations(); annotations != nil {
		if skipVal, ok := annotations[common.AnnotationKeyAppSkipReconcile]; ok {
			logCtx := log.WithFields(log.Fields{"application": app.QualifiedName()})
			if skipReconcile, err := strconv.ParseBool(skipVal); err == nil {
				if skipReconcile {
					logCtx.Debugf("Skipping Application reconcile based on annotation %s", common.AnnotationKeyAppSkipReconcile)
					return false
				}
			} else {
				logCtx.Debugf("Unable to determine if Application should skip reconcile based on annotation %s: %v", common.AnnotationKeyAppSkipReconcile, err)
			}
		}
	}

	if ctrl.clusterFilter != nil {
		cluster, err := ctrl.db.GetCluster(context.Background(), app.Spec.Destination.Server)
		if err != nil {
			return ctrl.clusterFilter(nil)
		}
		return ctrl.clusterFilter(cluster)
	}

	return true
}

func (ctrl *ApplicationController) newApplicationInformerAndLister() (cache.SharedIndexInformer, applisters.ApplicationLister) {
	watchNamespace := ctrl.namespace
	// If we have at least one additional namespace configured, we need to
	// watch on them all.
	if len(ctrl.applicationNamespaces) > 0 {
		watchNamespace = ""
	}
	refreshTimeout := ctrl.statusRefreshTimeout
	if ctrl.statusHardRefreshTimeout.Seconds() != 0 && (ctrl.statusHardRefreshTimeout < ctrl.statusRefreshTimeout) {
		refreshTimeout = ctrl.statusHardRefreshTimeout
	}
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (apiruntime.Object, error) {
				// We are only interested in apps that exist in namespaces the
				// user wants to be enabled.
				appList, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(watchNamespace).List(context.TODO(), options)
				if err != nil {
					return nil, err
				}
				newItems := []appv1.Application{}
				for _, app := range appList.Items {
					if ctrl.namespace == app.Namespace || glob.MatchStringInList(ctrl.applicationNamespaces, app.Namespace, false) {
						newItems = append(newItems, app)
					}
				}
				appList.Items = newItems
				return appList, nil
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return ctrl.applicationClientset.ArgoprojV1alpha1().Applications(watchNamespace).Watch(context.TODO(), options)
			},
		},
		&appv1.Application{},
		refreshTimeout,
		cache.Indexers{
			cache.NamespaceIndex: func(obj interface{}) ([]string, error) {
				app, ok := obj.(*appv1.Application)
				if ok {
					// This call to 'ValidateDestination' ensures that the .spec.destination field of all Applications
					// returned by the informer/lister will have server field set (if not already set) based on the name.
					// (or, if not found, an error app condition)

					// If the server field is not set, set it based on the cluster name; if the cluster name can't be found,
					// log an error as an App Condition.
					if err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, ctrl.db); err != nil {
						ctrl.setAppCondition(app, appv1.ApplicationCondition{Type: appv1.ApplicationConditionInvalidSpecError, Message: err.Error()})
					}

					// If the application is not allowed to use the project,
					// log an error.
					if _, err := ctrl.getAppProj(app); err != nil {
						ctrl.setAppCondition(app, ctrl.projectErrorToCondition(err, app))
					}
				}

				return cache.MetaNamespaceIndexFunc(obj)
			},
			orphanedIndex: func(obj interface{}) (i []string, e error) {
				app, ok := obj.(*appv1.Application)
				if !ok {
					return nil, nil
				}

				proj, err := applisters.NewAppProjectLister(ctrl.projInformer.GetIndexer()).AppProjects(ctrl.namespace).Get(app.Spec.GetProject())
				if err != nil {
					return nil, nil
				}
				if proj.Spec.OrphanedResources != nil {
					return []string{app.Spec.Destination.Namespace}, nil
				}
				return nil, nil
			},
		},
	)
	lister := applisters.NewApplicationLister(informer.GetIndexer())
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				if !ctrl.canProcessApp(obj) {
					return
				}
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					ctrl.appRefreshQueue.Add(key)
					ctrl.appOperationQueue.Add(key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				if !ctrl.canProcessApp(new) {
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(new)
				if err != nil {
					return
				}
				var compareWith *CompareWith
				oldApp, oldOK := old.(*appv1.Application)
				newApp, newOK := new.(*appv1.Application)
				if oldOK && newOK && automatedSyncEnabled(oldApp, newApp) {
					log.WithField("application", newApp.QualifiedName()).Info("Enabled automated sync")
					compareWith = CompareWithLatest.Pointer()
				}
				ctrl.requestAppRefresh(newApp.QualifiedName(), compareWith, nil)
				ctrl.appOperationQueue.Add(key)
			},
			DeleteFunc: func(obj interface{}) {
				if !ctrl.canProcessApp(obj) {
					return
				}
				// IndexerInformer uses a delta queue, therefore for deletes we have to use this
				// key function.
				key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				if err == nil {
					ctrl.appRefreshQueue.Add(key)
				}
			},
		},
	)
	return informer, lister
}

func (ctrl *ApplicationController) projectErrorToCondition(err error, app *appv1.Application) appv1.ApplicationCondition {
	var condition appv1.ApplicationCondition
	if apierr.IsNotFound(err) {
		condition = appv1.ApplicationCondition{
			Type:    appv1.ApplicationConditionInvalidSpecError,
			Message: fmt.Sprintf("Application referencing project %s which does not exist", app.Spec.Project),
		}
	} else {
		condition = appv1.ApplicationCondition{Type: appv1.ApplicationConditionUnknownError, Message: err.Error()}
	}
	return condition
}

func (ctrl *ApplicationController) RegisterClusterSecretUpdater(ctx context.Context) {
	updater := NewClusterInfoUpdater(ctrl.stateCache, ctrl.db, ctrl.appLister.Applications(""), ctrl.cache, ctrl.clusterFilter, ctrl.getAppProj, ctrl.namespace)
	go updater.Run(ctx)
}

func isOperationInProgress(app *appv1.Application) bool {
	return app.Status.OperationState != nil && !app.Status.OperationState.Phase.Completed()
}

// automatedSyncEnabled tests if an app went from auto-sync disabled to enabled.
// if it was toggled to be enabled, the informer handler will force a refresh
func automatedSyncEnabled(oldApp *appv1.Application, newApp *appv1.Application) bool {
	oldEnabled := false
	oldSelfHealEnabled := false
	if oldApp.Spec.SyncPolicy != nil && oldApp.Spec.SyncPolicy.Automated != nil {
		oldEnabled = true
		oldSelfHealEnabled = oldApp.Spec.SyncPolicy.Automated.SelfHeal
	}

	newEnabled := false
	newSelfHealEnabled := false
	if newApp.Spec.SyncPolicy != nil && newApp.Spec.SyncPolicy.Automated != nil {
		newEnabled = true
		newSelfHealEnabled = newApp.Spec.SyncPolicy.Automated.SelfHeal
	}
	if !oldEnabled && newEnabled {
		return true
	}
	if !oldSelfHealEnabled && newSelfHealEnabled {
		return true
	}
	// nothing changed
	return false
}

// toAppKey returns the application key from a given appName, that is, it will
// replace underscores with forward-slashes to become a <namespace>/<name>
// format. If the appName is an unqualified name (such as, "app"), it will use
// the controller's namespace in the key.
func (ctrl *ApplicationController) toAppKey(appName string) string {
	if !strings.Contains(appName, "_") && !strings.Contains(appName, "/") {
		return ctrl.namespace + "/" + appName
	} else if strings.Contains(appName, "/") {
		return appName
	} else {
		return strings.ReplaceAll(appName, "_", "/")
	}
}

func (ctrl *ApplicationController) toAppQualifiedName(appName, appNamespace string) string {
	return fmt.Sprintf("%s/%s", appNamespace, appName)
}
