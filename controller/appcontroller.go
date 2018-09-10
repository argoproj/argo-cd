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
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/health"
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
	kubectl               kube.Kubectl
	applicationClientset  appclientset.Interface
	auditLogger           *argo.AuditLogger
	appRefreshQueue       workqueue.RateLimitingInterface
	appOperationQueue     workqueue.RateLimitingInterface
	appInformer           cache.SharedIndexInformer
	appStateManager       AppStateManager
	statusRefreshTimeout  time.Duration
	repoClientset         reposerver.Clientset
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
	repoClientset reposerver.Clientset,
	appResyncPeriod time.Duration,
) *ApplicationController {
	db := db.NewDB(namespace, kubeClientset)
	kubectlCmd := kube.KubectlCmd{}
	appStateManager := NewAppStateManager(db, applicationClientset, repoClientset, namespace, kubectlCmd)
	ctrl := ApplicationController{
		namespace:             namespace,
		kubeClientset:         kubeClientset,
		kubectl:               kubectlCmd,
		applicationClientset:  applicationClientset,
		repoClientset:         repoClientset,
		appRefreshQueue:       workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		appOperationQueue:     workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		appStateManager:       appStateManager,
		db:                    db,
		statusRefreshTimeout:  appResyncPeriod,
		forceRefreshApps:      make(map[string]bool),
		forceRefreshAppsMutex: &sync.Mutex{},
		auditLogger:           argo.NewAuditLogger(namespace, kubeClientset, "application-controller"),
	}
	ctrl.appInformer = ctrl.newApplicationInformer()
	return &ctrl
}

// Run starts the Application CRD controller.
func (ctrl *ApplicationController) Run(ctx context.Context, statusProcessors int, operationProcessors int) {
	defer runtime.HandleCrash()
	defer ctrl.appRefreshQueue.ShutDown()

	go ctrl.appInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), ctrl.appInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}

	go ctrl.watchAppsResources()

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
	retryUntilSucceed(func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("Recovered from panic: %v\n", r)
			}
		}()
		config := item.RESTConfig()
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
	}, fmt.Sprintf("watch app resources on %s", item.Server), ctx, watchResourcesRetryTimeout)

}

func isClusterHasApps(apps []interface{}, cluster *appv1.Cluster) bool {
	for _, obj := range apps {
		if app, ok := obj.(*appv1.Application); ok && app.Spec.Destination.Server == cluster.Server {
			return true
		}
	}
	return false
}

