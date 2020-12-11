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

	"github.com/argoproj/gitops-engine/pkg/diff"
	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	// make sure to register workqueue prometheus metrics
	_ "k8s.io/component-base/metrics/prometheus/workqueue"

	"github.com/argoproj/argo-cd/common"
	statecache "github.com/argoproj/argo-cd/controller/cache"
	"github.com/argoproj/argo-cd/controller/metrics"
	"github.com/argoproj/argo-cd/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	"github.com/argoproj/argo-cd/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/util/argo"
	appstatecache "github.com/argoproj/argo-cd/util/cache/appstate"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/errors"
	"github.com/argoproj/argo-cd/util/glob"
	logutils "github.com/argoproj/argo-cd/util/log"
	settings_util "github.com/argoproj/argo-cd/util/settings"
)

const (
	updateOperationStateTimeout = 1 * time.Second
	// orphanedIndex contains application which monitor orphaned resources by namespace
	orphanedIndex = "orphaned"
)

type CompareWith int

const (
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
	selfHealTimeout               time.Duration
	repoClientset                 apiclient.Clientset
	db                            db.ArgoDB
	settingsMgr                   *settings_util.SettingsManager
	refreshRequestedApps          map[string]CompareWith
	refreshRequestedAppsMutex     *sync.Mutex
	metricsServer                 *metrics.MetricsServer
	kubectlSemaphore              *semaphore.Weighted
	clusterFilter                 func(cluster *appv1.Cluster) bool
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
	selfHealTimeout time.Duration,
	metricsPort int,
	kubectlParallelismLimit int64,
	clusterFilter func(cluster *appv1.Cluster) bool,
) (*ApplicationController, error) {
	log.Infof("appResyncPeriod=%v", appResyncPeriod)
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
		refreshRequestedApps:          make(map[string]CompareWith),
		refreshRequestedAppsMutex:     &sync.Mutex{},
		auditLogger:                   argo.NewAuditLogger(namespace, kubeClientset, "argocd-application-controller"),
		settingsMgr:                   settingsMgr,
		selfHealTimeout:               selfHealTimeout,
		clusterFilter:                 clusterFilter,
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
			}
		},
		UpdateFunc: func(old, new interface{}) {
			if key, err := cache.MetaNamespaceKeyFunc(new); err == nil {
				ctrl.projectRefreshQueue.Add(key)
			}
		},
		DeleteFunc: func(obj interface{}) {
			if key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj); err == nil {
				ctrl.projectRefreshQueue.Add(key)
			}
		},
	})
	metricsAddr := fmt.Sprintf("0.0.0.0:%d", metricsPort)
	var err error
	ctrl.metricsServer, err = metrics.NewMetricsServer(metricsAddr, appLister, ctrl.canProcessApp, func(r *http.Request) error {
		return nil
	})
	if err != nil {
		return nil, err
	}
	stateCache := statecache.NewLiveStateCache(db, appInformer, ctrl.settingsMgr, kubectl, ctrl.metricsServer, ctrl.handleObjectUpdated, clusterFilter)
	appStateManager := NewAppStateManager(db, applicationClientset, repoClientset, namespace, kubectl, ctrl.settingsMgr, stateCache, projInformer, ctrl.metricsServer)
	ctrl.appInformer = appInformer
	ctrl.appLister = appLister
	ctrl.projInformer = projInformer
	ctrl.appStateManager = appStateManager
	ctrl.stateCache = stateCache

	return &ctrl, nil
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

func (ctrl *ApplicationController) getAppProj(app *appv1.Application) (*appv1.AppProject, error) {
	return argo.GetAppProject(&app.Spec, applisters.NewAppProjectLister(ctrl.projInformer.GetIndexer()), ctrl.namespace, ctrl.settingsMgr)
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
				// exclude resource unless it is permitted in the app project. If project is not permitted then it is not controlled by the user and there is no point showing the warning.
				if proj, err := ctrl.getAppProj(app); err == nil && proj.IsGroupKindPermitted(ref.GroupVersionKind().GroupKind(), true) &&
					!isKnownOrphanedResourceExclusion(kube.NewResourceKey(ref.GroupVersionKind().Group, ref.GroupVersionKind().Kind, ref.Namespace, ref.Name), proj) {

					managedByApp[app.Name] = false
				}
			}
		}
	}
	for appName, isManagedResource := range managedByApp {
		obj, exists, err := ctrl.appInformer.GetIndexer().GetByKey(ctrl.namespace + "/" + appName)
		if app, ok := obj.(*appv1.Application); exists && err == nil && ok && isSelfReferencedApp(app, ref) {
			// Don't force refresh app if related resource is application itself. This prevents infinite reconciliation loop.
			continue
		}

		if !ctrl.canProcessApp(obj) {
			// Don't force refresh app if app belongs to a different controller shard
			continue
		}

		level := ComparisonWithNothing
		if isManagedResource {
			level = CompareWithRecent
		}
		ctrl.requestAppRefresh(appName, &level, nil)
	}
}

