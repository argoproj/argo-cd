package controller

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/argoproj/argo-cd/v3/util/glob"

	"github.com/argoproj/argo-cd/v3/util/configbus"
	"github.com/argoproj/argo-cd/v3/util/notification/k8s"

	service "github.com/argoproj/argo-cd/v3/util/notification/argocd"

	argocert "github.com/argoproj/argo-cd/v3/util/cert"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/argoproj/argo-cd/v3/util/notification/settings"

	"github.com/argoproj/notifications-engine/pkg/api"
	"github.com/argoproj/notifications-engine/pkg/controller"
	"github.com/argoproj/notifications-engine/pkg/services"
	"github.com/argoproj/notifications-engine/pkg/subscriptions"
	httputil "github.com/argoproj/notifications-engine/pkg/util/http"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"

	"github.com/argoproj/argo-cd/v3/pkg/apis/application"
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

type notificationController struct {
	ctrl                          controller.NotificationController
	appInformer                   cache.SharedIndexInformer
	appProjInformer               cache.SharedIndexInformer
	secretInformer                cache.SharedIndexInformer
	configMapInformer             cache.SharedIndexInformer
	configProvider                configbus.Provider
	controllerNamespace           string
	resolvedApplicationNamespaces []string

	// Deprecated: use configProvider.NotificationsSelfserviceEnabled.
	selfServiceNotificationEnabled bool
	// Deprecated: use configProvider.ApplicationNamespaces.
	applicationNamespaces []string
	// Deprecated: use configProvider.NotificationsConfigMapName.
	configMapName string
	// Deprecated: use configProvider.NotificationsSecretName.
	secretName string
	// Deprecated: use configProvider.NotificationsAppLabelSelector.
	appLabelSelector string
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
	crd configbus.CRDSource,
) (*notificationController, error) {
	res := &notificationController{
		selfServiceNotificationEnabled: selfServiceNotificationEnabled,
		applicationNamespaces:          applicationNamespaces,
		configMapName:                  configMapName,
		secretName:                     secretName,
		appLabelSelector:               appLabelSelector,
		controllerNamespace:            namespace,
	}
	res.InitConfigProvider(crd)

	applicationNamespacesCfg, err := res.configProvider.ApplicationNamespaces(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve notifications application namespaces: %w", err)
	}
	res.resolvedApplicationNamespaces = applicationNamespacesCfg
	appLabelSelectorCfg, err := res.configProvider.NotificationsAppLabelSelector(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve notifications app label selector: %w", err)
	}
	selfServiceCfg, err := res.configProvider.NotificationsSelfserviceEnabled(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve notifications selfservice enabled: %w", err)
	}
	secretNameCfg, err := res.configProvider.NotificationsSecretName(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve notifications secret name: %w", err)
	}
	configMapNameCfg, err := res.configProvider.NotificationsConfigMapName(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve notifications config map name: %w", err)
	}

	var appClient dynamic.ResourceInterface

	namespaceableAppClient := client.Resource(applications)
	appClient = namespaceableAppClient

	if len(applicationNamespacesCfg) == 0 {
		appClient = namespaceableAppClient.Namespace(namespace)
	}
	appInformer := newInformer(appClient, namespace, applicationNamespacesCfg, appLabelSelectorCfg)
	appProjInformer := newInformer(newAppProjClient(client, namespace), namespace, []string{namespace}, "")
	var notificationConfigNamespace string
	if selfServiceCfg {
		notificationConfigNamespace = metav1.NamespaceAll
	} else {
		notificationConfigNamespace = namespace
	}
	secretInformer := k8s.NewSecretInformer(k8sClient, notificationConfigNamespace, secretNameCfg)
	configMapInformer := k8s.NewConfigMapInformer(k8sClient, notificationConfigNamespace, configMapNameCfg)
	// Let the service serve the `appProject` template var from this AppProject
	// informer cache (keyed on the controller namespace) instead of a per-evaluation
	// API GET.
	if argocdService != nil {
		argocdService.SetAppProjectInformer(appProjInformer)
	}
	apiFactory := api.NewFactory(settings.GetFactorySettings(argocdService, secretNameCfg, configMapNameCfg, selfServiceCfg), namespace, secretInformer, configMapInformer)

	res.secretInformer = secretInformer
	res.configMapInformer = configMapInformer
	res.appInformer = appInformer
	res.appProjInformer = appProjInformer
	skipProcessingOpt := controller.WithSkipProcessing(func(obj metav1.Object) (bool, string) {
		app, ok := obj.(*unstructured.Unstructured)
		if !ok {
			return false, ""
		}
		if checkAppNotInAdditionalNamespaces(app, res.controllerNamespace, res.resolvedApplicationNamespaces) {
			return true, "app is not in one of the application-namespaces, nor the notification controller namespace"
		}
		return !isAppSyncStatusRefreshed(app, log.WithField("app", obj.GetName())), "sync status out of date"
	})
	metricsRegistryOpt := controller.WithMetricsRegistry(registry)
	alterDestinationsOpt := controller.WithAlterDestinations(res.alterDestinations)

	if !selfServiceCfg {
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
	return res, nil
}

// Check if app is not in the namespace where the controller is in, and also app is not in one of the applicationNamespaces
func checkAppNotInAdditionalNamespaces(app *unstructured.Unstructured, namespace string, applicationNamespaces []string) bool {
	return namespace != app.GetNamespace() && !glob.MatchStringInList(applicationNamespaces, app.GetNamespace(), glob.REGEXP)
}

func (c *notificationController) alterDestinations(obj metav1.Object, destinations services.Destinations, cfg api.Config) services.Destinations {
	app, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return destinations
	}

	if proj := getAppProj(app, c.appProjInformer, c.controllerNamespace); proj != nil {
		destinations.Merge(subscriptions.NewAnnotations(proj.GetAnnotations()).GetDestinations(cfg.DefaultTriggers, cfg.ServiceDefaultTriggers))
		destinations.Merge(settings.GetLegacyDestinations(proj.GetAnnotations(), cfg.DefaultTriggers, cfg.ServiceDefaultTriggers))
	}
	return destinations
}

