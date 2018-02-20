package controller

import (
	"context"

	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned"
	appinformers "github.com/argoproj/argo-cd/pkg/client/informers/externalversions"
	log "github.com/sirupsen/logrus"

	"time"

	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	appResyncPeriod = 10 * time.Minute
)

// ApplicationController is the controller for application resources.
type ApplicationController struct {
	kubeclientset        kubernetes.Interface
	applicationclientset appclientset.Interface
	appQueue             workqueue.RateLimitingInterface

	appInformer cache.SharedIndexInformer
}

// NewApplicationController creates new instance of ApplicationController.
func NewApplicationController(kubeclientset kubernetes.Interface, applicationclientset appclientset.Interface) *ApplicationController {
	appQueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
	return &ApplicationController{
		kubeclientset:        kubeclientset,
		applicationclientset: applicationclientset,
		appQueue:             appQueue,
		appInformer:          newApplicationInformer(applicationclientset, appQueue),
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
	defer ctrl.appQueue.Done(appKey)
	if shutdown {
		return false
	}
	return true
}

func (ctrl *ApplicationController) runWorker() {
	for ctrl.processNextItem() {
	}
}

func newApplicationInformer(appclientset appclientset.Interface, appQueue workqueue.RateLimitingInterface) cache.SharedIndexInformer {
	appInformerFactory := appinformers.NewSharedInformerFactory(
		appclientset,
		appResyncPeriod,
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
