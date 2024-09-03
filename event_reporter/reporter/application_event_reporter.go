package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	"github.com/argoproj/argo-cd/v2/event_reporter/utils"

	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"

	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/pkg/codefresh"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/env"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/watch"

	appclient "github.com/argoproj/argo-cd/v2/event_reporter/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

var resourceEventCacheExpiration = time.Minute * time.Duration(env.ParseNumFromEnv(argocommon.EnvResourceEventCacheDuration, 20, 0, math.MaxInt32))

type applicationEventReporter struct {
	cache                    *servercache.Cache
	codefreshClient          codefresh.CodefreshClientInterface
	appLister                applisters.ApplicationLister
	applicationServiceClient appclient.ApplicationClient
	metricsServer            *metrics.MetricsServer
}

type ApplicationEventReporter interface {
	StreamApplicationEvents(
		ctx context.Context,
		a *appv1.Application,
		ts string,
		ignoreResourceCache bool,
		appInstanceLabelKey string,
		trackingMethod appv1.TrackingMethod,
	) error
	ShouldSendApplicationEvent(ae *appv1.ApplicationWatchEvent) (shouldSend bool, syncStatusChanged bool)
}

func NewApplicationEventReporter(cache *servercache.Cache, applicationServiceClient appclient.ApplicationClient, appLister applisters.ApplicationLister, codefreshConfig *codefresh.CodefreshConfig, metricsServer *metrics.MetricsServer) ApplicationEventReporter {
	return &applicationEventReporter{
		cache:                    cache,
		applicationServiceClient: applicationServiceClient,
		codefreshClient:          codefresh.NewCodefreshClient(codefreshConfig),
		appLister:                appLister,
		metricsServer:            metricsServer,
	}
}

func (s *applicationEventReporter) shouldSendResourceEvent(a *appv1.Application, rs appv1.ResourceStatus) bool {
	logCtx := utils.LogWithResourceStatus(log.WithFields(log.Fields{
		"app":      a.Name,
		"gvk":      fmt.Sprintf("%s/%s/%s", rs.Group, rs.Version, rs.Kind),
		"resource": fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
	}), rs)

	cachedRes, err := s.cache.GetLastResourceEvent(a, rs, utils.GetApplicationLatestRevision(a))
	if err != nil {
		logCtx.Debug("resource not in cache")
		return true
	}

	if reflect.DeepEqual(&cachedRes, &rs) {
		logCtx.Debug("resource status not changed")

		// status not changed
		return false
	}

	logCtx.Info("resource status changed")
	return true
}

func (r *applicationEventReporter) getDesiredManifests(ctx context.Context, a *appv1.Application, logCtx *log.Entry) (*apiclient.ManifestResponse, error, bool) {
	// get the desired state manifests of the application
	project := a.Spec.GetProject()
	desiredManifests, err := r.applicationServiceClient.GetManifests(ctx, &application.ApplicationManifestQuery{
		Name:         &a.Name,
		AppNamespace: &a.Namespace,
		Revision:     &a.Status.Sync.Revision,
		Project:      &project,
	})
	if err != nil {
		// if it's manifest generation error we need to still report the actual state
		// of the resources, but since we can't get the desired state, we will report
		// each resource with empty desired state
		logCtx.WithError(err).Warn("failed to get application desired state manifests, reporting actual state only")
		desiredManifests = &apiclient.ManifestResponse{Manifests: []*apiclient.Manifest{}}
		return desiredManifests, nil, true // will ignore requiresPruning=true to not delete resources with actual state
	}
	return desiredManifests, nil, false
}

