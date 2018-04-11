package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sync"

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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
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
	go ctrl.watchAppResources()

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

func (ctrl *ApplicationController) watchAppResources() error {
	watchingClusters := make(map[string]context.CancelFunc)
	clusters, err := ctrl.apiClusterService.List(context.Background(), &cluster.ClusterQuery{})
	if err != nil {
		return err
	}

	watchClusterResources := func(item appv1.Cluster) {
		config := item.RESTConfig()
		ctx, cancel := context.WithCancel(context.Background())
		watchingClusters[item.Server] = cancel
		ch, err := kube.WatchResourcesWithLabel(ctx, config, "", common.LabelApplicationName)
		if err == nil {
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
		}
	}

	for i := range clusters.Items {
		go watchClusterResources(clusters.Items[i])
	}

	err = ctrl.apiClusterService.WatchClusters(context.Background(), func(event *cluster.ClusterEvent) {
		cancel, ok := watchingClusters[event.Cluster.Server]
		if event.Type == watch.Deleted && ok {
			cancel()
			delete(watchingClusters, event.Cluster.Server)
		} else if event.Type != watch.Deleted && !ok {
			watchClusterResources(*event.Cluster)
		}
	})
	if err != nil {
		return err
	}

	<-context.Background().Done()
	return nil
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
		updatedApp := app.DeepCopy()
		status, err := ctrl.tryRefreshAppStatus(updatedApp)
		if err != nil {
			updatedApp.Status.ComparisonResult = appv1.ComparisonResult{
				Status:     appv1.ComparisonStatusError,
				Error:      fmt.Sprintf("Failed to get application status for application '%s': %v", app.Name, err),
				ComparedTo: app.Spec.Source,
				ComparedAt: metav1.Time{Time: time.Now().UTC()},
			}
		}
		ctrl.updateAppStatus(updatedApp.Name, updatedApp.Namespace, status)
	}

	return true
}

func (ctrl *ApplicationController) tryRefreshAppStatus(app *appv1.Application) (*appv1.ApplicationStatus, error) {
	conn, client, err := ctrl.repoClientset.NewRepositoryClient()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	targetObjs := make([]*unstructured.Unstructured, len(manifestInfo.Manifests))
	for i, manifestStr := range manifestInfo.Manifests {
		var obj unstructured.Unstructured
		if err := json.Unmarshal([]byte(manifestStr), &obj); err != nil {
			if err != nil {
				return nil, err
			}
		}
		targetObjs[i] = &obj
	}

	server, namespace := argoutil.ResolveServerNamespace(app.Spec.Destination, manifestInfo)
	comparisonResult, err := ctrl.appComparator.CompareAppState(server, namespace, targetObjs, app)
	if err != nil {
		return nil, err
	}
	log.Infof("App %s comparison result: prev: %s. current: %s", app.Name, app.Status.ComparisonResult.Status, comparisonResult.Status)
	newStatus := app.Status
	newStatus.ComparisonResult = *comparisonResult
	paramsReq := repository.EnvParamsRequest{
		Repo:        repo,
		Revision:    revision,
		Path:        app.Spec.Source.Path,
		Environment: app.Spec.Source.Environment,
	}
	params, err := client.GetEnvParams(context.Background(), &paramsReq)
	if err != nil {
		return nil, err
	}
	newStatus.Parameters = make([]appv1.ComponentParameter, len(params.Params))
	for i := range params.Params {
		newStatus.Parameters[i] = *params.Params[i]
	}
	return &newStatus, nil
}

func (ctrl *ApplicationController) runWorker() {
	for ctrl.processNextItem() {
	}
}

func (ctrl *ApplicationController) updateAppStatus(appName string, namespace string, status *appv1.ApplicationStatus) {
	appKey := fmt.Sprintf("%s/%s", namespace, appName)
	obj, exists, err := ctrl.appInformer.GetIndexer().GetByKey(appKey)
	if err != nil {
		log.Warnf("Failed to get application '%s' from informer index: %+v", appKey, err)
	} else {
		if exists {
			app := obj.(*appv1.Application).DeepCopy()
			app.Status = *status
			appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(namespace)
			_, err := appClient.Update(app)
			if err != nil {
				log.Warnf("Error updating application: %v", err)
			} else {
				log.Info("Application update successful")
			}
		}
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