func (ctrl *ApplicationController) setAppManagedResources(a *appv1.Application, comparisonResult *comparisonResult) (*appv1.ApplicationTree, error) {
	managedResources, err := ctrl.managedResources(comparisonResult)
	if err != nil {
		return nil, err
	}
	tree, err := ctrl.getResourceTree(a, managedResources)
	if err != nil {
		return nil, err
	}
	err = ctrl.cache.SetAppResourcesTree(a.Name, tree)
	if err != nil {
		return nil, err
	}
	return tree, ctrl.cache.SetAppManagedResources(a.Name, managedResources)
}

// returns true of given resources exist in the namespace by default and not managed by the user
func isKnownOrphanedResourceExclusion(key kube.ResourceKey, proj *appv1.AppProject) bool {
	if key.Namespace == "default" && key.Group == "" && key.Kind == kube.ServiceKind && key.Name == "kubernetes" {
		return true
	}
	if key.Group == "" && key.Kind == kube.ServiceAccountKind && key.Name == "default" {
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

	proj, err := argo.GetAppProject(&a.Spec, applisters.NewAppProjectLister(ctrl.projInformer.GetIndexer()), ctrl.namespace, ctrl.settingsMgr)
	if err != nil {
		return nil, err
	}
	orphanedNodesMap := make(map[kube.ResourceKey]appv1.ResourceNode)
	warnOrphaned := true
	if proj.Spec.OrphanedResources != nil {
		orphanedNodesMap, err = ctrl.stateCache.GetNamespaceTopLevelResources(a.Spec.Destination.Server, a.Spec.Destination.Namespace)
		if err != nil {
			return nil, err
		}
		warnOrphaned = proj.Spec.OrphanedResources.IsWarn()
	}

	for i := range managedResources {
		managedResource := managedResources[i]
		delete(orphanedNodesMap, kube.NewResourceKey(managedResource.Group, managedResource.Kind, managedResource.Namespace, managedResource.Name))
		var live = &unstructured.Unstructured{}
		err := json.Unmarshal([]byte(managedResource.LiveState), &live)
		if err != nil {
			return nil, err
		}
		var target = &unstructured.Unstructured{}
		err = json.Unmarshal([]byte(managedResource.TargetState), &target)
		if err != nil {
			return nil, err
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
			err := ctrl.stateCache.IterateHierarchy(a.Spec.Destination.Server, kube.GetResourceKey(live), func(child appv1.ResourceNode, appName string) {
				nodes = append(nodes, child)
			})
			if err != nil {
				return nil, err
			}
		}
	}
	orphanedNodes := make([]appv1.ResourceNode, 0)
	for k := range orphanedNodesMap {
		if k.Namespace != "" && proj.IsGroupKindPermitted(k.GroupKind(), true) && !isKnownOrphanedResourceExclusion(k, proj) {
			err := ctrl.stateCache.IterateHierarchy(a.Spec.Destination.Server, k, func(child appv1.ResourceNode, appName string) {
				belongToAnotherApp := false
				if appName != "" {
					if _, exists, err := ctrl.appInformer.GetIndexer().GetByKey(ctrl.namespace + "/" + appName); exists && err == nil {
						belongToAnotherApp = true
					}
				}
				if !belongToAnotherApp {
					orphanedNodes = append(orphanedNodes, child)
				}
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
	return &appv1.ApplicationTree{Nodes: nodes, OrphanedNodes: orphanedNodes}, nil
}

func (ctrl *ApplicationController) managedResources(comparisonResult *comparisonResult) ([]*appv1.ResourceDiff, error) {
	items := make([]*appv1.ResourceDiff, len(comparisonResult.managedResources))
	for i := range comparisonResult.managedResources {
		res := comparisonResult.managedResources[i]
		item := appv1.ResourceDiff{
			Namespace: res.Namespace,
			Name:      res.Name,
			Group:     res.Group,
			Kind:      res.Kind,
			Hook:      res.Hook,
		}

		target := res.Target
		live := res.Live
		resDiff := res.Diff
		if res.Kind == kube.SecretKind && res.Group == "" {
			var err error
			target, live, err = diff.HideSecretData(res.Target, res.Live)
			if err != nil {
				return nil, err
			}
			compareOptions, err := ctrl.settingsMgr.GetResourceCompareOptions()
			if err != nil {
				return nil, err
			}
			resDiffPtr, err := diff.Diff(target, live,
				diff.WithNormalizer(comparisonResult.diffNormalizer),
				diff.WithLogr(logutils.NewLogrusLogger(log.New())),
				diff.IgnoreAggregatedRoles(compareOptions.IgnoreAggregatedRoles))
			if err != nil {
				return nil, err
			}
			resDiff = *resDiffPtr
		}

		if live != nil {
			data, err := json.Marshal(live)
			if err != nil {
				return nil, err
			}
			item.LiveState = string(data)
		} else {
			item.LiveState = "null"
		}

		if target != nil {
			data, err := json.Marshal(target)
			if err != nil {
				return nil, err
			}
			item.TargetState = string(data)
		} else {
			item.TargetState = "null"
		}
		item.PredictedLiveState = string(resDiff.PredictedLive)
		item.NormalizedLiveState = string(resDiff.NormalizedLive)

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

func (ctrl *ApplicationController) requestAppRefresh(appName string, compareWith *CompareWith, after *time.Duration) {
	key := fmt.Sprintf("%s/%s", ctrl.namespace, appName)

	if compareWith != nil && after != nil {
		ctrl.appComparisonTypeRefreshQueue.AddAfter(fmt.Sprintf("%s/%d", key, compareWith), *after)
	} else {
		if compareWith != nil {
			ctrl.refreshRequestedAppsMutex.Lock()
			ctrl.refreshRequestedApps[appName] = compareWith.Max(ctrl.refreshRequestedApps[appName])
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
		ctrl.processRequestedAppOperation(app)
	} else if app.DeletionTimestamp != nil && app.CascadedDeletion() {
		_, err = ctrl.finalizeApplicationDeletion(app)
		if err != nil {
			ctrl.setAppCondition(app, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionDeletionError,
				Message: err.Error(),
			})
			message := fmt.Sprintf("Unable to delete application resources: %v", err.Error())
			ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonStatusRefreshed, Type: v1.EventTypeWarning}, message)
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
			ctrl.requestAppRefresh(parts[1], CompareWith(compareWith).Pointer(), nil)
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
		return err
	}
	appsCount := 0
	for i := range apps {
		if apps[i].Spec.GetProject() == proj.Name {
			appsCount++
			break
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
	return !kube.IsCRD(obj) && !isSelfReferencedApp(app, kube.GetObjectRef(obj))
}

func (ctrl *ApplicationController) getPermittedAppLiveObjects(app *appv1.Application, proj *appv1.AppProject) (map[kube.ResourceKey]*unstructured.Unstructured, error) {
	objsMap, err := ctrl.stateCache.GetManagedLiveObjs(app, []*unstructured.Unstructured{})
	if err != nil {
		return nil, err
	}
	// Don't delete live resources which are not permitted in the app project
	for k, v := range objsMap {
		if !proj.IsLiveResourcePermitted(v, app.Spec.Destination.Server) {
			delete(objsMap, k)
		}
	}
	return objsMap, nil
}

func (ctrl *ApplicationController) finalizeApplicationDeletion(app *appv1.Application) ([]*unstructured.Unstructured, error) {
	logCtx := log.WithField("application", app.Name)
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

	err = argo.ValidateDestination(context.Background(), &app.Spec.Destination, ctrl.db)
	if err != nil {
		return nil, err
	}

	objsMap, err := ctrl.getPermittedAppLiveObjects(app, proj)
	if err != nil {
		return nil, err
	}

	objs := make([]*unstructured.Unstructured, 0)
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

	cluster, err := ctrl.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		return nil, err
	}
	config := metrics.AddMetricsTransportWrapper(ctrl.metricsServer, app, cluster.RESTConfig())

	filteredObjs := FilterObjectsForDeletion(objs)
	err = kube.RunAllAsync(len(filteredObjs), func(i int) error {
		obj := filteredObjs[i]
		return ctrl.kubectl.DeleteResource(context.Background(), config, obj.GroupVersionKind(), obj.GetName(), obj.GetNamespace(), false)
	})
	if err != nil {
		return objs, err
	}

	objsMap, err = ctrl.getPermittedAppLiveObjects(app, proj)
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
	err = ctrl.cache.SetAppManagedResources(app.Name, nil)
	if err != nil {
		return objs, err
	}
	err = ctrl.cache.SetAppResourcesTree(app.Name, nil)
	if err != nil {
		return objs, err
	}
	app.SetCascadedDeletion(false)
	var patch []byte
	patch, _ = json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": app.Finalizers,
		},
	})
	_, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(context.Background(), app.Name, types.MergePatchType, patch, metav1.PatchOptions{})
	if err != nil {
		return objs, err
	}

	logCtx.Infof("Successfully deleted %d resources", len(objs))
	ctrl.projectRefreshQueue.Add(fmt.Sprintf("%s/%s", app.Namespace, app.Spec.GetProject()))
	return objs, nil
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
	logCtx := log.WithField("application", app.Name)
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
		// If we get here, we are about process an operation but we notice it is already in progress.
		// We need to detect if the app object we pulled off the informer is stale and doesn't
		// reflect the fact that the operation is completed. We don't want to perform the operation
		// again. To detect this, always retrieve the latest version to ensure it is not stale.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(context.Background(), app.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			logCtx.Errorf("Failed to retrieve latest application state: %v", err)
			return
		}
		if !isOperationInProgress(freshApp) {
			logCtx.Infof("Skipping operation on stale application state")
			return
		}
		app = freshApp
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
				ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), &retryAfter)
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

	if state.Phase == synccommon.OperationRunning {
		// It's possible for an app to be terminated while we were operating on it. We do not want
		// to clobber the Terminated state with Running. Get the latest app state to check for this.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(context.Background(), app.ObjectMeta.Name, metav1.GetOptions{})
		if err == nil {
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
	if state.Phase.Completed() && !app.Operation.Sync.DryRun {
		// if we just completed an operation, force a refresh so that UI will report up-to-date
		// sync/health information
		if _, err := cache.MetaNamespaceKeyFunc(app); err == nil {
			// force app refresh with using CompareWithLatest comparison type and trigger app reconciliation loop
			ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), nil)
		} else {
			logCtx.Warnf("Fails to requeue application: %v", err)
		}
	}
}

func (ctrl *ApplicationController) setOperationState(app *appv1.Application, state *appv1.OperationState) {
	kube.RetryUntilSucceed(context.Background(), updateOperationStateTimeout, "Update application operation state", logutils.NewLogrusLogger(log.New()), func() error {
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
			log.Infof("No operation updates necessary to '%s'. Skipping patch", app.Name)
			return nil
		}
		patchJSON, err := json.Marshal(patch)
		if err != nil {
			return err
		}
		if app.Status.OperationState != nil && app.Status.OperationState.FinishedAt != nil && state.FinishedAt == nil {
			patchJSON, err = jsonpatch.MergeMergePatches(patchJSON, []byte(`{"status": {"operationState": {"finishedAt": null}}}`))
			if err != nil {
				return err
			}
		}

		appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace)
		_, err = appClient.Patch(context.Background(), app.Name, types.MergePatchType, patchJSON, metav1.PatchOptions{})
		if err != nil {
			// Stop retrying updating deleted application
			if apierr.IsNotFound(err) {
				return nil
			}
			return err
		}
		log.Infof("updated '%s' operation (phase: %s)", app.Name, state.Phase)
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
			ctrl.auditLogger.LogAppEvent(app, eventInfo, strings.Join(messages, " "))
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
	needRefresh, refreshType, comparisonLevel := ctrl.needRefreshAppStatus(origApp, ctrl.statusRefreshTimeout)

	if !needRefresh {
		return
	}

	app := origApp.DeepCopy()
	logCtx := log.WithFields(log.Fields{"application": app.Name})
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
		if err := ctrl.cache.GetAppManagedResources(app.Name, &managedResources); err != nil {
			logCtx.Warnf("Failed to get cached managed resources for tree reconciliation, fallback to full reconciliation")
		} else {
			var tree *appv1.ApplicationTree
			if tree, err = ctrl.getResourceTree(app, managedResources); err == nil {
				app.Status.Summary = tree.GetSummary()
				if err := ctrl.cache.SetAppResourcesTree(app.Name, tree); err != nil {
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
		return
	}

	var localManifests []string
	if opState := app.Status.OperationState; opState != nil && opState.Operation.Sync != nil {
		localManifests = opState.Operation.Sync.Manifests
	}

	revision := app.Spec.Source.TargetRevision
	if comparisonLevel == CompareWithRecent {
		revision = app.Status.Sync.Revision
	}

	now := metav1.Now()
	compareResult := ctrl.appStateManager.CompareAppState(app, project, revision, app.Spec.Source, refreshType == appv1.RefreshTypeHard, localManifests)
	for k, v := range compareResult.timings {
		logCtx = logCtx.WithField(k, v.Milliseconds())
	}

	ctrl.normalizeApplication(origApp, app)

	tree, err := ctrl.setAppManagedResources(app, compareResult)
	if err != nil {
		logCtx.Errorf("Failed to cache app resources: %v", err)
	} else {
		app.Status.Summary = tree.GetSummary()
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

	if app.Status.ReconciledAt == nil || comparisonLevel == CompareWithLatest {
		app.Status.ReconciledAt = &now
	}
	app.Status.Sync = *compareResult.syncStatus
	app.Status.Health = *compareResult.healthStatus
	app.Status.Resources = compareResult.resources
	sort.Slice(app.Status.Resources, func(i, j int) bool {
		return resourceStatusKey(app.Status.Resources[i]) < resourceStatusKey(app.Status.Resources[j])
	})
	app.Status.SourceType = compareResult.appSourceType
	ctrl.persistAppStatus(origApp, &app.Status)
	return
}

func resourceStatusKey(res appv1.ResourceStatus) string {
	return strings.Join([]string{res.Group, res.Kind, res.Namespace, res.Name}, "/")
}

// needRefreshAppStatus answers if application status needs to be refreshed.
// Returns true if application never been compared, has changed or comparison result has expired.
// Additionally returns whether full refresh was requested or not.
// If full refresh is requested then target and live state should be reconciled, else only live state tree should be updated.
func (ctrl *ApplicationController) needRefreshAppStatus(app *appv1.Application, statusRefreshTimeout time.Duration) (bool, appv1.RefreshType, CompareWith) {
	logCtx := log.WithFields(log.Fields{"application": app.Name})
	var reason string
	compareWith := CompareWithLatest
	refreshType := appv1.RefreshTypeNormal
	expired := app.Status.ReconciledAt == nil || app.Status.ReconciledAt.Add(statusRefreshTimeout).Before(time.Now().UTC())

	if requestedType, ok := app.IsRefreshRequested(); ok {
		// user requested app refresh.
		refreshType = requestedType
		reason = fmt.Sprintf("%s refresh requested", refreshType)
	} else if expired {
		// The commented line below mysteriously crashes if app.Status.ReconciledAt is nil
		// reason = fmt.Sprintf("comparison expired. reconciledAt: %v, expiry: %v", app.Status.ReconciledAt, statusRefreshTimeout)
		//TODO: find existing Golang bug or create a new one
		reconciledAtStr := "never"
		if app.Status.ReconciledAt != nil {
			reconciledAtStr = app.Status.ReconciledAt.String()
		}
		reason = fmt.Sprintf("comparison expired. reconciledAt: %v, expiry: %v", reconciledAtStr, statusRefreshTimeout)
	} else if !app.Spec.Source.Equals(app.Status.Sync.ComparedTo.Source) {
		reason = "spec.source differs"
	} else if !app.Spec.Destination.Equals(app.Status.Sync.ComparedTo.Destination) {
		reason = "spec.destination differs"
	} else if requested, level := ctrl.isRefreshRequested(app.Name); requested {
		compareWith = level
		reason = "controller refresh requested"
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
		if apierr.IsNotFound(err) {
			errorConditions = append(errorConditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Application referencing project %s which does not exist", app.Spec.Project),
			})
		} else {
			errorConditions = append(errorConditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionUnknownError,
				Message: err.Error(),
			})
		}
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
	logCtx := log.WithFields(log.Fields{"application": app.Name})
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
	logCtx := log.WithFields(log.Fields{"application": orig.Name})
	if orig.Status.Sync.Status != newStatus.Sync.Status {
		message := fmt.Sprintf("Updated sync status: %s -> %s", orig.Status.Sync.Status, newStatus.Sync.Status)
		ctrl.auditLogger.LogAppEvent(orig, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Type: v1.EventTypeNormal}, message)
	}
	if orig.Status.Health.Status != newStatus.Health.Status {
		message := fmt.Sprintf("Updated health status: %s -> %s", orig.Status.Health.Status, newStatus.Health.Status)
		ctrl.auditLogger.LogAppEvent(orig, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Type: v1.EventTypeNormal}, message)
	}
	var newAnnotations map[string]string
	if orig.GetAnnotations() != nil {
		newAnnotations = make(map[string]string)
		for k, v := range orig.GetAnnotations() {
			newAnnotations[k] = v
		}
		delete(newAnnotations, common.AnnotationKeyRefresh)
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
	logCtx.Debugf("patch: %s", string(patch))
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
	logCtx := log.WithFields(log.Fields{"application": app.Name})
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
	alreadyAttempted, attemptPhase := alreadyAttemptedSync(app, desiredCommitSHA)
	selfHeal := app.Spec.SyncPolicy.Automated.SelfHeal
	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:    desiredCommitSHA,
			Prune:       app.Spec.SyncPolicy.Automated.Prune,
			SyncOptions: app.Spec.SyncPolicy.SyncOptions,
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
			ctrl.requestAppRefresh(app.Name, CompareWithLatest.Pointer(), &retryAfter)
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
	ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonOperationStarted, Type: v1.EventTypeNormal}, message)
	logCtx.Info(message)
	return nil
}