func (s *applicationEventReporter) StreamApplicationEvents(
	ctx context.Context,
	a *appv1.Application,
	ts string,
	ignoreResourceCache bool,
	appInstanceLabelKey string,
	trackingMethod appv1.TrackingMethod,
) error {
	startTime := time.Now()
	logCtx := log.WithField("app", a.Name)

	logCtx.WithField("ignoreResourceCache", ignoreResourceCache).Info("streaming application events")

	project := a.Spec.GetProject()
	appTree, err := s.applicationServiceClient.ResourceTree(ctx, &application.ResourcesQuery{
		ApplicationName: &a.Name,
		Project:         &project,
		AppNamespace:    &a.Namespace,
	})
	if err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("failed to get application tree: %w", err)
		}

		// we still need process app even without tree, it is in case of app yaml originally contain error,
		// we still want to show it the erorrs that related to it on codefresh ui
		logCtx.WithError(err).Warn("failed to get application tree, resuming")
	}

	logCtx.Info("getting desired manifests")

	desiredManifests, err, manifestGenErr := s.getDesiredManifests(ctx, a, logCtx)
	if err != nil {
		return err
	}

	logCtx.Info("getting parent application name")

	parentAppIdentity := utils.GetParentAppIdentity(a, appInstanceLabelKey, trackingMethod)

	if utils.IsChildApp(parentAppIdentity) {
		logCtx.Info("processing as child application")
		parentApplicationEntity, err := s.applicationServiceClient.Get(ctx, &application.ApplicationQuery{
			Name:         &parentAppIdentity.Name,
			AppNamespace: &parentAppIdentity.Namespace,
		})
		if err != nil {
			return fmt.Errorf("failed to get parent application entity: %w", err)
		}

		rs := utils.GetAppAsResource(a)

		parentDesiredManifests, err, manifestGenErr := s.getDesiredManifests(ctx, parentApplicationEntity, logCtx)
		if err != nil {
			logCtx.WithError(err).Warn("failed to get parent application's desired manifests, resuming")
		}

		// helm app hasnt revision
		// TODO: add check if it helm application
		parentOperationRevision := utils.GetOperationRevision(parentApplicationEntity)
		parentRevisionMetadata, err := s.getApplicationRevisionDetails(ctx, parentApplicationEntity, parentOperationRevision)
		if err != nil {
			logCtx.WithError(err).Warn("failed to get parent application's revision metadata, resuming")
		}

		utils.SetHealthStatusIfMissing(rs)
		err = s.processResource(ctx, *rs, parentApplicationEntity, logCtx, ts, parentDesiredManifests, appTree, manifestGenErr, a, parentRevisionMetadata, appInstanceLabelKey, trackingMethod, desiredManifests.ApplicationVersions)
		if err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricChildAppEventType, metrics.MetricEventUnknownErrorType, a.Name)
			return err
		}
		reconcileDuration := time.Since(startTime)
		s.metricsServer.ObserveEventProcessingDurationHistogramDuration(a.Name, metrics.MetricChildAppEventType, reconcileDuration)
	} else {
		logCtx.Info("processing as root application")
		// will get here only for root applications (not managed as a resource by another application)
		appEvent, err := s.getApplicationEventPayload(ctx, a, ts, appInstanceLabelKey, trackingMethod, desiredManifests.ApplicationVersions)
		if err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricParentAppEventType, metrics.MetricEventGetPayloadErrorType, a.Name)
			return fmt.Errorf("failed to get application event: %w", err)
		}

		if appEvent == nil {
			// event did not have an OperationState - skip all events
			return nil
		}

		utils.LogWithAppStatus(a, logCtx, ts).Info("sending root application event")
		if err := s.codefreshClient.SendEvent(ctx, a.Name, appEvent); err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricParentAppEventType, metrics.MetricEventDeliveryErrorType, a.Name)
			return fmt.Errorf("failed to send event for root application %s/%s: %w", a.Namespace, a.Name, err)
		}
		reconcileDuration := time.Since(startTime)
		s.metricsServer.ObserveEventProcessingDurationHistogramDuration(a.Name, metrics.MetricParentAppEventType, reconcileDuration)
	}

	revisionMetadata, _ := s.getApplicationRevisionDetails(ctx, a, utils.GetOperationRevision(a))
	// for each resource in the application get desired and actual state,
	// then stream the event
	for _, rs := range a.Status.Resources {
		if utils.IsApp(rs) {
			continue
		}
		utils.SetHealthStatusIfMissing(&rs)
		if !ignoreResourceCache && !s.shouldSendResourceEvent(a, rs) {
			s.metricsServer.IncCachedIgnoredEventsCounter(metrics.MetricResourceEventType, a.Name)
			continue
		}
		err := s.processResource(ctx, rs, a, logCtx, ts, desiredManifests, appTree, manifestGenErr, nil, revisionMetadata, appInstanceLabelKey, trackingMethod, nil)
		if err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricResourceEventType, metrics.MetricEventUnknownErrorType, a.Name)
			return err
		}
	}
	return nil
}

func (s *applicationEventReporter) getAppForResourceReporting(
	rs appv1.ResourceStatus,
	ctx context.Context,
	a *appv1.Application,
	revisionMetadata *appv1.RevisionMetadata,
) (*appv1.Application, *appv1.RevisionMetadata) {
	if rs.Kind != "Rollout" { // for rollout it's crucial to report always correct operationSyncRevision
		return a, revisionMetadata
	}

	latestAppStatus, err := s.appLister.Applications(a.Namespace).Get(a.Name)
	if err != nil {
		return a, revisionMetadata
	}

	revisionMetadataToReport, err := s.getApplicationRevisionDetails(ctx, latestAppStatus, utils.GetOperationRevision(latestAppStatus))
	if err != nil {
		return a, revisionMetadata
	}

	return latestAppStatus, revisionMetadataToReport
}

