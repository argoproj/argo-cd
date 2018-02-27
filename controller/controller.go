package controller

import (
	"context"

	"github.com/argoproj/argo-cd/common"
	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"

	"time"

	"github.com/argoproj/argo-cd/application"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	appManager *application.Manager

	kubeClientset        kubernetes.Interface
	applicationClientset appclientset.Interface
	appQueue             workqueue.RateLimitingInterface
	appInformer          cache.SharedIndexInformer
}

type ApplicationControllerConfig struct {
	InstanceID string
	Namespace  string
}

// NewApplicationController creates new instance of ApplicationController.
func NewApplicationController(
	kubeClientset kubernetes.Interface,
	applicationClientset appclientset.Interface,
	appManager *application.Manager,
	appResyncPeriod time.Duration,
	config *ApplicationControllerConfig) *ApplicationController {
	appQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	return &ApplicationController{
		appManager:           appManager,
		kubeClientset:        kubeClientset,
		applicationClientset: applicationClientset,
		appQueue:             appQueue,
		appInformer:          newApplicationInformer(applicationClientset, appQueue, appResyncPeriod, config),
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

	updatedApp := app.DeepCopy()
	if ctrl.appManager.NeedRefreshAppStatus(updatedApp) || true {
		updatedApp.Status = *ctrl.appManager.RefreshAppStatus(updatedApp)
		ctrl.persistApp(updatedApp)
	}

	return true
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
