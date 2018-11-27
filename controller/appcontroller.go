package controller

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/argoproj/argo-cd/common"
	"github.com/argoproj/argo-cd/controller/services"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util/argo"
	cache_util "github.com/argoproj/argo-cd/util/cache"
	"github.com/argoproj/argo-cd/util/db"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/health"
	"github.com/argoproj/argo-cd/util/kube"
	settings_util "github.com/argoproj/argo-cd/util/settings"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
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
	appResources          cache_util.Cache
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
	settingsMgr := settings_util.NewSettingsManager(kubeClientset, namespace)
	db := db.NewDB(namespace, settingsMgr, kubeClientset)
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
		appResources:          cache_util.NewInMemoryCache(24 * time.Hour),
	}
	ctrl.appInformer = ctrl.newApplicationInformer()
	return &ctrl
}

func (ctrl *ApplicationController) setAppResources(appName string, resources []appv1.ResourceState) {
	err := ctrl.appResources.Set(&cache_util.Item{Object: resources, Key: appName})
	if err != nil {
		log.Warnf("Unable to save app resources state in cache: %v", err)
	}
}

func (ctrl *ApplicationController) Resources(ctx context.Context, q *services.ResourcesQuery) (*services.ResourcesResponse, error) {
	resources := make([]appv1.ResourceState, 0)
	if q.ApplicationName == nil {
		return nil, status.Errorf(codes.InvalidArgument, "application name is not specified")
	}
	err := ctrl.appResources.Get(*q.ApplicationName, &resources)
	if err == nil {
		items := make([]*appv1.ResourceState, 0)
		for i := range resources {
			res := resources[i]
			obj, err := res.TargetObject()
			if err != nil {
				return nil, err
			}
			if obj == nil {
				obj, err = res.LiveObject()
				if err != nil {
					return nil, err
				}
			}
			if obj == nil {
				return nil, fmt.Errorf("both live and target objects are nil")
			}
			gvk := obj.GroupVersionKind()
			if q.Version != nil && gvk.Version != *q.Version {
				continue
			}
			if q.Group != nil && gvk.Group != *q.Group {
				continue
			}
			if q.Kind != nil && gvk.Kind != *q.Kind {
				continue
			}
			var data map[string]interface{}
			res.LiveState, data = hideSecretData(res.LiveState, nil)
			res.TargetState, _ = hideSecretData(res.TargetState, data)
			res.ChildLiveResources = hideNodesSecrets(res.ChildLiveResources)
			items = append(items, &res)
		}
		return &services.ResourcesResponse{Items: items}, nil
	}
	return &services.ResourcesResponse{Items: make([]*appv1.ResourceState, 0)}, nil
}

func toString(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%s", val)
}

// hideSecretData checks if given object kind is Secret, replaces data keys with stars and returns unchanged data map. The method additionally check if data key if different
// from corresponding key of optional parameter `otherData` and adds extra star to keep information about difference. So if secret data is out of sync user still can see which
// fields are different.
func hideSecretData(state string, otherData map[string]interface{}) (string, map[string]interface{}) {
	obj, err := appv1.UnmarshalToUnstructured(state)
	if err == nil {
		if obj != nil && obj.GetKind() == kube.SecretKind {
			if data, ok, err := unstructured.NestedMap(obj.Object, "data"); err == nil && ok {
				unchangedData := make(map[string]interface{})
				for k, v := range data {
					unchangedData[k] = v
				}
				for k := range data {
					replacement := "********"
					if otherData != nil {
						if val, ok := otherData[k]; ok && toString(val) != toString(data[k]) {
							replacement = replacement + "*"
						}
					}
					data[k] = replacement
				}
				_ = unstructured.SetNestedMap(obj.Object, data, "data")
				newState, err := json.Marshal(obj)
				if err == nil {
					return string(newState), unchangedData
				}
			}
		}
	}
	return state, nil
}

