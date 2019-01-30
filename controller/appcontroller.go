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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/apis/core"

	"github.com/argoproj/argo-cd/common"
	statecache "github.com/argoproj/argo-cd/controller/cache"
	"github.com/argoproj/argo-cd/controller/services"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/util"
	"github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/db"
	"github.com/argoproj/argo-cd/util/diff"
	grpc_util "github.com/argoproj/argo-cd/util/grpc"
	"github.com/argoproj/argo-cd/util/kube"
	settings_util "github.com/argoproj/argo-cd/util/settings"
	tlsutil "github.com/argoproj/argo-cd/util/tls"
)

const (
	updateOperationStateTimeout = 1 * time.Second
)

// ApplicationController is the controller for application resources.
type ApplicationController struct {
	namespace                 string
	kubeClientset             kubernetes.Interface
	kubectl                   kube.Kubectl
	applicationClientset      appclientset.Interface
	auditLogger               *argo.AuditLogger
	appRefreshQueue           workqueue.RateLimitingInterface
	appOperationQueue         workqueue.RateLimitingInterface
	appInformer               cache.SharedIndexInformer
	projInformer              cache.SharedIndexInformer
	appStateManager           AppStateManager
	stateCache                statecache.LiveStateCache
	statusRefreshTimeout      time.Duration
	repoClientset             reposerver.Clientset
	db                        db.ArgoDB
	settings                  *settings_util.ArgoCDSettings
	settingsMgr               *settings_util.SettingsManager
	refreshRequestedApps      map[string]bool
	refreshRequestedAppsMutex *sync.Mutex
	managedResources          map[string][]managedResource
	managedResourcesMutex     *sync.Mutex
}

type ApplicationControllerConfig struct {
	InstanceID string
	Namespace  string
}

// NewApplicationController creates new instance of ApplicationController.
func NewApplicationController(
	namespace string,
	settingsMgr *settings_util.SettingsManager,
	kubeClientset kubernetes.Interface,
	applicationClientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	appResyncPeriod time.Duration,
) (*ApplicationController, error) {
	db := db.NewDB(namespace, settingsMgr, kubeClientset)
	settings, err := settingsMgr.GetSettings()
	if err != nil {
		return nil, err
	}
	kubectlCmd := kube.KubectlCmd{}
	ctrl := ApplicationController{
		namespace:                 namespace,
		kubeClientset:             kubeClientset,
		kubectl:                   kubectlCmd,
		applicationClientset:      applicationClientset,
		repoClientset:             repoClientset,
		appRefreshQueue:           workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		appOperationQueue:         workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		db:                        db,
		statusRefreshTimeout:      appResyncPeriod,
		refreshRequestedApps:      make(map[string]bool),
		refreshRequestedAppsMutex: &sync.Mutex{},
		auditLogger:               argo.NewAuditLogger(namespace, kubeClientset, "argocd-application-controller"),
		managedResources:          make(map[string][]managedResource),
		managedResourcesMutex:     &sync.Mutex{},
		settingsMgr:               settingsMgr,
		settings:                  settings,
	}
	appInformer := ctrl.newApplicationInformer()
	projInformer := v1alpha1.NewAppProjectInformer(applicationClientset, namespace, appResyncPeriod, cache.Indexers{})
	stateCache := statecache.NewLiveStateCache(db, appInformer, ctrl.settings, kubectlCmd, func(appName string) {
		ctrl.requestAppRefresh(appName)
		ctrl.appRefreshQueue.Add(fmt.Sprintf("%s/%s", ctrl.namespace, appName))
	})
	appStateManager := NewAppStateManager(db, applicationClientset, repoClientset, namespace, kubectlCmd, ctrl.settings, stateCache, projInformer)
	ctrl.appInformer = appInformer
	ctrl.projInformer = projInformer
	ctrl.appStateManager = appStateManager
	ctrl.stateCache = stateCache
	return &ctrl, nil
}

func (ctrl *ApplicationController) getApp(name string) (*appv1.Application, error) {
	obj, exists, err := ctrl.appInformer.GetStore().GetByKey(fmt.Sprintf("%s/%s", ctrl.namespace, name))
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("unable to find application with name %s", name))
	}
	a, ok := (obj).(*appv1.Application)
	if !ok {
		return nil, status.Errorf(codes.Internal, fmt.Sprintf("unexpected object type in app informer"))
	}
	return a, nil
}

