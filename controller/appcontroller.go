package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/kube"
)

const (
	watchResourcesRetryTimeout  = 10 * time.Second
	updateOperationStateTimeout = 1 * time.Second
)

// ApplicationController is the controller for application resources.
type ApplicationController struct {
	namespace             string
	kubeClientset         kubernetes.Interface
	applicationClientset  appclientset.Interface
	appRefreshQueue       workqueue.RateLimitingInterface
	appOperationQueue     workqueue.RateLimitingInterface
	appInformer           cache.SharedIndexInformer
	appStateManager       AppStateManager
	appHealthManager      AppHealthManager
	statusRefreshTimeout  time.Duration
	db                    db.ArgoDB
	forceRefreshApps      map[string]bool
	forceRefreshAppsMutex *sync.Mutex
}

type ApplicationControllerConfig struct {
	InstanceID string
	Namespace  string
}

// NewApplicationController creates new instance of ApplicationController.
func NewApplicationController(
	namespace string,
	kubeClientset kubernetes.Interface,
	applicationClientset appclientset.Interface,
	db db.ArgoDB,
	appStateManager AppStateManager,
	appHealthManager AppHealthManager,
	appResyncPeriod time.Duration,
	config *ApplicationControllerConfig,
) *ApplicationController {
	appRefreshQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	appOperationQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	return &ApplicationController{
		namespace:             namespace,
		kubeClientset:         kubeClientset,
		applicationClientset:  applicationClientset,
		appRefreshQueue:       appRefreshQueue,
		appOperationQueue:     appOperationQueue,
		appStateManager:       appStateManager,
		appHealthManager:      appHealthManager,
		appInformer:           newApplicationInformer(applicationClientset, appRefreshQueue, appOperationQueue, appResyncPeriod, config),
		db:                    db,
		statusRefreshTimeout:  appResyncPeriod,
		forceRefreshApps:      make(map[string]bool),
		forceRefreshAppsMutex: &sync.Mutex{},
	}
}

// Run starts the Application CRD controller.
func (ctrl *ApplicationController) Run(ctx context.Context, statusProcessors int, operationProcessors int) {
	defer runtime.HandleCrash()
	defer ctrl.appRefreshQueue.ShutDown()

	go ctrl.appInformer.Run(ctx.Done())
	go ctrl.watchAppsResources()

	if !cache.WaitForCacheSync(ctx.Done(), ctrl.appInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}

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

	<-ctx.Done()
}

func (ctrl *ApplicationController) forceAppRefresh(appName string) {
	ctrl.forceRefreshAppsMutex.Lock()
	defer ctrl.forceRefreshAppsMutex.Unlock()
	ctrl.forceRefreshApps[appName] = true
}

func (ctrl *ApplicationController) isRefreshForced(appName string) bool {
	ctrl.forceRefreshAppsMutex.Lock()
	defer ctrl.forceRefreshAppsMutex.Unlock()
	_, ok := ctrl.forceRefreshApps[appName]
	if ok {
		delete(ctrl.forceRefreshApps, appName)
	}
	return ok
}

// watchClusterResources watches for resource changes annotated with application label on specified cluster and schedule corresponding app refresh.
func (ctrl *ApplicationController) watchClusterResources(ctx context.Context, item appv1.Cluster) {
	config := item.RESTConfig()
	retryUntilSucceed(func() error {
		ch, err := kube.WatchResourcesWithLabel(ctx, config, "", common.LabelApplicationName)
		if err != nil {
			return err
		}
		for event := range ch {
			eventObj := event.Object.(*unstructured.Unstructured)
			objLabels := eventObj.GetLabels()
			if objLabels == nil {
				objLabels = make(map[string]string)
			}
			if appName, ok := objLabels[common.LabelApplicationName]; ok {
				ctrl.forceAppRefresh(appName)
				ctrl.appRefreshQueue.Add(ctrl.namespace + "/" + appName)
			}
		}
		return fmt.Errorf("resource updates channel has closed")
	}, fmt.Sprintf("watch app resources on %s", config.Host), ctx, watchResourcesRetryTimeout)

}