func newInformer(resClient dynamic.ResourceInterface, controllerNamespace string, applicationNamespaces []string, selector string) cache.SharedIndexInformer {
	informer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListWithContextFunc: func(ctx context.Context, options metav1.ListOptions) (runtime.Object, error) {
				// We are only interested in apps that exist in namespaces the
				// user wants to be enabled.
				options.LabelSelector = selector
				appList, err := resClient.List(ctx, options)
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
			WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
				options.LabelSelector = selector
				return resClient.Watch(ctx, options)
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

func (c *notificationController) Init(ctx context.Context) error {
	// resolve certificates using injected "argocd-tls-certs-cm" ConfigMap
	httputil.SetCertResolver(argocert.GetCertificateForConnect)

	go c.appInformer.Run(ctx.Done())
	go c.appProjInformer.Run(ctx.Done())
	go c.secretInformer.Run(ctx.Done())
	go c.configMapInformer.Run(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), c.appInformer.HasSynced, c.appProjInformer.HasSynced, c.secretInformer.HasSynced, c.configMapInformer.HasSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	return nil
}

func (c *notificationController) Run(ctx context.Context, processors int) {
	c.ctrl.Run(processors, ctx.Done())
}

func getAppProj(app *unstructured.Unstructured, appProjInformer cache.SharedIndexInformer, controllerNamespace string) *unstructured.Unstructured {
	projName, _, err := unstructured.NestedString(app.Object, "spec", "project")
	if err != nil {
		return nil
	}
	// An empty or absent project means the app belongs to the 'default' project.
	if projName == "" {
		projName = "default"
	}
	// AppProjects live in the controller namespace, and the informer only watches
	// that namespace, so key on it rather than the application's namespace (which
	// misses for apps-in-any-namespace). See argoproj/argo-cd#28137.
	projObj, ok, err := appProjInformer.GetIndexer().GetByKey(fmt.Sprintf("%s/%s", controllerNamespace, projName))
	if !ok || err != nil {
		return nil
	}
	proj, ok := projObj.(*unstructured.Unstructured)
	if !ok {
		return nil
	}
	// Informer cache objects are shared; deep-copy before mutating.
	proj = proj.DeepCopy()
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