func (s *applicationEventReporter) processResource(
	ctx context.Context,
	rs appv1.ResourceStatus,
	parentApplication *appv1.Application,
	logCtx *log.Entry,
	ts string,
	desiredManifests *apiclient.ManifestResponse,
	appTree *appv1.ApplicationTree,
	manifestGenErr bool,
	originalApplication *appv1.Application,
	revisionMetadata *appv1.RevisionMetadata,
	appInstanceLabelKey string,
	trackingMethod appv1.TrackingMethod,
	applicationVersions *apiclient.ApplicationVersions,
) error {
	metricsEventType := metrics.MetricResourceEventType
	if utils.IsApp(rs) {
		metricsEventType = metrics.MetricChildAppEventType
	}

	logCtx = logCtx.WithFields(log.Fields{
		"gvk":      fmt.Sprintf("%s/%s/%s", rs.Group, rs.Version, rs.Kind),
		"resource": fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
	})

	// get resource desired state
	desiredState := getResourceDesiredState(&rs, desiredManifests, logCtx)

	actualState, err := s.getResourceActualState(ctx, logCtx, metricsEventType, rs, parentApplication, originalApplication)
	if err != nil {
		return err
	}
	if actualState == nil {
		return nil
	}

	parentApplicationToReport, revisionMetadataToReport := s.getAppForResourceReporting(rs, ctx, parentApplication, revisionMetadata)

	var originalAppRevisionMetadata *appv1.RevisionMetadata = nil

	if originalApplication != nil {
		originalAppRevisionMetadata, _ = s.getApplicationRevisionDetails(ctx, originalApplication, utils.GetOperationRevision(originalApplication))
	}

	ev, err := getResourceEventPayload(parentApplicationToReport, &rs, actualState, desiredState, appTree, manifestGenErr, ts, originalApplication, revisionMetadataToReport, originalAppRevisionMetadata, appInstanceLabelKey, trackingMethod, applicationVersions)
	if err != nil {
		s.metricsServer.IncErroredEventsCounter(metricsEventType, metrics.MetricEventGetPayloadErrorType, parentApplication.Name)
		logCtx.WithError(err).Warn("failed to get event payload, resuming")
		return nil
	}

	appRes := appv1.Application{}
	appName := ""
	if utils.IsApp(rs) && actualState.Manifest != nil && json.Unmarshal([]byte(*actualState.Manifest), &appRes) == nil {
		utils.LogWithAppStatus(&appRes, logCtx, ts).Info("streaming resource event")
		appName = appRes.Name
	} else {
		utils.LogWithResourceStatus(logCtx, rs).Info("streaming resource event")
		appName = rs.Name
	}

	if err := s.codefreshClient.SendEvent(ctx, appName, ev); err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("failed to send resource event: %w", err)
		}

		s.metricsServer.IncErroredEventsCounter(metricsEventType, metrics.MetricEventDeliveryErrorType, appName)
		logCtx.WithError(err).Warn("failed to send resource event, resuming")
		return nil
	}

	if err := s.cache.SetLastResourceEvent(parentApplicationToReport, rs, resourceEventCacheExpiration, utils.GetApplicationLatestRevision(parentApplicationToReport)); err != nil {
		logCtx.WithError(err).Warn("failed to cache resource event")
	}

	return nil
}

func (s *applicationEventReporter) getResourceActualState(ctx context.Context, logCtx *log.Entry, metricsEventType metrics.MetricEventType, rs appv1.ResourceStatus, parentApplication *appv1.Application, childApplication *appv1.Application) (*application.ApplicationResourceResponse, error) {
	if utils.IsApp(rs) {
		if childApplication.IsEmptyTypeMeta() {
			// make sure there is type meta on object
			childApplication.SetDefaultTypeMeta()
		}

		manifestBytes, err := json.Marshal(childApplication)

		if err == nil && len(manifestBytes) > 0 {
			manifest := string(manifestBytes)
			return &application.ApplicationResourceResponse{Manifest: &manifest}, nil
		}
	}

	// get resource actual state
	project := parentApplication.Spec.GetProject()

	actualState, err := s.applicationServiceClient.GetResource(ctx, &application.ApplicationResourceRequest{
		Name:         &parentApplication.Name,
		AppNamespace: &parentApplication.Namespace,
		Namespace:    &rs.Namespace,
		ResourceName: &rs.Name,
		Version:      &rs.Version,
		Group:        &rs.Group,
		Kind:         &rs.Kind,
		Project:      &project,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			// only return error if there is no point in trying to send the
			// next resource. For example if the shared context has exceeded
			// its deadline
			if strings.Contains(err.Error(), "context deadline exceeded") {
				return nil, fmt.Errorf("failed to get actual state: %w", err)
			}

			s.metricsServer.IncErroredEventsCounter(metricsEventType, metrics.MetricEventUnknownErrorType, parentApplication.Name)
			logCtx.WithError(err).Warn("failed to get actual state, resuming")
			return nil, nil
		}

		manifest := ""
		// empty actual state
		actualState = &application.ApplicationResourceResponse{Manifest: &manifest}
	}

	return actualState, nil
}