// alreadyAttemptedSync returns whether or not the most recent sync was performed against the
// commitSHA and with the same app source config which are currently set in the app
func alreadyAttemptedSync(app *appv1.Application, commitSHA string) (bool, synccommon.OperationPhase) {
	if app.Status.OperationState == nil || app.Status.OperationState.Operation.Sync == nil || app.Status.OperationState.SyncResult == nil {
		return false, ""
	}
	if app.Status.OperationState.SyncResult.Revision != commitSHA {
		return false, ""
	}
	// Ignore differences in target revision, since we already just verified commitSHAs are equal,
	// and we do not want to trigger auto-sync due to things like HEAD != master
	specSource := app.Spec.Source.DeepCopy()
	specSource.TargetRevision = ""
	syncResSource := app.Status.OperationState.SyncResult.Source.DeepCopy()
	syncResSource.TargetRevision = ""
	return reflect.DeepEqual(app.Spec.Source, app.Status.OperationState.SyncResult.Source), app.Status.OperationState.Phase
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
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (apiruntime.Object, error) {
				return ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Watch(context.TODO(), options)
			},
		},
		&appv1.Application{},
		ctrl.statusRefreshTimeout,
		cache.Indexers{
			cache.NamespaceIndex: func(obj interface{}) ([]string, error) {
				app, ok := obj.(*appv1.Application)
				if ok {
					if err := argo.ValidateDestination(context.Background(), &app.Spec.Destination, ctrl.db); err != nil {
						ctrl.setAppCondition(app, appv1.ApplicationCondition{Type: appv1.ApplicationConditionInvalidSpecError, Message: err.Error()})
					}
				}

				return cache.MetaNamespaceIndexFunc(obj)
			},
			orphanedIndex: func(obj interface{}) (i []string, e error) {
				app, ok := obj.(*appv1.Application)
				if !ok {
					return nil, nil
				}

				proj, err := ctrl.getAppProj(app)
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
					log.WithField("application", newApp.Name).Info("Enabled automated sync")
					compareWith = CompareWithLatest.Pointer()
				}
				ctrl.requestAppRefresh(newApp.Name, compareWith, nil)
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

func (ctrl *ApplicationController) RegisterClusterSecretUpdater(ctx context.Context) {
	updater := NewClusterInfoUpdater(ctrl.stateCache, ctrl.db, ctrl.appLister.Applications(ctrl.namespace), ctrl.cache, ctrl.clusterFilter)
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