// WatchAppsResources watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (ctrl *ApplicationController) watchAppsResources() {
	watchingClusters := make(map[string]context.CancelFunc)

	retryUntilSucceed(func() error {
		return ctrl.db.WatchClusters(context.Background(), func(event *db.ClusterEvent) {
			cancel, ok := watchingClusters[event.Cluster.Server]
			if event.Type == watch.Deleted && ok {
				cancel()
				delete(watchingClusters, event.Cluster.Server)
			} else if event.Type != watch.Deleted && !ok {
				ctx, cancel := context.WithCancel(context.Background())
				watchingClusters[event.Cluster.Server] = cancel
				go ctrl.watchClusterResources(ctx, *event.Cluster)
			}
		})
	}, "watch clusters", context.Background(), watchResourcesRetryTimeout)

	<-context.Background().Done()
}

// retryUntilSucceed keep retrying given action with specified timeout until action succeed or specified context is done.
func retryUntilSucceed(action func() error, desc string, ctx context.Context, timeout time.Duration) {
	ctxCompleted := false
	go func() {
		select {
		case <-ctx.Done():
			ctxCompleted = true
		}
	}()
	for {
		err := action()
		if err == nil {
			return
		}
		if err != nil {
			if ctxCompleted {
				log.Infof("Stop retrying %s", desc)
				return
			} else {
				log.Warnf("Failed to %s: %v, retrying in %v", desc, err, timeout)
				time.Sleep(timeout)
			}
		}

	}
}

func (ctrl *ApplicationController) processAppOperationQueueItem() (processNext bool) {
	appKey, shutdown := ctrl.appOperationQueue.Get()
	if shutdown {
		processNext = false
		return
	} else {
		processNext = true
	}

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
	app, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}
	if app.Operation != nil {
		ctrl.processRequestedAppOperation(app)
	} else if app.DeletionTimestamp != nil && app.CascadedDeletion() {
		ctrl.finalizeApplicationDeletion(app)
	}

	return
}

func (ctrl *ApplicationController) finalizeApplicationDeletion(app *appv1.Application) {
	log.Infof("Deleting resources for application %s", app.Name)
	// Get refreshed application info, since informer app copy might be stale
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(app.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			log.Errorf("Unable to get refreshed application info prior deleting resources: %v", err)
		}
		return
	}

	clst, err := ctrl.db.GetCluster(context.Background(), app.Spec.Destination.Server)

	if err == nil {
		config := clst.RESTConfig()
		err = kube.DeleteResourceWithLabel(config, app.Spec.Destination.Namespace, common.LabelApplicationName, app.Name)
		if err == nil {
			app.SetCascadedDeletion(false)
			var patch []byte
			patch, err = json.Marshal(map[string]interface{}{
				"metadata": map[string]interface{}{
					"finalizers": app.Finalizers,
				},
			})
			if err == nil {
				_, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(app.Name, types.MergePatchType, patch)
			}
		}
	}
	if err != nil {
		log.Errorf("Unable to delete application resources: %v", err)
		ctrl.setAppCondition(app, appv1.ApplicationCondition{
			Type:    appv1.ApplicationConditionDeletionError,
			Message: err.Error(),
		})
	} else {
		log.Infof("Successfully deleted resources for application %s", app.Name)
	}
}

func (ctrl *ApplicationController) setAppCondition(app *appv1.Application, condition appv1.ApplicationCondition) {
	index := -1
	for i, exiting := range app.Status.Conditions {
		if exiting.Type == condition.Type {
			index = i
			break
		}
	}
	if index > -1 {
		app.Status.Conditions[index] = condition
	} else {
		app.Status.Conditions = append(app.Status.Conditions, condition)
	}
	var patch []byte
	patch, err := json.Marshal(map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": app.Status.Conditions,
		},
	})
	if err == nil {
		_, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(app.Name, types.MergePatchType, patch)
	}
	if err != nil {
		log.Errorf("Unable to set application condition: %v", err)
	}
}