func (s *applicationEventReporter) ShouldSendApplicationEvent(ae *appv1.ApplicationWatchEvent) (shouldSend bool, syncStatusChanged bool) {
	logCtx := log.WithField("app", ae.Application.Name)

	if ae.Type == watch.Deleted {
		logCtx.Info("application deleted")
		return true, false
	}

	cachedApp, err := s.cache.GetLastApplicationEvent(&ae.Application)
	if err != nil || cachedApp == nil {
		return true, false
	}

	cachedApp.Status.ReconciledAt = ae.Application.Status.ReconciledAt // ignore those in the diff
	cachedApp.Spec.Project = ae.Application.Spec.Project               // not using GetProject() so that the comparison will be with the real field values
	for i := range cachedApp.Status.Conditions {
		cachedApp.Status.Conditions[i].LastTransitionTime = nil
	}
	for i := range ae.Application.Status.Conditions {
		ae.Application.Status.Conditions[i].LastTransitionTime = nil
	}

	// check if application changed to healthy status
	if ae.Application.Status.Health.Status == health.HealthStatusHealthy && cachedApp.Status.Health.Status != health.HealthStatusHealthy {
		return true, true
	}

	if !reflect.DeepEqual(ae.Application.Spec, cachedApp.Spec) {
		logCtx.Info("application spec changed")
		return true, false
	}

	if !reflect.DeepEqual(ae.Application.Status, cachedApp.Status) {
		logCtx.Info("application status changed")
		return true, false
	}

	if !reflect.DeepEqual(ae.Application.Operation, cachedApp.Operation) {
		logCtx.Info("application operation changed")
		return true, false
	}

	metadataChanged := applicationMetadataChanged(ae, cachedApp)

	if metadataChanged {
		logCtx.Info("application metadata changed")
		return true, false
	}

	return false, false
}

func applicationMetadataChanged(ae *appv1.ApplicationWatchEvent, cachedApp *appv1.Application) (changed bool) {
	if ae.Type != watch.Modified {
		return false
	}

	cachedAppMeta := cachedApp.ObjectMeta.DeepCopy()
	newEventAppMeta := ae.Application.ObjectMeta.DeepCopy()

	if newEventAppMeta.Annotations != nil {
		delete(newEventAppMeta.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
		delete(cachedAppMeta.Annotations, "kubectl.kubernetes.io/last-applied-configuration")
	}

	cachedAppMeta.ResourceVersion = newEventAppMeta.ResourceVersion // ignore those in the diff
	cachedAppMeta.Generation = newEventAppMeta.Generation           // ignore those in the diff
	cachedAppMeta.GenerateName = newEventAppMeta.GenerateName       // ignore those in the diff
	newEventAppMeta.ManagedFields = nil                             // ignore those in the diff
	cachedAppMeta.ManagedFields = nil                               // ignore those in the diff

	return !reflect.DeepEqual(newEventAppMeta, cachedAppMeta)
}

func getResourceDesiredState(rs *appv1.ResourceStatus, ds *apiclient.ManifestResponse, logger *log.Entry) *apiclient.Manifest {
	if ds == nil {
		return &apiclient.Manifest{}
	}
	for _, m := range ds.Manifests {
		u, err := appv1.UnmarshalToUnstructured(m.CompiledManifest)
		if err != nil {
			logger.WithError(err).Warnf("failed to unmarshal compiled manifest")
			continue
		}

		if u == nil {
			continue
		}

		ns := text.FirstNonEmpty(u.GetNamespace(), rs.Namespace)

		if u.GroupVersionKind().String() == rs.GroupVersionKind().String() &&
			u.GetName() == rs.Name &&
			ns == rs.Namespace {
			if rs.Kind == kube.SecretKind && rs.Version == "v1" {
				m.RawManifest = m.CompiledManifest
			}

			return m
		}
	}

	// no desired state for resource
	// it's probably deleted from git
	return &apiclient.Manifest{}
}