// WatchAppsResources watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (ctrl *ApplicationController) watchAppsResources() {
	watchingClusters := make(map[string]struct {
		cancel  context.CancelFunc
		cluster *appv1.Cluster
	})

	retryUntilSucceed(func() error {
		clusterEventCallback := func(event *db.ClusterEvent) {
			info, ok := watchingClusters[event.Cluster.Server]
			hasApps := isClusterHasApps(ctrl.appInformer.GetStore().List(), event.Cluster)

			// cluster resources must be watched only if cluster has at least one app
			if (event.Type == watch.Deleted || !hasApps) && ok {
				info.cancel()
				delete(watchingClusters, event.Cluster.Server)
			} else if event.Type != watch.Deleted && !ok && hasApps {
				ctx, cancel := context.WithCancel(context.Background())
				watchingClusters[event.Cluster.Server] = struct {
					cancel  context.CancelFunc
					cluster *appv1.Cluster
				}{
					cancel:  cancel,
					cluster: event.Cluster,
				}
				go ctrl.watchClusterResources(ctx, *event.Cluster)
			}
		}

		onAppModified := func(obj interface{}) {
			if app, ok := obj.(*appv1.Application); ok {
				var cluster *appv1.Cluster
				info, infoOk := watchingClusters[app.Spec.Destination.Server]
				if infoOk {
					cluster = info.cluster
				} else {
					cluster, _ = ctrl.db.GetCluster(context.Background(), app.Spec.Destination.Server)
				}
				if cluster != nil {
					// trigger cluster event every time when app created/deleted to either start or stop watching resources
					clusterEventCallback(&db.ClusterEvent{Cluster: cluster, Type: watch.Modified})
				}
			}
		}

		ctrl.appInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{AddFunc: onAppModified, DeleteFunc: onAppModified})

		return ctrl.db.WatchClusters(context.Background(), clusterEventCallback)

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
				log.Warnf("Failed to %s: %+v, retrying in %v", desc, err, timeout)
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
		ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonStatusRefreshed, Action: "refresh_status"}, v1.EventTypeWarning)
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
	logCtx := log.WithField("application", app.Name)
	var state *appv1.OperationState
	// Recover from any unexpected panics and automatically set the status to be failed
	defer func() {
		if r := recover(); r != nil {
			logCtx.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
			state.Phase = appv1.OperationError
			if rerr, ok := r.(error); ok {
				state.Message = rerr.Error()
			} else {
				state.Message = fmt.Sprintf("%v", r)
			}
			ctrl.setOperationState(app, state)
		}
	}()
	if isOperationInProgress(app) {
		// If we get here, we are about process an operation but we notice it is already in progress.
		// We need to detect if the app object we pulled off the informer is stale and doesn't
		// reflect the fact that the operation is completed. We don't want to perform the operation
		// again. To detect this, always retrieve the latest version to ensure it is not stale.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
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
		logCtx.Infof("Resuming in-progress operation. phase: %s, message: %s", state.Phase, state.Message)
	} else {
		state = &appv1.OperationState{Phase: appv1.OperationRunning, Operation: *app.Operation, StartedAt: metav1.Now()}
		ctrl.setOperationState(app, state)
		logCtx.Infof("Initialized new operation: %v", *app.Operation)
	}
	ctrl.appStateManager.SyncAppState(app, state)

	if state.Phase == appv1.OperationRunning {
		// It's possible for an app to be terminated while we were operating on it. We do not want
		// to clobber the Terminated state with Running. Get the latest app state to check for this.
		freshApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace).Get(app.ObjectMeta.Name, metav1.GetOptions{})
		if err == nil {
			if freshApp.Status.OperationState != nil && freshApp.Status.OperationState.Phase == appv1.OperationTerminating {
				state.Phase = appv1.OperationTerminating
				state.Message = "operation is terminating"
				// after this, we will get requeued to the workqueue, but next time the
				// SyncAppState will operate in a Terminating phase, allowing the worker to perform
				// cleanup (e.g. delete jobs, workflows, etc...)
			}
		}
	}

	ctrl.setOperationState(app, state)
	if state.Phase.Completed() {
		// if we just completed an operation, force a refresh so that UI will report up-to-date
		// sync/health information
		ctrl.forceAppRefresh(app.ObjectMeta.Name)
	}
}

func (ctrl *ApplicationController) setOperationState(app *appv1.Application, state *appv1.OperationState) {
	retryUntilSucceed(func() error {
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
			ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Action: "refresh_status"}, v1.EventTypeNormal)
		}
		if reflect.DeepEqual(app.Status.OperationState, state) {
			log.Infof("No operation updates necessary to '%s'. Skipping patch", app.Name)
			return nil
		}
		patchJSON, err := json.Marshal(patch)
		if err != nil {
			return err
		}
		appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(ctrl.namespace)
		_, err = appClient.Patch(app.Name, types.MergePatchType, patchJSON)
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
	app, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}
	if !ctrl.needRefreshAppStatus(app, ctrl.statusRefreshTimeout) {
		return
	}

	app = app.DeepCopy()
	conditions, hasErrors := ctrl.refreshAppConditions(app)
	if hasErrors {
		comparisonResult := app.Status.ComparisonResult.DeepCopy()
		comparisonResult.Status = appv1.ComparisonStatusUnknown
		health := app.Status.Health.DeepCopy()
		health.Status = appv1.HealthStatusUnknown
		ctrl.updateAppStatus(app, comparisonResult, health, nil, conditions)
		return
	}

	comparisonResult, manifestInfo, compConditions, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: appv1.ApplicationConditionComparisonError, Message: err.Error()})
	} else {
		conditions = append(conditions, compConditions...)
	}

	var parameters []*appv1.ComponentParameter
	if manifestInfo != nil {
		parameters = manifestInfo.Params
	}

	healthState, err := setApplicationHealth(ctrl.kubectl, comparisonResult)
	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: appv1.ApplicationConditionComparisonError, Message: err.Error()})
	}

	syncErrCond := ctrl.autoSync(app, comparisonResult)
	if syncErrCond != nil {
		conditions = append(conditions, *syncErrCond)
	}

	ctrl.updateAppStatus(app, comparisonResult, healthState, parameters, conditions)
	return
}