func (ctrl *ApplicationController) processRequestedAppOperation(app *appv1.Application) {
	var state *appv1.OperationState
	// Recover from any unexpected panics and automatically set the status to be failed
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
			state.Phase = appv1.OperationError
			if rerr, ok := r.(error); ok {
				state.Message = rerr.Error()
			} else {
				state.Message = fmt.Sprintf("%v", r)
			}
			ctrl.setOperationState(app, state, app.Operation)
		}
	}()
	if isOperationRunning(app) {
		// If we get here, we are about process an operation but we notice it is already Running.
		// We need to detect if the controller crashed before completing the operation, or if the
		// the app object we pulled off the informer is simply stale and doesn't reflect the fact
		// that the operation is completed. We don't want to perform the operation again. To detect
		// this, always retrieve the latest version to ensure it is not stale.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		if err != nil {
			log.Errorf("Failed to retrieve latest application state: %v", err)
			return
		}
		if !isOperationRunning(freshApp) {
			log.Infof("Skipping operation on stale application state (%s)", app.ObjectMeta.Name)
			return
		}
		app = freshApp
		state = app.Status.OperationState.DeepCopy()
		log.Infof("Resuming in-progress operation %s %v", app.ObjectMeta.Name, state)
	} else {
		state = &appv1.OperationState{Phase: appv1.OperationRunning, Operation: *app.Operation, StartedAt: metav1.Now()}
		ctrl.setOperationState(app, state, app.Operation)
		log.Infof("Initialized new operation %s %v", app.ObjectMeta.Name, state)
	}
	ctrl.appStateManager.SyncAppState(app, state)
	ctrl.setOperationState(app, state, app.Operation)
}

func (ctrl *ApplicationController) setOperationState(app *appv1.Application, state *appv1.OperationState, operation *appv1.Operation) {
	retryUntilSucceed(func() error {
		var inProgressOpValue *appv1.Operation
		if state.Phase == "" {
			// expose any bugs where we neglect to set phase
			panic("no phase was set")
		}
		if !state.Phase.Completed() {
			// If operation is still running, we populate the app.operation field, which prevents
			// any other operation from running at the same time. Otherwise, it is cleared by setting
			// it to nil which indicates no operation is in progress.
			inProgressOpValue = operation
		} else {
			nowTime := metav1.Now()
			state.FinishedAt = &nowTime
		}

		if reflect.DeepEqual(app.Operation, inProgressOpValue) && reflect.DeepEqual(app.Status.OperationState, state) {
			log.Infof("no updates necessary to '%s' skipping patch", app.Name)
			return nil
		}

		patch, err := json.Marshal(map[string]interface{}{
			"status": map[string]interface{}{
				"operationState": state,
			},
			"operation": inProgressOpValue,
		})
		if err != nil {
			return err
		}
		appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace)
		_, err = appClient.Patch(app.Name, types.MergePatchType, patch)
		if err != nil {
			return err
		}
		log.Infof("updated '%s' operation (phase: %s)", app.Name, state.Phase)
		return nil
	}, "Update application operation state", context.Background(), updateOperationStateTimeout)
}

func (ctrl *ApplicationController) processAppRefreshQueueItem() (processNext bool) {
	appKey, shutdown := ctrl.appRefreshQueue.Get()
	if shutdown {
		processNext = false
		return
	} else {
		processNext = true
	}

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
	app, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}

	isForceRefreshed := ctrl.isRefreshForced(app.Name)
	if isForceRefreshed || app.NeedRefreshAppStatus(ctrl.statusRefreshTimeout) {
		log.Infof("Refreshing application '%s' status (force refreshed: %v)", app.Name, isForceRefreshed)

		comparisonResult, parameters, healthState, err := ctrl.tryRefreshAppStatus(app.DeepCopy())
		if err != nil {
			comparisonResult = &appv1.ComparisonResult{
				Status:     appv1.ComparisonStatusError,
				Error:      fmt.Sprintf("Failed to get application status for application '%s': %v", app.Name, err),
				ComparedTo: app.Spec.Source,
				ComparedAt: metav1.Time{Time: time.Now().UTC()},
			}
			parameters = nil
			healthState = &appv1.HealthStatus{Status: appv1.HealthStatusUnknown}
		}
		ctrl.updateAppStatus(app.Name, app.Namespace, comparisonResult, parameters, *healthState)
	}

	return
}