func hideNodesSecrets(nodes []appv1.ResourceNode) []appv1.ResourceNode {
	for i := range nodes {
		node := nodes[i]
		node.State, _ = hideSecretData(node.State, nil)
		node.Children = hideNodesSecrets(node.Children)
		nodes[i] = node
	}
	return nodes
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

func (ctrl *ApplicationController) CreateGRPC(tlsConfCustomizer tlsutil.ConfigCustomizer) (*grpc.Server, error) {
	// generate TLS cert
	hosts := []string{
		"localhost",
		"application-controller",
	}
	cert, err := tlsutil.GenerateX509KeyPair(tlsutil.CertOptions{
		Hosts:        hosts,
		Organization: "Argo CD",
		IsCA:         true,
	})

	if err != nil {
		return nil, err
	}

	tlsConfig := &tls.Config{Certificates: []tls.Certificate{*cert}}
	tlsConfCustomizer(tlsConfig)

	logEntry := log.NewEntry(log.New())
	server := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(tlsConfig)),
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer(
			grpc_logrus.StreamServerInterceptor(logEntry),
			grpc_util.PanicLoggerStreamServerInterceptor(logEntry),
		)),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
			grpc_logrus.UnaryServerInterceptor(logEntry),
			grpc_util.PanicLoggerUnaryServerInterceptor(logEntry),
		)),
	)
	services.RegisterApplicationServiceServer(server, ctrl)
	reflection.Register(server)
	return server, nil
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
		watchStartTime := time.Now()
		ch, err := ctrl.kubectl.WatchResources(ctx, config, "", func(gvk schema.GroupVersionKind) metav1.ListOptions {
			ops := metav1.ListOptions{}
			if !kube.IsCRDGroupVersionKind(gvk) {
				ops.LabelSelector = common.LabelApplicationName
			}
			return ops
		})

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
			}
			log.Warnf("Failed to %s: %+v, retrying in %v", desc, err, timeout)
			time.Sleep(timeout)
		}

	}
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
	app, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}
	if app.Operation != nil {
		ctrl.processRequestedAppOperation(app)
	} else if app.DeletionTimestamp != nil && app.CascadedDeletion() {
		err = ctrl.finalizeApplicationDeletion(app)
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

func (ctrl *ApplicationController) finalizeApplicationDeletion(app *appv1.Application) error {
	logCtx := log.WithField("application", app.Name)
	logCtx.Infof("Deleting resources")
	// Get refreshed application info, since informer app copy might be stale
	app, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(app.Name, metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			logCtx.Errorf("Unable to get refreshed application info prior deleting resources: %v", err)
		}
		return nil
	}
	clst, err := ctrl.db.GetCluster(context.Background(), app.Spec.Destination.Server)
	if err != nil {
		return err
	}
	config := clst.RESTConfig()
	err = kube.DeleteResourcesWithLabel(config, app.Spec.Destination.Namespace, common.LabelApplicationName, app.Name)
	if err != nil {
		return err
	}
	objs, err := kube.GetResourcesWithLabel(config, app.Spec.Destination.Namespace, common.LabelApplicationName, app.Name)
	if err != nil {
		return err
	}
	if len(objs) > 0 {
		logCtx.Info("%d objects remaining for deletion", len(objs))
		return nil
	}
	app.SetCascadedDeletion(false)
	var patch []byte
	patch, _ = json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers": app.Finalizers,
		},
	})
	_, err = ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Patch(app.Name, types.MergePatchType, patch)
	if err != nil {
		return err
	}
	// Purge the key from the cache
	err = ctrl.appResources.Delete(app.Name)
	if err != nil {
		logCtx.Warnf("Failed to purge app cache after deletion")
	}
	logCtx.Info("Successfully deleted resources")
	return nil
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
		}
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

	comparisonResult, manifestInfo, resources, compConditions, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: appv1.ApplicationConditionComparisonError, Message: err.Error()})
	} else {
		conditions = append(conditions, compConditions...)
	}

	var parameters []*appv1.ComponentParameter
	if manifestInfo != nil {
		parameters = manifestInfo.Params
	}

	healthState, err := setApplicationHealth(ctrl.kubectl, comparisonResult, resources)
	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: appv1.ApplicationConditionComparisonError, Message: err.Error()})
	}
	ctrl.setAppResources(app.Name, resources)

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
	logCtx := log.WithFields(log.Fields{"application": app.Name})
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
		logCtx.Infof("Refreshing app status (%s)", reason)
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
func setApplicationHealth(kubectl kube.Kubectl, comparisonResult *appv1.ComparisonResult, resources []appv1.ResourceState) (*appv1.HealthStatus, error) {
	var savedErr error
	appHealth := appv1.HealthStatus{Status: appv1.HealthStatusHealthy}
	if comparisonResult.Status == appv1.ComparisonStatusUnknown {
		appHealth.Status = appv1.HealthStatusUnknown
	}
	for i, resource := range resources {
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
		resources[i] = resource
		comparisonResult.Resources[i].Health = resource.Health
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
	logCtx := log.WithFields(log.Fields{"application": app.Name})
	modifiedApp := app.DeepCopy()
	if comparisonResult != nil {
		modifiedApp.Status.ComparisonResult = *comparisonResult
		if app.Status.ComparisonResult.Status != comparisonResult.Status {
			message := fmt.Sprintf("Updated sync status: %s -> %s", app.Status.ComparisonResult.Status, comparisonResult.Status)
			ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Type: v1.EventTypeNormal}, message)
		}
		logCtx.Infof("Comparison result: prev: %s. current: %s", app.Status.ComparisonResult.Status, comparisonResult.Status)
	}
	if healthState != nil {
		if modifiedApp.Status.Health.Status != healthState.Status {
			message := fmt.Sprintf("Updated health status: %s -> %s", modifiedApp.Status.Health.Status, healthState.Status)
			ctrl.auditLogger.LogAppEvent(app, argo.EventInfo{Reason: argo.EventReasonResourceUpdated, Type: v1.EventTypeNormal}, message)
		}
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
		logCtx.Errorf("Error updating (marshal orig app): %v", err)
		return
	}
	modifiedBytes, err := json.Marshal(modifiedApp)
	if err != nil {
		logCtx.Errorf("Error updating (marshal modified app): %v", err)
		return
	}
	patch, err := strategicpatch.CreateTwoWayMergePatch(origBytes, modifiedBytes, appv1.Application{})
	if err != nil {
		logCtx.Errorf("Error calculating patch for update: %v", err)
		return
	}
	if string(patch) == "{}" {
		logCtx.Infof("No status changes. Skipping patch")
		return
	}
	appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
	_, err = appClient.Patch(app.Name, types.MergePatchType, patch)
	if err != nil {
		logCtx.Warnf("Error updating application: %v", err)
	} else {
		logCtx.Infof("Update successful")
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
	// and parameter overrides are different from our most recent sync operation.
	if alreadyAttemptedSync(app, desiredCommitSHA) {
		if app.Status.OperationState.Phase != appv1.OperationSucceeded {
			logCtx.Warnf("Skipping auto-sync: failed previous sync attempt to %s", desiredCommitSHA)
			message := fmt.Sprintf("Failed sync attempt to %s: %s", desiredCommitSHA, app.Status.OperationState.Message)
			return &appv1.ApplicationCondition{Type: appv1.ApplicationConditionSyncError, Message: message}
		}
		logCtx.Infof("Skipping auto-sync: most recent sync already to %s", desiredCommitSHA)
		return nil
	}

	op := appv1.Operation{
		Sync: &appv1.SyncOperation{
			Revision:           desiredCommitSHA,
			Prune:              app.Spec.SyncPolicy.Automated.Prune,
			ParameterOverrides: app.Spec.Source.ComponentParameterOverrides,
		},
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
// commitSHA and with the same parameter overrides which are currently set in the app
func alreadyAttemptedSync(app *appv1.Application, commitSHA string) bool {
	if app.Status.OperationState == nil || app.Status.OperationState.Operation.Sync == nil || app.Status.OperationState.SyncResult == nil {
		return false
	}
	if app.Status.OperationState.SyncResult.Revision != commitSHA {
		return false
	}
	if !reflect.DeepEqual(appv1.ParameterOverrides(app.Spec.Source.ComponentParameterOverrides), app.Status.OperationState.Operation.Sync.ParameterOverrides) {
		return false
	}
	return true
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
