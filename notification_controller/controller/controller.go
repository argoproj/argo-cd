package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/v2/util/notification/k8s"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v2/util/notification/settings"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/controller"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const (
	resyncPeriod = 60 * time.Second
)

var (
	applications = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	appProjects  = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects"}
)

func newAppProjClient(client dynamic.Interface, namespace string) dynamic.ResourceInterface {
	resClient := client.Resource(appProjects).Namespace(namespace)
	return resClient
}

type NotificationController interface {
	Run(ctx context.Context, processors int)
	Init(ctx context.Context) error
}

func NewController(
	k8sClient kubernetes.Interface,
	client dynamic.Interface,
	argocdService service.Service,
	namespace string,
	appLabelSelector string,
	registry *controller.MetricsRegistry,
	secretName string,
	configMapName string,
) *notificationController {
	appClient := client.Resource(applications)
	appInformer := newInformer(appClient.Namespace(namespace), appLabelSelector)
	appProjInformer := newInformer(newAppProjClient(client, namespace), "")
	secretInformer := k8s.NewSecretInformer(k8sClient, namespace, secretName)
	configMapInformer := k8s.NewConfigMapInformer(k8sClient, namespace, configMapName)
	apiFactory := api.NewFactory(settings.GetFactorySettings(argocdService, secretName, configMapName), namespace, secretInformer, configMapInformer)

	res := &notificationController{
		secretInformer:    secretInformer,
		configMapInformer: configMapInformer,
		appInformer:       appInformer,
		appProjInformer:   appProjInformer,
		apiFactory:        apiFactory}
	res.ctrl = controller.NewController(appClient, appInformer, apiFactory,
		controller.WithSkipProcessing(func(obj v1.Object) (bool, string) {
			app, ok := (obj).(*unstructured.Unstructured)
			if !ok {
				return false, ""
			}
			return !isAppSyncStatusRefreshed(app, log.WithField("app", obj.GetName())), "sync status out of date"
		}),
		controller.WithMetricsRegistry(registry),
		controller.WithAlterDestinations(res.alterDestinations))
	return res
}

func (c *notificationController) alterDestinations(obj v1.Object, destinations services.Destinations, cfg api.Config) services.Destinations {
	app, ok := (obj).(*unstructured.Unstructured)
	if !ok {
		return destinations
	}

	if proj := getAppProj(app, c.appProjInformer); proj != nil {
		destinations.Merge(subscriptions.NewAnnotations(proj.GetAnnotations()).GetDestinations(cfg.DefaultTriggers, cfg.ServiceDefaultTriggers))
		destinations.Merge(settings.GetLegacyDestinations(proj.GetAnnotations(), cfg.DefaultTriggers, cfg.ServiceDefaultTriggers))
	}
	return destinations
}

func newInformer(resClient dynamic.ResourceInterface, selector string) cache.SharedIndexInformer {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (object runtime.Object, err error) {
				options.LabelSelector = selector
				return resClient.List(context.Background(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = selector
				return resClient.Watch(context.Background(), options)
			},
		},
		&unstructured.Unstructured{},
		resyncPeriod,
		cache.Indexers{},
	)
	return informer
}

type notificationController struct {
	apiFactory        api.Factory
	ctrl              controller.NotificationController
	appInformer       cache.SharedIndexInformer
	appProjInformer   cache.SharedIndexInformer
	secretInformer    cache.SharedIndexInformer
	configMapInformer cache.SharedIndexInformer
}

func (c *notificationController) Init(ctx context.Context) error {
	go c.appInformer.Run(ctx.Done())
	go c.appProjInformer.Run(ctx.Done())
	go c.secretInformer.Run(ctx.Done())
	go c.configMapInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.appInformer.HasSynced, c.appProjInformer.HasSynced, c.secretInformer.HasSynced, c.configMapInformer.HasSynced) {
		return errors.New("Timed out waiting for caches to sync")
	}
	return nil
}

func (c *notificationController) Run(ctx context.Context, processors int) {
	c.ctrl.Run(processors, ctx.Done())
}

func getAppProj(app *unstructured.Unstructured, appProjInformer cache.SharedIndexInformer) *unstructured.Unstructured {
	projName, ok, err := unstructured.NestedString(app.Object, "spec", "project")
	if !ok || err != nil {
		return nil
	}
	projObj, ok, err := appProjInformer.GetIndexer().GetByKey(fmt.Sprintf("%s/%s", app.GetNamespace(), projName))
	if !ok || err != nil {
		return nil
	}
	proj, ok := projObj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	if proj.GetAnnotations() == nil {
		proj.SetAnnotations(map[string]string{})
	}
	return proj
}

// Checks if the application SyncStatus has been refreshed by Argo CD after an operation has completed
func isAppSyncStatusRefreshed(app *unstructured.Unstructured, logEntry *log.Entry) bool {
	_, ok, err := unstructured.NestedMap(app.Object, "status", "operationState")
	if !ok || err != nil {
		logEntry.Debug("No OperationState found, SyncStatus is assumed to be up-to-date")
		return true
	}

	phase, ok, err := unstructured.NestedString(app.Object, "status", "operationState", "phase")
	if !ok || err != nil {
		logEntry.Debug("No OperationPhase found, SyncStatus is assumed to be up-to-date")
		return true
	}
	switch phase {
	case "Failed", "Error", "Succeeded":
		finishedAtRaw, ok, err := unstructured.NestedString(app.Object, "status", "operationState", "finishedAt")
		if !ok || err != nil {
			logEntry.Debugf("No FinishedAt found for completed phase '%s', SyncStatus is assumed to be out-of-date", phase)
			return false
		}
		finishedAt, err := time.Parse(time.RFC3339, finishedAtRaw)
		if err != nil {
			logEntry.Warnf("Failed to parse FinishedAt '%s'", finishedAtRaw)
			return false
		}
		var reconciledAt, observedAt time.Time
		reconciledAtRaw, ok, err := unstructured.NestedString(app.Object, "status", "reconciledAt")
		if ok && err == nil {
			reconciledAt, _ = time.Parse(time.RFC3339, reconciledAtRaw)
		}
		observedAtRaw, ok, err := unstructured.NestedString(app.Object, "status", "observedAt")
		if ok && err == nil {
			observedAt, _ = time.Parse(time.RFC3339, observedAtRaw)
		}
		if finishedAt.After(reconciledAt) && finishedAt.After(observedAt) {
			logEntry.Debugf("SyncStatus out-of-date (FinishedAt=%v, ReconciledAt=%v, Observed=%v", finishedAt, reconciledAt, observedAt)
			return false
		}
		logEntry.Debugf("SyncStatus up-to-date (FinishedAt=%v, ReconciledAt=%v, Observed=%v", finishedAt, reconciledAt, observedAt)
	default:
		logEntry.Debugf("Found phase '%s', SyncStatus is assumed to be up-to-date", phase)
	}

	return true
}
