package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/v2/util/glob"

	"github.com/argoproj/argo-cd/v2/util/notification/k8s"

	service "github.com/argoproj/argo-cd/v2/util/notification/argocd"

	argocert "github.com/argoproj/argo-cd/v2/util/cert"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v2/util/notification/settings"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/controller"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"
)

const (
	resyncPeriod = 60 * time.Second
)

var (
	applications = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.ApplicationPlural}
	appProjects  = schema.GroupVersionResource{Group: application.Group, Version: "v1alpha1", Resource: application.AppProjectPlural}
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
	applicationNamespaces []string,
	appLabelSelector string,
	registry *controller.MetricsRegistry,
	secretName string,
	configMapName string,
	selfServiceNotificationEnabled bool,
) *notificationController {
	var appClient dynamic.ResourceInterface

	namespaceableAppClient := client.Resource(applications)
	appClient = namespaceableAppClient

	if len(applicationNamespaces) == 0 {
		appClient = namespaceableAppClient.Namespace(namespace)
	}
	appInformer := newInformer(appClient, namespace, applicationNamespaces, appLabelSelector)
	appProjInformer := newInformer(newAppProjClient(client, namespace), namespace, []string{namespace}, "")
	var notificationConfigNamespace string
	if selfServiceNotificationEnabled {
		notificationConfigNamespace = v1.NamespaceAll
	} else {
		notificationConfigNamespace = namespace
	}
	secretInformer := k8s.NewSecretInformer(k8sClient, notificationConfigNamespace, secretName)
	configMapInformer := k8s.NewConfigMapInformer(k8sClient, notificationConfigNamespace, configMapName)
	apiFactory := api.NewFactory(settings.GetFactorySettings(argocdService, secretName, configMapName, selfServiceNotificationEnabled), namespace, secretInformer, configMapInformer)

	res := &notificationController{
		secretInformer:    secretInformer,
		configMapInformer: configMapInformer,
		appInformer:       appInformer,
		appProjInformer:   appProjInformer,
		apiFactory:        apiFactory,
	}
	skipProcessingOpt := controller.WithSkipProcessing(func(obj v1.Object) (bool, string) {
		app, ok := (obj).(*unstructured.Unstructured)
		if !ok {
			return false, ""
		}
		if checkAppNotInAdditionalNamespaces(app, namespace, applicationNamespaces) {
			return true, "app is not in one of the application-namespaces, nor the notification controller namespace"
		}
		return !isAppSyncStatusRefreshed(app, log.WithField("app", obj.GetName())), "sync status out of date"
	})
	metricsRegistryOpt := controller.WithMetricsRegistry(registry)
	alterDestinationsOpt := controller.WithAlterDestinations(res.alterDestinations)

	if !selfServiceNotificationEnabled {
		res.ctrl = controller.NewController(namespaceableAppClient, appInformer, apiFactory,
			skipProcessingOpt,
			metricsRegistryOpt,
			alterDestinationsOpt)
	} else {
		res.ctrl = controller.NewControllerWithNamespaceSupport(namespaceableAppClient, appInformer, apiFactory,
			skipProcessingOpt,
			metricsRegistryOpt,
			alterDestinationsOpt)
	}
	return res
}

// Check if app is not in the namespace where the controller is in, and also app is not in one of the applicationNamespaces
func checkAppNotInAdditionalNamespaces(app *unstructured.Unstructured, namespace string, applicationNamespaces []string) bool {
	return namespace != app.GetNamespace() && !glob.MatchStringInList(applicationNamespaces, app.GetNamespace(), glob.REGEXP)
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

func newInformer(resClient dynamic.ResourceInterface, controllerNamespace string, applicationNamespaces []string, selector string) cache.SharedIndexInformer {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				// We are only interested in apps that exist in namespaces the
				// user wants to be enabled.
				options.LabelSelector = selector
				appList, err := resClient.List(context.TODO(), options)
				if err != nil {
					return nil, fmt.Errorf("failed to list applications: %w", err)
				}
				newItems := []unstructured.Unstructured{}
				for _, res := range appList.Items {
					if controllerNamespace == res.GetNamespace() || glob.MatchStringInList(applicationNamespaces, res.GetNamespace(), glob.REGEXP) {
						newItems = append(newItems, res)
					}
				}
				appList.Items = newItems
				return appList, nil
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = selector
				return resClient.Watch(context.TODO(), options)
			},
		},
		&unstructured.Unstructured{},
		resyncPeriod,
		cache.Indexers{
			cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
		},
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
	// resolve certificates using injected "argocd-tls-certs-cm" ConfigMap
	httputil.SetCertResolver(argocert.GetCertificateForConnect)

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