func (ctrl *ApplicationController) setAppManagedResources(appName string, resources []managedResource) {
	ctrl.managedResourcesMutex.Lock()
	defer ctrl.managedResourcesMutex.Unlock()
	ctrl.managedResources[appName] = resources
}

func (ctrl *ApplicationController) getAppManagedResources(appName string) []managedResource {
	ctrl.managedResourcesMutex.Lock()
	defer ctrl.managedResourcesMutex.Unlock()
	return ctrl.managedResources[appName]
}

func (ctrl *ApplicationController) ResourceTree(ctx context.Context, q *services.ResourcesQuery) (*services.ResourceTreeResponse, error) {
	a, err := ctrl.getApp(q.ApplicationName)
	if err != nil {
		return nil, err
	}
	managedResources := ctrl.getAppManagedResources(q.ApplicationName)
	items := make([]*appv1.ResourceNode, 0)
	for i := range managedResources {
		managedResource := managedResources[i]
		node := appv1.ResourceNode{
			Name:      managedResource.Name,
			Version:   managedResource.Version,
			Kind:      managedResource.Kind,
			Group:     managedResource.Group,
			Namespace: managedResource.Namespace,
		}
		if managedResource.Live != nil {
			node.ResourceVersion = managedResource.Live.GetResourceVersion()
			children, err := ctrl.stateCache.GetChildren(a.Spec.Destination.Server, managedResource.Live)
			if err != nil {
				return nil, err
			}
			node.Children = children
		}
		items = append(items, &node)
	}
	return &services.ResourceTreeResponse{Items: items}, nil
}

func (ctrl *ApplicationController) ManagedResources(ctx context.Context, q *services.ResourcesQuery) (*services.ManagedResourcesResponse, error) {
	resources := ctrl.getAppManagedResources(q.ApplicationName)
	items := make([]*appv1.ResourceDiff, len(resources))
	for i := range resources {
		res := resources[i]
		item := appv1.ResourceDiff{
			Namespace: res.Namespace,
			Name:      res.Name,
			Group:     res.Group,
			Kind:      res.Kind,
		}

		target := res.Target
		live := res.Live
		resDiff := res.Diff
		if res.Kind == kube.SecretKind && res.Group == "" {
			var err error
			target, live, err = hideSecretData(res.Target, res.Live)
			if err != nil {
				return nil, err
			}
			resDiff = *diff.Diff(target, live)
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
		jsonDiff, err := resDiff.JSONFormat()
		if err != nil {
			return nil, err
		}
		item.Diff = jsonDiff

		items[i] = &item
	}
	return &services.ManagedResourcesResponse{Items: items}, nil
}

func toString(val interface{}) string {
	if val == nil {
		return ""
	}
	return fmt.Sprintf("%s", val)
}

// hideSecretData replaces secret data values in specified target, live secrets and in last applied configuration of live secret with stars. Also preserves differences between
// target, live and last applied config values. E.g. if all three are equal the values would be replaced with same number of stars. If all the are different then number of stars
// in replacement should be different.
func hideSecretData(target *unstructured.Unstructured, live *unstructured.Unstructured) (*unstructured.Unstructured, *unstructured.Unstructured, error) {
	var orig *unstructured.Unstructured
	if live != nil {
		orig = diff.GetLastAppliedConfigAnnotation(live)
		live = live.DeepCopy()
	}
	if target != nil {
		target = target.DeepCopy()
	}

	keys := map[string]bool{}
	for _, obj := range []*unstructured.Unstructured{target, live, orig} {
		if obj == nil {
			continue
		}
		diff.NormalizeSecret(obj)
		if data, found, err := unstructured.NestedMap(obj.Object, "data"); found && err == nil {
			for k := range data {
				keys[k] = true
			}
		}
	}

	for k := range keys {
		nextReplacement := "*********"
		valToReplacement := make(map[string]string)
		for _, obj := range []*unstructured.Unstructured{target, live, orig} {
			var data map[string]interface{}
			if obj != nil {
				var err error
				data, _, err = unstructured.NestedMap(obj.Object, "data")
				if err != nil {
					return nil, nil, err
				}
			}
			if data == nil {
				data = make(map[string]interface{})
			}
			valData, ok := data[k]
			if !ok {
				continue
			}
			val := toString(valData)
			replacement, ok := valToReplacement[val]
			if !ok {
				replacement = nextReplacement
				nextReplacement = nextReplacement + "*"
				valToReplacement[val] = replacement
			}
			data[k] = replacement
			err := unstructured.SetNestedField(obj.Object, data, "data")
			if err != nil {
				return nil, nil, err
			}
		}
	}
	if live != nil && orig != nil {
		annotations := live.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}
		lastAppliedData, err := json.Marshal(orig)
		if err != nil {
			return nil, nil, err
		}
		annotations[core.LastAppliedConfigAnnotation] = string(lastAppliedData)
		live.SetAnnotations(annotations)
	}
	return target, live, nil
}