// needRefreshAppStatus answers if application status needs to be refreshed.
// Returns true if application never been compared, has changed or comparison result has expired.
func (ctrl *ApplicationController) needRefreshAppStatus(app *appv1.Application, statusRefreshTimeout time.Duration) bool {
	var reason string
	expired := app.Status.ComparisonResult.ComparedAt.Add(statusRefreshTimeout).Before(time.Now().UTC())
	if ctrl.isRefreshForced(app.Name) {
		reason = "force refresh"
	} else if app.Status.ComparisonResult.Status == appv1.ComparisonStatusUnknown && expired {
		reason = "comparison status unknown"
	} else if !app.Spec.Source.Equals(app.Status.ComparisonResult.ComparedTo) {
		reason = "spec.source differs"
	} else if expired {
		reason = fmt.Sprintf("comparison expired. comparedAt: %v, expiry: %v", app.Status.ComparisonResult.ComparedAt, statusRefreshTimeout)
	}
	if reason != "" {
		log.Infof("Refreshing application '%s' status (%s)", app.Name, reason)
		return true
	}
	return false
}

func (ctrl *ApplicationController) refreshAppConditions(app *appv1.Application) ([]appv1.ApplicationCondition, bool) {
	conditions := make([]appv1.ApplicationCondition, 0)
	proj, err := argo.GetAppProject(&app.Spec, ctrl.applicationClientset, ctrl.namespace)
	if err != nil {
		if errors.IsNotFound(err) {
			conditions = append(conditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionInvalidSpecError,
				Message: fmt.Sprintf("Application referencing project %s which does not exist", app.Spec.Project),
			})
		} else {
			conditions = append(conditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionUnknownError,
				Message: err.Error(),
			})
		}
	} else {
		specConditions, err := argo.GetSpecErrors(context.Background(), &app.Spec, proj, ctrl.repoClientset, ctrl.db)
		if err != nil {
			conditions = append(conditions, appv1.ApplicationCondition{
				Type:    appv1.ApplicationConditionUnknownError,
				Message: err.Error(),
			})
		} else {
			conditions = append(conditions, specConditions...)
		}
	}

	// List of condition types which have to be reevaluated by controller; all remaining conditions should stay as is.
	reevaluateTypes := map[appv1.ApplicationConditionType]bool{
		appv1.ApplicationConditionInvalidSpecError:      true,
		appv1.ApplicationConditionUnknownError:          true,
		appv1.ApplicationConditionComparisonError:       true,
		appv1.ApplicationConditionSharedResourceWarning: true,
		appv1.ApplicationConditionSyncError:             true,
	}
	appConditions := make([]appv1.ApplicationCondition, 0)
	for i := 0; i < len(app.Status.Conditions); i++ {
		condition := app.Status.Conditions[i]
		if _, ok := reevaluateTypes[condition.Type]; !ok {
			appConditions = append(appConditions, condition)
		}
	}
	hasErrors := false
	for i := range conditions {
		condition := conditions[i]
		appConditions = append(appConditions, condition)
		if condition.IsError() {
			hasErrors = true
		}

	}
	return appConditions, hasErrors
}

