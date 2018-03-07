package controller

import (
	"context"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	"github.com/argoproj/argo-cd/util"
	log "github.com/sirupsen/logrus"

	"time"

	"fmt"

	"encoding/json"

	"github.com/argoproj/argo-cd/reposerver"
	"github.com/argoproj/argo-cd/reposerver/repository"
	apireposerver "github.com/argoproj/argo-cd/server/repository"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// ApplicationController is the controller for application resources.
type ApplicationController struct {
	repoClientset        reposerver.Clientset
	kubeClientset        kubernetes.Interface
	applicationClientset appclientset.Interface
	appQueue             workqueue.RateLimitingInterface
	appInformer          cache.SharedIndexInformer
	appComparator        AppComparator
	statusRefreshTimeout time.Duration
	apiRepoService       apireposerver.RepositoryServiceServer
}

type ApplicationControllerConfig struct {
	InstanceID string
	Namespace  string
}

// NewApplicationController creates new instance of ApplicationController.
func NewApplicationController(
	kubeClientset kubernetes.Interface,
	applicationClientset appclientset.Interface,
	repoClientset reposerver.Clientset,
	apiRepoService apireposerver.RepositoryServiceServer,
	appComparator AppComparator,
	appResyncPeriod time.Duration,
	config *ApplicationControllerConfig,
) *ApplicationController {
	appQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	return &ApplicationController{
		kubeClientset:        kubeClientset,
		applicationClientset: applicationClientset,
		repoClientset:        repoClientset,
		appQueue:             appQueue,
		apiRepoService:       apiRepoService,
		appComparator:        appComparator,
		appInformer:          newApplicationInformer(applicationClientset, appQueue, appResyncPeriod, config),
		statusRefreshTimeout: appResyncPeriod,
	}
}

// Run starts the Application CRD controller.
func (ctrl *ApplicationController) Run(ctx context.Context, appWorkers int) {
	defer runtime.HandleCrash()
	defer ctrl.appQueue.ShutDown()

	go ctrl.appInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), ctrl.appInformer.HasSynced) {
		log.Error("Timed out waiting for caches to sync")
		return
	}

	for i := 0; i < appWorkers; i++ {
		go wait.Until(ctrl.runWorker, time.Second, ctx.Done())
	}

	<-ctx.Done()
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

	if app.NeedRefreshAppStatus(ctrl.statusRefreshTimeout) {
		updatedApp := app.DeepCopy()
		status, err := ctrl.tryRefreshAppStatus(updatedApp)
		if err != nil {
			status = &appv1.ApplicationStatus{
				ComparisonResult: appv1.ComparisonResult{
					Status:     appv1.ComparisonStatusError,
					Error:      fmt.Sprintf("Failed to get application status for application '%s': %v", app.Name, err),
					ComparedTo: app.Spec.Source,
					ComparedAt: metav1.Time{Time: time.Now().UTC()},
				},
			}
		}
		updatedApp.Status = *status
		ctrl.persistApp(updatedApp)
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
		return nil, err
	}
	revision := app.Spec.Source.TargetRevision
	manifestInfo, err := client.GenerateManifest(context.Background(), &repository.ManifestRequest{
		Repo:        repo,
		Revision:    revision,
		Path:        app.Spec.Source.Path,
		Environment: app.Spec.Source.Environment,
	})
	if err != nil {
		return nil, err
	}
	targetObjs := make([]*unstructured.Unstructured, len(manifestInfo.Manifests))
	for i, manifestStr := range manifestInfo.Manifests {
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(manifestStr), &obj); err != nil {
			if err != nil {
				return nil, err
			}
		}
		targetObjs[i] = &unstructured.Unstructured{Object: obj}
	}
	comparisonResult, err := ctrl.appComparator.CompareAppState(manifestInfo.Server, manifestInfo.Namespace, targetObjs, app)
	if err != nil {
		return nil, err
	}
	log.Infof("App %s comparison result: prev: %s. current: %s", app.Name, app.Status.ComparisonResult.Status, comparisonResult.Status)
	return &appv1.ApplicationStatus{
		ComparisonResult: *comparisonResult,
	}, nil
}

func (ctrl *ApplicationController) runWorker() {
	for ctrl.processNextItem() {
	}
}

func (ctrl *ApplicationController) persistApp(app *appv1.Application) {
	appClient := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.ObjectMeta.Namespace)
	_, err := appClient.Update(app)
	if err != nil {
		log.Warnf("Error updating application: %v", err)
	}
	log.Info("Application update successful")
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