func (ctrl *ApplicationController) tryRefreshAppStatus(app *appv1.Application) (*appv1.ComparisonResult, *[]appv1.ComponentParameter, *appv1.HealthStatus, error) {
	comparisonResult, manifestInfo, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	if err != nil {
		return nil, nil, nil, err
	}
	log.Infof("App %s comparison result: prev: %s. current: %s", app.Name, app.Status.ComparisonResult.Status, comparisonResult.Status)

	parameters := make([]appv1.ComponentParameter, len(manifestInfo.Params))
	for i := range manifestInfo.Params {
		parameters[i] = *manifestInfo.Params[i]
	}
	healthState, err := ctrl.appHealthManager.GetAppHealth(app.Spec.Destination.Server, app.Spec.Destination.Namespace, comparisonResult)
	if err != nil {
		return nil, nil, nil, err
	}
	return comparisonResult, &parameters, healthState, nil
}

func (ctrl *ApplicationController) updateAppStatus(
	appName string, namespace string, comparisonResult *appv1.ComparisonResult, parameters *[]appv1.ComponentParameter, healthState appv1.HealthStatus) {
	statusPatch := make(map[string]interface{})
	statusPatch["comparisonResult"] = comparisonResult
	statusPatch["parameters"] = parameters
	statusPatch["health"] = healthState
	patch, err := json.Marshal(map[string]interface{}{
		"status": statusPatch,
	})

	if err == nil {
		appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(namespace)
		_, err = appClient.Patch(appName, types.MergePatchType, patch)
	}
	if err != nil {
		log.Warnf("Error updating application: %v", err)
	} else {
		log.Info("Application update successful")
	}
}

func newApplicationInformer(
	appClientset appclientset.Interface,
	appQueue workqueue.RateLimitingInterface,
	appOperationQueue workqueue.RateLimitingInterface,
	appResyncPeriod time.Duration,
	config *ApplicationControllerConfig) cache.SharedIndexInformer {

	appInformerFactory := appinformers.NewFilteredSharedInformerFactory(
		appClientset,
		appResyncPeriod,
		config.Namespace,
		func(options *metav1.ListOptions) {
			var instanceIDReq *labels.Requirement
			var err error
			if config.InstanceID != "" {
				instanceIDReq, err = labels.NewRequirement(common.LabelKeyApplicationControllerInstanceID, selection.Equals, []string{config.InstanceID})
			} else {
				instanceIDReq, err = labels.NewRequirement(common.LabelKeyApplicationControllerInstanceID, selection.DoesNotExist, nil)
			}
			if err != nil {
				panic(err)
			}

			options.FieldSelector = fields.Everything().String()
			labelSelector := labels.NewSelector().Add(*instanceIDReq)
			options.LabelSelector = labelSelector.String()
		},
	)
	informer := appInformerFactory.Argoproj().V1alpha1().Applications().Informer()
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					appQueue.Add(key)
					appOperationQueue.Add(key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					appQueue.Add(key)
					appOperationQueue.Add(key)
				}
			},
			DeleteFunc: func(obj interface{}) {
				// IndexerInformer uses a delta queue, therefore for deletes we have to use this
				// key function.
				key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				if err == nil {
					appQueue.Add(key)
				}
			},
		},
	)
	return informer
}

func isOperationRunning(app *appv1.Application) bool {
	return app.Status.OperationState != nil && !app.Status.OperationState.Phase.Completed()
}