// setApplicationHealth updates the health statuses of all resources performed in the comparison
func setApplicationHealth(kubectl kube.Kubectl, comparisonResult *appv1.ComparisonResult) (*appv1.HealthStatus, error) {
	var savedErr error
	appHealth := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if comparisonResult.Status == appv1.ComparisonStatusUnknown {
		appHealth.Status = appv1.HealthStatusUnknown
	}
	for i, resource := range comparisonResult.Resources {
		if resource.LiveState == "null" {
			resource.Health = appv1.HealthStatus{Status: appv1.HealthStatusMissing}
		} else {
			var obj unstructured.Unstructured
			err := json.Unmarshal([]byte(resource.LiveState), &obj)
			if err != nil {
				return nil, err
			}
			healthState, err := health.GetAppHealth(kubectl, &obj)
			if err != nil && savedErr == nil {
				savedErr = err
			}
			resource.Health = *healthState
		}
		comparisonResult.Resources[i] = resource
		if health.IsWorse(appHealth.Status, resource.Health.Status) {
			appHealth.Status = resource.Health.Status
		}
	}
	return &appHealth, savedErr
}

// updateAppStatus persists updates to application status. Detects if there patch
func (ctrl *ApplicationController) updateAppStatus(
	app *appv1.Application,
	comparisonResult *appv1.ComparisonResult,
	healthState *appv1.HealthStatus,
	parameters []*appv1.ComponentParameter,
	conditions []appv1.ApplicationCondition,
) {
	modifiedApp := app.DeepCopy()
	if comparisonResult != nil {
		modifiedApp.Status.ComparisonResult = *comparisonResult
		log.Infof("App %s comparison result: prev: %s. current: %s", app.Name, app.Status.ComparisonResult.Status, comparisonResult.Status)
	}
	if healthState != nil {
		modifiedApp.Status.Health = *healthState
	}
	if parameters != nil {
		modifiedApp.Status.Parameters = make([]appv1.ComponentParameter, len(parameters))
		for i := range parameters {
			modifiedApp.Status.Parameters[i] = *parameters[i]
		}
	}
	if conditions != nil {
		modifiedApp.Status.Conditions = conditions
	}
	origBytes, err := json.Marshal(app)
	if err != nil {
		log.Errorf("Error updating application %s (marshal orig app): %v", app.Name, err)
		return
	}
	modifiedBytes, err := json.Marshal(modifiedApp)
	if err != nil {
		log.Errorf("Error updating application %s (marshal modified app): %v", app.Name, err)
		return
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(origBytes, modifiedBytes, appv1.Application{})
	if err != nil {
		log.Errorf("Error calculating patch for app %s update: %v", app.Name, err)
		return
	}
	if string(patch) == "{}" {
		log.Infof("No status changes to %s. Skipping patch", app.Name)
		return
	}
	appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
	_, err = appClient.Patch(app.Name, types.MergePatchType, patch)
	if err != nil {
		log.Warnf("Error updating application %s: %v", app.Name, err)
	} else {
		log.Infof("Application %s update successful", app.Name)
	}
}

// autoSync will initiate a sync operation for an application configured with automated sync
func (ctrl *ApplicationController) autoSync(app *appv1.Application, comparisonResult *appv1.ComparisonResult) *appv1.ApplicationCondition {
	if app.Spec.SyncPolicy == nil || app.Spec.SyncPolicy.Automated == nil {
		return nil
	}
	logCtx := log.WithFields(log.Fields{"application": app.Name})
	if app.Operation != nil {
		logCtx.Infof("Skipping auto-sync: another operation is in progress")
		return nil
	}
	// Only perform auto-sync if we detect OutOfSync status. This is to prevent us from attempting
	// a sync when application is already in a Synced or Unknown state
	if comparisonResult.Status != appv1.ComparisonStatusOutOfSync {
		logCtx.Infof("Skipping auto-sync: application status is %s", comparisonResult.Status)
		return nil
	}
	desiredCommitSHA := comparisonResult.Revision
	// It is possible for manifests to remain OutOfSync even after a sync/kubectl apply (e.g.
	// auto-sync with pruning disabled). We need to ensure that we do not keep Syncing an
	// application in an infinite loop. To detect this, we only attempt the Sync if the revision
	// and parameter overrides do *not* appear in the application's most recent history.
	historyLen := len(app.Status.History)
	if historyLen > 0 {
		mostRecent := app.Status.History[historyLen-1]
		if mostRecent.Revision == desiredCommitSHA && reflect.DeepEqual(app.Spec.Source.ComponentParameterOverrides, mostRecent.ComponentParameterOverrides) {
			logCtx.Infof("Skipping auto-sync: most recent sync already to %s", desiredCommitSHA)
			return nil
		}
	}
	// If a sync failed, the revision will not make it's way into application history. We also need
	// to check the operationState to see if the last operation was the one we just attempted.
	if app.Status.OperationState != nil && app.Status.OperationState.SyncResult != nil {
		if app.Status.OperationState.SyncResult.Revision == desiredCommitSHA {
			logCtx.Warnf("Skipping auto-sync: failed previous sync attempt to %s", desiredCommitSHA)
			message := fmt.Sprintf("Failed sync attempt to %s: %s", desiredCommitSHA, app.Status.OperationState.Message)
			return &appv1.ApplicationCondition{Type: appv1.ApplicationConditionSyncError, Message: message}
		}
	}

	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision: desiredCommitSHA,
			Prune:    app.Spec.SyncPolicy.Automated.Prune,
		},
	}
	appIf := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
	_, err := argo.SetAppOperation(context.Background(), appIf, ctrl.auditLogger, app.Name, &op)
	if err != nil {
		logCtx.Errorf("Failed to initiate auto-sync to %s: %v", desiredCommitSHA, err)
		return &appv1.ApplicationCondition{Type: appv1.ApplicationConditionSyncError, Message: err.Error()}
	}
	logCtx.Infof("Initiated auto-sync to %s", desiredCommitSHA)
	return nil
}

