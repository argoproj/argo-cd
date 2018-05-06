package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/server/cluster"
	apireposerver "github.com/argoproj/argo-cd/server/repository"
	"github.com/argoproj/argo-cd/util"
	argoutil "github.com/argoproj/argo-cd/util/argo"
	"github.com/argoproj/argo-cd/util/kube"
	log "github.com/sirupsen/logrus"
	"k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
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
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	watchResourcesRetryTimeout = 10 * time.Second
)

// ApplicationController is the controller for application resources.
type ApplicationController struct {
	namespace             string
	repoClientset         reposerver.Clientset
	kubeClientset         kubernetes.Interface
	applicationClientset  appclientset.Interface
	appQueue              workqueue.RateLimitingInterface
	appInformer           cache.SharedIndexInformer
	appComparator         AppComparator
	statusRefreshTimeout  time.Duration
	apiRepoService        apireposerver.RepositoryServiceServer
	apiClusterService     *cluster.Server
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
	apiRepoService apireposerver.RepositoryServiceServer,
	apiClusterService *cluster.Server,
	appComparator AppComparator,
	appResyncPeriod time.Duration,
	config *ApplicationControllerConfig,
) *ApplicationController {
	appQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	return &ApplicationController{
		namespace:             namespace,
		kubeClientset:         kubeClientset,
		applicationClientset:  applicationClientset,
		repoClientset:         repoClientset,
		appQueue:              appQueue,
		apiRepoService:        apiRepoService,
		apiClusterService:     apiClusterService,
		appComparator:         appComparator,
		appInformer:           newApplicationInformer(applicationClientset, appQueue, appResyncPeriod, config),
		statusRefreshTimeout:  appResyncPeriod,
		forceRefreshApps:      make(map[string]bool),
		forceRefreshAppsMutex: &sync.Mutex{},
	}
}

// Run starts the Application CRD controller.
func (ctrl *ApplicationController) Run(ctx context.Context, appWorkers int) {
	defer runtime.HandleCrash()
	defer ctrl.appQueue.ShutDown()

	go ctrl.appInformer.Run(ctx.Done())
	go ctrl.watchAppsResources()

	if !cache.WaitForCacheSync(ctx.Done(), ctrl.appInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}

	for i := 0; i < appWorkers; i++ {
		go wait.Until(ctrl.runWorker, time.Second, ctx.Done())
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
				ctrl.appQueue.Add(ctrl.namespace + "/" + appName)
			}
		}
		return fmt.Errorf("resource updates channel has closed")
	}, fmt.Sprintf("watch app resources on %s", config.Host), ctx, watchResourcesRetryTimeout)

}