// Run starts the Application CRD controller.
func (ctrl *ApplicationController) Run(ctx context.Context, statusProcessors int, operationProcessors int) {
	defer runtime.HandleCrash()
	defer ctrl.appRefreshQueue.ShutDown()

	go ctrl.appInformer.Run(ctx.Done())
	go ctrl.projInformer.Run(ctx.Done())
	go ctrl.watchSettings(ctx)

	if !cache.WaitForCacheSync(ctx.Done(), ctrl.appInformer.HasSynced, ctrl.projInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}

	go ctrl.stateCache.Run(ctx)

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
		"argocd-application-controller",
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

func (ctrl *ApplicationController) requestAppRefresh(appName string) {
	ctrl.refreshRequestedAppsMutex.Lock()
	defer ctrl.refreshRequestedAppsMutex.Unlock()
	ctrl.refreshRequestedApps[appName] = true
}

func (ctrl *ApplicationController) isRefreshRequested(appName string) bool {
	ctrl.refreshRequestedAppsMutex.Lock()
	defer ctrl.refreshRequestedAppsMutex.Unlock()
	_, ok := ctrl.refreshRequestedApps[appName]
	if ok {
		delete(ctrl.refreshRequestedApps, appName)
	}
	return ok
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

	objsMap, err := ctrl.stateCache.GetManagedLiveObjs(app, []*unstructured.Unstructured{})
	if err != nil {
		return err
	}
	objs := make([]*unstructured.Unstructured, 0)
	for k := range objsMap {
		objs = append(objs, objsMap[k])
	}
	err = util.RunAllAsync(len(objs), func(i int) error {
		obj := objs[i]
		return ctrl.stateCache.Delete(app.Spec.Destination.Server, obj)
	})
	if err != nil {
		return err
	}

	objsMap, err = ctrl.stateCache.GetManagedLiveObjs(app, []*unstructured.Unstructured{})
	if err != nil {
		return err
	}
	if len(objsMap) > 0 {
		logCtx.Infof("%d objects remaining for deletion", len(objsMap))
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
		ctrl.requestAppRefresh(app.ObjectMeta.Name)
	}
}

func (ctrl *ApplicationController) setOperationState(app *appv1.Application, state *appv1.OperationState) {
	util.RetryUntilSucceed(func() error {
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
	origApp, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return
	}
	needRefresh, refreshType := ctrl.needRefreshAppStatus(origApp, ctrl.statusRefreshTimeout)

	if !needRefresh {
		return
	}
	startTime := time.Now()
	defer func() {
		logCtx := log.WithFields(log.Fields{"application": origApp.Name})
		logCtx.Infof("Reconciliation completed in %v", time.Now().Sub(startTime))
	}()
	// NOTE: normalization returns a copy
	app := ctrl.normalizeApplication(origApp)

	conditions, hasErrors := ctrl.refreshAppConditions(app)
	if hasErrors {
		app.Status.Sync.Status = appv1.SyncStatusCodeUnknown
		app.Status.Health.Status = appv1.HealthStatusUnknown
		app.Status.Conditions = conditions
		ctrl.persistAppStatus(origApp, &app.Status)
		return
	}

	compareResult, err := ctrl.appStateManager.CompareAppState(app, "", nil, refreshType == appv1.RefreshTypeHard)
	if err != nil {
		conditions = append(conditions, appv1.ApplicationCondition{Type: appv1.ApplicationConditionComparisonError, Message: err.Error()})
	} else {
		conditions = append(conditions, compareResult.conditions...)
	}
	ctrl.setAppManagedResources(app.Name, compareResult.managedResources)

	syncErrCond := ctrl.autoSync(app, compareResult.syncStatus)
	if syncErrCond != nil {
		conditions = append(conditions, *syncErrCond)
	}

	app.Status.ObservedAt = compareResult.observedAt
	app.Status.Sync = *compareResult.syncStatus
	app.Status.Health = *compareResult.healthStatus
	app.Status.Resources = compareResult.resources
	app.Status.Conditions = conditions
	ctrl.persistAppStatus(origApp, &app.Status)
	return
}

// needRefreshAppStatus answers if application status needs to be refreshed.
// Returns true if application never been compared, has changed or comparison result has expired.
func (ctrl *ApplicationController) needRefreshAppStatus(app *appv1.Application, statusRefreshTimeout time.Duration) (bool, appv1.RefreshType) {
	logCtx := log.WithFields(log.Fields{"application": app.Name})
	var reason string
	refreshType := appv1.RefreshTypeNormal
	expired := app.Status.ObservedAt.Add(statusRefreshTimeout).Before(time.Now().UTC())
	if requestedType, ok := app.IsRefreshRequested(); ok {
		refreshType = requestedType
		reason = fmt.Sprintf("%s refresh requested", refreshType)
	} else if ctrl.isRefreshRequested(app.Name) {
		reason = fmt.Sprintf("controller refresh requested")
	} else if app.Status.Sync.Status == appv1.SyncStatusCodeUnknown && expired {
		reason = "comparison status unknown"
	} else if !app.Spec.Source.Equals(app.Status.Sync.ComparedTo.Source) {
		reason = "spec.source differs"
	} else if !app.Spec.Destination.Equals(app.Status.Sync.ComparedTo.Destination) {
		reason = "spec.source differs"
	} else if expired {
		reason = fmt.Sprintf("comparison expired. observedAt: %v, expiry: %v", app.Status.ObservedAt, statusRefreshTimeout)
	}
	if reason != "" {
		logCtx.Infof("Refreshing app status (%s)", reason)
		return true, refreshType
	}
	return false, refreshType
}

func (ctrl *ApplicationController) refreshAppConditions(app *appv1.Application) ([]appv1.ApplicationCondition, bool) {
	conditions := make([]appv1.ApplicationCondition, 0)
	proj, err := argo.GetAppProject(&app.Spec, applisters.NewAppProjectLister(ctrl.projInformer.GetIndexer()), ctrl.namespace)
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

// normalizeApplication normalizes an application.spec and additionally persists updates if it changed
// Always returns a copy of the application
func (ctrl *ApplicationController) normalizeApplication(app *appv1.Application) *appv1.Application {
	logCtx := log.WithFields(log.Fields{"application": app.Name})
	appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace)
	modifiedApp := app.DeepCopy()
	modifiedApp.Spec = *argo.NormalizeApplicationSpec(&app.Spec)
	patch, modified, err := diff.CreateTwoWayMergePatch(app, modifiedApp, appv1.Application{})
	if err != nil {
		logCtx.Errorf("error constructing app spec patch: %v", err)
		return modifiedApp
	} else if modified {
		_, err = appClient.Patch(app.Name, types.MergePatchType, patch)
		if err != nil {
			logCtx.Errorf("Error persisting normalized application spec: %v", err)
		} else {
			logCtx.Infof("Normalized app spec: %s", string(patch))
		}
	}
	return modifiedApp
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
	appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(orig.Namespace)
	_, err = appClient.Patch(orig.Name, types.MergePatchType, patch)
	if err != nil {
		logCtx.Warnf("Error updating application: %v", err)
	} else {
		logCtx.Infof("Update successful")
	}
}

// autoSync will initiate a sync operation for an application configured with automated sync
func (ctrl *ApplicationController) autoSync(app *appv1.Application, syncStatus *appv1.SyncStatus) *appv1.ApplicationCondition {
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
	if syncStatus.Status != appv1.SyncStatusCodeOutOfSync {
		logCtx.Infof("Skipping auto-sync: application status is %s", syncStatus.Status)
		return nil
	}
	desiredCommitSHA := syncStatus.Revision

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
						ctrl.requestAppRefresh(newApp.Name)
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

func (ctrl *ApplicationController) watchSettings(ctx context.Context) {
	updateCh := make(chan *settings_util.ArgoCDSettings, 1)
	ctrl.settingsMgr.Subscribe(updateCh)
	prevAppLabelKey := ctrl.settings.GetAppInstanceLabelKey()
	done := false
	for !done {
		select {
		case newSettings := <-updateCh:
			newAppLabelKey := newSettings.GetAppInstanceLabelKey()
			*ctrl.settings = *newSettings
			if prevAppLabelKey != newAppLabelKey {
				log.Infof("label key changed: %s -> %s", prevAppLabelKey, newAppLabelKey)
				ctrl.stateCache.Invalidate()
				prevAppLabelKey = newAppLabelKey
			}
		case <-ctx.Done():
			done = true
		}
	}
	log.Info("shutting down settings watch")
	ctrl.settingsMgr.Unsubscribe(updateCh)
	close(updateCh)
}