func (ctrl *ApplicationController) newApplicationInformer() cache.SharedIndexInformer {
	appInformerFactory := appinformers.NewFilteredSharedInformerFactory(
		ctrl.applicationClientset,
		ctrl.statusRefreshTimeout,
		ctrl.namespace,
		func(options *metav1.ListOptions) {},
	)
	informer := appInformerFactory.Argoproj().V1alpha1().Applications().Informer()
	informer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(obj)
				if err == nil {
					ctrl.appRefreshQueue.Add(key)
					ctrl.appOperationQueue.Add(key)
				}
			},
			UpdateFunc: func(old, new interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err != nil {
					return
				}
				oldApp, oldOK := old.(*appv1.Application)
				newApp, newOK := new.(*appv1.Application)
				if oldOK && newOK {
					if toggledAutomatedSync(oldApp, newApp) {
						log.WithField("application", newApp.Name).Info("Enabled automated sync")
						ctrl.forceAppRefresh(newApp.Name)
					}
				}
				ctrl.appRefreshQueue.Add(key)
				ctrl.appOperationQueue.Add(key)
			},
			DeleteFunc: func(obj interface{}) {
				// IndexerInformer uses a delta queue, therefore for deletes we have to use this
				// key function.
				key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
				if err == nil {
					ctrl.appRefreshQueue.Add(key)
				}
			},
		},
	)
	return informer
}

func isOperationInProgress(app *appv1.Application) bool {
	return app.Status.OperationState != nil && !app.Status.OperationState.Phase.Completed()
}

// toggledAutomatedSync tests if an app went from auto-sync disabled to enabled.
// if it was toggled to be enabled, the informer handler will force a refresh
func toggledAutomatedSync(old *appv1.Application, new *appv1.Application) bool {
	if new.Spec.SyncPolicy == nil || new.Spec.SyncPolicy.Automated == nil {
		return false
	}
	// auto-sync is enabled. check if it was previously disabled
	if old.Spec.SyncPolicy == nil || old.Spec.SyncPolicy.Automated == nil {
		return true
	}
	// nothing changed
	return false
}