// watchAppsResources watches for resource changes annotated with application label on all registered clusters and schedule corresponding app refresh.
func (ctrl *ApplicationController) watchAppsResources() {
	watchingClusters := make(map[string]context.CancelFunc)

	retryUntilSucceed(func() error {
		return ctrl.apiClusterService.WatchClusters(context.Background(), func(event *cluster.ClusterEvent) {
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

func (ctrl *ApplicationController) processNextItem() bool {
	appKey, shutdown := ctrl.appQueue.Get()
	if shutdown {
		return false
	}

	defer ctrl.appQueue.Done(appKey)

	obj, exists, err := ctrl.appInformer.GetIndexer().GetByKey(appKey.(string))
	if err != nil {
		log.Errorf("Failed to get application '%s' from informer index: %+v", appKey, err)
		return true
	}
	if !exists {
		// This happens after app was deleted, but the work queue still had an entry for it.
		return true
	}
	app, ok := obj.(*appv1.Application)
	if !ok {
		log.Warnf("Key '%s' in index is not an application", appKey)
		return true
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
			healthState = &appv1.HealthState{Status: appv1.HealthStatusUnknown}
		}
		ctrl.updateAppStatus(app.Name, app.Namespace, comparisonResult, parameters, *healthState)
	}

	return true
}

func (ctrl *ApplicationController) tryRefreshAppStatus(app *appv1.Application) (*appv1.ComparisonResult, *[]appv1.ComponentParameter, *appv1.HealthState, error) {
	conn, client, err := ctrl.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, nil, nil, err
	}
	defer util.Close(conn)
	repo, err := ctrl.apiRepoService.Get(context.Background(), &apireposerver.RepoQuery{Repo: app.Spec.Source.RepoURL})
	if err != nil {
		// If we couldn't retrieve from the repo service, assume public repositories
		repo = &appv1.Repository{
			Repo:     app.Spec.Source.RepoURL,
			Username: "",
			Password: "",
		}
	}
	overrides := make([]*appv1.ComponentParameter, len(app.Spec.Source.ComponentParameterOverrides))
	if app.Spec.Source.ComponentParameterOverrides != nil {
		for i := range app.Spec.Source.ComponentParameterOverrides {
			item := app.Spec.Source.ComponentParameterOverrides[i]
			overrides[i] = &item
		}
	}
	revision := app.Spec.Source.TargetRevision
	manifestInfo, err := client.GenerateManifest(context.Background(), &repository.ManifestRequest{
		Repo:                        repo,
		Revision:                    revision,
		Path:                        app.Spec.Source.Path,
		Environment:                 app.Spec.Source.Environment,
		AppLabel:                    app.Name,
		ComponentParameterOverrides: overrides,
	})
	if err != nil {
		log.Errorf("Failed to load application manifest %v", err)
		return nil, nil, nil, err
	}
	targetObjs := make([]*unstructured.Unstructured, len(manifestInfo.Manifests))
	for i, manifestStr := range manifestInfo.Manifests {
		var obj unstructured.Unstructured
		if err := json.Unmarshal([]byte(manifestStr), &obj); err != nil {
			if err != nil {
				return nil, nil, nil, err
			}
		}
		targetObjs[i] = &obj
	}

	server, namespace := argoutil.ResolveServerNamespace(app.Spec.Destination, manifestInfo)
	comparisonResult, err := ctrl.appComparator.CompareAppState(server, namespace, targetObjs, app)
	if err != nil {
		return nil, nil, nil, err
	}
	log.Infof("App %s comparison result: prev: %s. current: %s", app.Name, app.Status.ComparisonResult.Status, comparisonResult.Status)

	paramsReq := repository.EnvParamsRequest{
		Repo:        repo,
		Revision:    revision,
		Path:        app.Spec.Source.Path,
		Environment: app.Spec.Source.Environment,
	}
	params, err := client.GetEnvParams(context.Background(), &paramsReq)
	if err != nil {
		return nil, nil, nil, err
	}
	parameters := make([]appv1.ComponentParameter, len(params.Params))
	for i := range params.Params {
		parameters[i] = *params.Params[i]
	}
	healthState, err := ctrl.getAppHealthState(server, namespace, comparisonResult)
	if err != nil {
		return nil, nil, nil, err
	}
	return comparisonResult, &parameters, healthState, nil
}

func (ctrl *ApplicationController) getServiceHealthState(config *rest.Config, namespace string, name string) (*appv1.HealthState, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	service, err := clientSet.CoreV1().Services(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	healthState := appv1.HealthState{Status: appv1.HealthStatusHealthy}
	if service.Spec.Type == coreV1.ServiceTypeLoadBalancer {
		healthState.Status = appv1.HealthStatusProgressing
		for _, ingress := range service.Status.LoadBalancer.Ingress {
			if ingress.Hostname != "" || ingress.IP != "" {
				healthState.Status = appv1.HealthStatusHealthy
				break
			}
		}
	}
	return &healthState, nil
}

func (ctrl *ApplicationController) getDeploymentHealthState(config *rest.Config, namespace string, name string) (*appv1.HealthState, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	deploy, err := clientSet.AppsV1().Deployments(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	healthState := appv1.HealthState{
		Status: appv1.HealthStatusUnknown,
	}
	for _, condition := range deploy.Status.Conditions {
		// deployment is healthy is it successfully progressed
		if condition.Type == v1.DeploymentProgressing && condition.Status == "True" {
			healthState.Status = appv1.HealthStatusHealthy
		} else if condition.Type == v1.DeploymentReplicaFailure && condition.Status == "True" {
			healthState.Status = appv1.HealthStatusDegraded
		} else if condition.Type == v1.DeploymentProgressing && condition.Status == "False" {
			healthState.Status = appv1.HealthStatusDegraded
		} else if condition.Type == v1.DeploymentAvailable && condition.Status == "False" {
			healthState.Status = appv1.HealthStatusDegraded
		}
		if healthState.Status != appv1.HealthStatusUnknown {
			healthState.StatusDetails = fmt.Sprintf("%s:%s", condition.Reason, condition.Message)
			break
		}
	}
	return &healthState, nil
}

func (ctrl *ApplicationController) getAppHealthState(server string, namespace string, comparisonResult *appv1.ComparisonResult) (*appv1.HealthState, error) {
	clst, err := ctrl.apiClusterService.Get(context.Background(), &cluster.ClusterQuery{Server: server})
	if err != nil {
		return nil, err
	}
	restConfig := clst.RESTConfig()

	appHealthState := appv1.HealthState{Status: appv1.HealthStatusHealthy}
	for i := range comparisonResult.Resources {
		resource := comparisonResult.Resources[i]
		if resource.LiveState == "null" {
			resource.HealthState = appv1.HealthState{Status: appv1.HealthStatusUnknown}
		} else {
			var obj unstructured.Unstructured
			err := json.Unmarshal([]byte(resource.LiveState), &obj)
			if err != nil {
				return nil, err
			}
			switch obj.GetKind() {
			case kube.DeploymentKind:
				state, err := ctrl.getDeploymentHealthState(restConfig, namespace, obj.GetName())
				if err != nil {
					return nil, err
				}
				resource.HealthState = *state
			case kube.ServiceKind:
				state, err := ctrl.getServiceHealthState(restConfig, namespace, obj.GetName())
				if err != nil {
					return nil, err
				}
				resource.HealthState = *state
			default:
				resource.HealthState = appv1.HealthState{Status: appv1.HealthStatusHealthy}
			}

			if resource.HealthState.Status == appv1.HealthStatusProgressing {
				if appHealthState.Status == appv1.HealthStatusHealthy {
					appHealthState.Status = appv1.HealthStatusProgressing
				}
			} else if resource.HealthState.Status == appv1.HealthStatusDegraded {
				if appHealthState.Status == appv1.HealthStatusHealthy || appHealthState.Status == appv1.HealthStatusProgressing {
					appHealthState.Status = appv1.HealthStatusDegraded
				}
			}
		}
		comparisonResult.Resources[i] = resource
	}
	return &appHealthState, nil
}

func (ctrl *ApplicationController) runWorker() {
	for ctrl.processNextItem() {
	}
}

func (ctrl *ApplicationController) updateAppStatus(
	appName string, namespace string, comparisonResult *appv1.ComparisonResult, parameters *[]appv1.ComponentParameter, healthState appv1.HealthState) {
	statusPatch := make(map[string]interface{})
	statusPatch["comparisonResult"] = comparisonResult
	statusPatch["parameters"] = parameters
	statusPatch["healthState"] = healthState
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
	appClientset appclientset.Interface, appQueue workqueue.RateLimitingInterface, appResyncPeriod time.Duration, config *ApplicationControllerConfig) cache.SharedIndexInformer {

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
				}
			},
			UpdateFunc: func(old, new interface{}) {
				key, err := cache.MetaNamespaceKeyFunc(new)
				if err == nil {
					appQueue.Add(key)
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
