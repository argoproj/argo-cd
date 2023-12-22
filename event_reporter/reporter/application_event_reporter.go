package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strings"
	"time"

	argocommon "github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/event_reporter/codefresh"
	"github.com/argoproj/argo-cd/v2/event_reporter/metrics"
	applicationpkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	applisters "github.com/argoproj/argo-cd/v2/pkg/client/listers/application/v1alpha1"
	servercache "github.com/argoproj/argo-cd/v2/server/cache"
	"github.com/argoproj/argo-cd/v2/util/env"

	"github.com/argoproj/argo-cd/v2/util/argo"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/argoproj/gitops-engine/pkg/utils/text"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/yaml"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	appv1reg "github.com/argoproj/argo-cd/v2/pkg/apis/application"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

var (
	resourceEventCacheExpiration = time.Minute * time.Duration(env.ParseNumFromEnv(argocommon.EnvResourceEventCacheDuration, 20, 0, math.MaxInt32))
)

type applicationEventReporter struct {
	cache                    *servercache.Cache
	codefreshClient          codefresh.CodefreshClient
	appLister                applisters.ApplicationLister
	applicationServiceClient applicationpkg.ApplicationServiceClient
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

func NewApplicationEventReporter(cache *servercache.Cache, applicationServiceClient applicationpkg.ApplicationServiceClient, appLister applisters.ApplicationLister, codefreshConfig *codefresh.CodefreshConfig, metricsServer *metrics.MetricsServer) ApplicationEventReporter {
	return &applicationEventReporter{
		cache:                    cache,
		applicationServiceClient: applicationServiceClient,
		codefreshClient:          codefresh.NewCodefreshClient(codefreshConfig),
		appLister:                appLister,
		metricsServer:            metricsServer,
	}
}

func (s *applicationEventReporter) shouldSendResourceEvent(a *appv1.Application, rs appv1.ResourceStatus) bool {
	logCtx := logWithResourceStatus(log.WithFields(log.Fields{
		"app":      a.Name,
		"gvk":      fmt.Sprintf("%s/%s/%s", rs.Group, rs.Version, rs.Kind),
		"resource": fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
	}), rs)

	cachedRes, err := s.cache.GetLastResourceEvent(a, rs, getApplicationLatestRevision(a))
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

func getParentAppName(a *appv1.Application, appInstanceLabelKey string, trackingMethod appv1.TrackingMethod) string {
	resourceTracking := argo.NewResourceTracking()
	unApp := kube.MustToUnstructured(&a)

	return resourceTracking.GetAppName(unApp, appInstanceLabelKey, trackingMethod)
}

func isChildApp(parentAppName string) bool {
	return parentAppName != ""
}

func getAppAsResource(a *appv1.Application) *appv1.ResourceStatus {
	return &appv1.ResourceStatus{
		Name:            a.Name,
		Namespace:       a.Namespace,
		Version:         "v1alpha1",
		Kind:            "Application",
		Group:           "argoproj.io",
		Status:          a.Status.Sync.Status,
		Health:          &a.Status.Health,
		RequiresPruning: a.DeletionTimestamp != nil,
	}
}

func (r *applicationEventReporter) getDesiredManifests(ctx context.Context, a *appv1.Application, logCtx *log.Entry) (*apiclient.ManifestResponse, error, bool) {
	// get the desired state manifests of the application
	desiredManifests, err := r.applicationServiceClient.GetManifests(ctx, &application.ApplicationManifestQuery{
		Name:     &a.Name,
		Revision: &a.Status.Sync.Revision,
	})
	if err != nil {
		notManifestGenerationError := !strings.Contains(err.Error(), "Manifest generation error")

		// we can ignore the error
		notAppPathDoesntExistsError := !strings.Contains(err.Error(), "app path does not exist")

		wrongDestinationError := !strings.Contains(err.Error(), "error validating destination")

		// when application deleted rbac also throws erorr with PermissionDenied
		// we can ignore the error, as we check rbac access before reporting events
		notPermissionDeniedError := !strings.Contains(err.Error(), "PermissionDenied")

		if notManifestGenerationError && notPermissionDeniedError && notAppPathDoesntExistsError && wrongDestinationError {
			return nil, fmt.Errorf("failed to get application desired state manifests: %w", err), false
		}
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

	appTree, err := s.applicationServiceClient.ResourceTree(ctx, &application.ResourcesQuery{
		ApplicationName: &a.Name,
		Project:         &a.Spec.Project,
		Namespace:       &a.Namespace,
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

	parentAppName := getParentAppName(a, appInstanceLabelKey, trackingMethod)

	if isChildApp(parentAppName) {
		logCtx.Info("processing as child application")
		parentApplicationEntity, err := s.applicationServiceClient.Get(ctx, &application.ApplicationQuery{
			Name: &parentAppName,
		})
		if err != nil {
			return fmt.Errorf("failed to get parent application entity: %w", err)
		}

		rs := getAppAsResource(a)

		parentDesiredManifests, err, manifestGenErr := s.getDesiredManifests(ctx, parentApplicationEntity, logCtx)
		if err != nil {
			logCtx.WithError(err).Warn("failed to get parent application's desired manifests, resuming")
		}

		// helm app hasnt revision
		// TODO: add check if it helm application
		parentOperationRevision := getOperationRevision(parentApplicationEntity)
		parentRevisionMetadata, err := s.getApplicationRevisionDetails(ctx, parentApplicationEntity, parentOperationRevision)
		if err != nil {
			logCtx.WithError(err).Warn("failed to get parent application's revision metadata, resuming")
		}

		err = s.processResource(ctx, *rs, parentApplicationEntity, logCtx, ts, parentDesiredManifests, appTree, manifestGenErr, a, parentRevisionMetadata, true, appInstanceLabelKey, trackingMethod, desiredManifests.ApplicationVersions)
		if err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricChildAppEventType, metrics.MetricEventUnknownErrorType, a.Name)
			return err
		}
		reconcileDuration := time.Since(startTime)
		s.metricsServer.ObserveEventProcessingDurationHistogramDuration(metrics.MetricChildAppEventType, reconcileDuration)
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

		logWithAppStatus(a, logCtx, ts).Info("sending root application event")
		if err := s.codefreshClient.Send(ctx, a.Name, appEvent); err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricParentAppEventType, metrics.MetricEventDeliveryErrorType, a.Name)
			return fmt.Errorf("failed to send event for root application %s/%s: %w", a.Namespace, a.Name, err)
		}
		reconcileDuration := time.Since(startTime)
		s.metricsServer.ObserveEventProcessingDurationHistogramDuration(metrics.MetricParentAppEventType, reconcileDuration)
	}

	revisionMetadata, _ := s.getApplicationRevisionDetails(ctx, a, getOperationRevision(a))
	// for each resource in the application get desired and actual state,
	// then stream the event
	for _, rs := range a.Status.Resources {
		if isApp(rs) {
			continue
		}
		startTime := time.Now()
		err := s.processResource(ctx, rs, a, logCtx, ts, desiredManifests, appTree, manifestGenErr, nil, revisionMetadata, ignoreResourceCache, appInstanceLabelKey, trackingMethod, nil)
		if err != nil {
			s.metricsServer.IncErroredEventsCounter(metrics.MetricResourceEventType, metrics.MetricEventUnknownErrorType, a.Name)
			return err
		}
		reconcileDuration := time.Since(startTime)
		s.metricsServer.ObserveEventProcessingDurationHistogramDuration(metrics.MetricResourceEventType, reconcileDuration)
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

	revisionMetadataToReport, err := s.getApplicationRevisionDetails(ctx, latestAppStatus, getOperationRevision(latestAppStatus))

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
	ignoreResourceCache bool,
	appInstanceLabelKey string,
	trackingMethod appv1.TrackingMethod,
	applicationVersions *apiclient.ApplicationVersions,
) error {
	metricsEventType := metrics.MetricResourceEventType
	if isApp(rs) {
		metricsEventType = metrics.MetricChildAppEventType
	}

	logCtx = logCtx.WithFields(log.Fields{
		"gvk":      fmt.Sprintf("%s/%s/%s", rs.Group, rs.Version, rs.Kind),
		"resource": fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
	})

	if rs.Health == nil && rs.Status == appv1.SyncStatusCodeSynced {
		// for resources without health status we need to add 'Healthy' status
		// when they are synced because we might have sent an event with 'Missing'
		// status earlier and they would be stuck in it if we don't switch to 'Healthy'
		rs.Health = &appv1.HealthStatus{
			Status: health.HealthStatusHealthy,
		}
	}

	if !ignoreResourceCache && !s.shouldSendResourceEvent(parentApplication, rs) {
		s.metricsServer.IncCachedIgnoredEventsCounter(metricsEventType, parentApplication.Name)
		return nil
	}

	// get resource desired state
	desiredState := getResourceDesiredState(&rs, desiredManifests, logCtx)

	// get resource actual state
	actualState, err := s.applicationServiceClient.GetResource(ctx, &application.ApplicationResourceRequest{
		Name:         &parentApplication.Name,
		Namespace:    &rs.Namespace,
		ResourceName: &rs.Name,
		Version:      &rs.Version,
		Group:        &rs.Group,
		Kind:         &rs.Kind,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			// only return error if there is no point in trying to send the
			// next resource. For example if the shared context has exceeded
			// its deadline
			if strings.Contains(err.Error(), "context deadline exceeded") {
				return fmt.Errorf("failed to get actual state: %w", err)
			}

			s.metricsServer.IncErroredEventsCounter(metricsEventType, metrics.MetricEventUnknownErrorType, parentApplication.Name)
			logCtx.WithError(err).Warn("failed to get actual state, resuming")
			return nil
		}

		manifest := ""
		// empty actual state
		actualState = &application.ApplicationResourceResponse{Manifest: &manifest}
	}

	parentApplicationToReport, revisionMetadataToReport := s.getAppForResourceReporting(rs, ctx, parentApplication, revisionMetadata)

	var originalAppRevisionMetadata *appv1.RevisionMetadata = nil

	if originalApplication != nil {
		originalAppRevisionMetadata, _ = s.getApplicationRevisionDetails(ctx, originalApplication, getOperationRevision(originalApplication))
	}

	ev, err := getResourceEventPayload(parentApplicationToReport, &rs, actualState, desiredState, appTree, manifestGenErr, ts, originalApplication, revisionMetadataToReport, originalAppRevisionMetadata, appInstanceLabelKey, trackingMethod, applicationVersions)
	if err != nil {
		s.metricsServer.IncErroredEventsCounter(metricsEventType, metrics.MetricEventGetPayloadErrorType, parentApplication.Name)
		logCtx.WithError(err).Warn("failed to get event payload, resuming")
		return nil
	}

	appRes := appv1.Application{}
	appName := ""
	if isApp(rs) && actualState.Manifest != nil && json.Unmarshal([]byte(*actualState.Manifest), &appRes) == nil {
		logWithAppStatus(&appRes, logCtx, ts).Info("streaming resource event")
		appName = appRes.Name
	} else {
		logWithResourceStatus(logCtx, rs).Info("streaming resource event")
		appName = rs.Name
	}

	if err := s.codefreshClient.Send(ctx, appName, ev); err != nil {
		if strings.Contains(err.Error(), "context deadline exceeded") {
			return fmt.Errorf("failed to send resource event: %w", err)
		}

		s.metricsServer.IncErroredEventsCounter(metricsEventType, metrics.MetricEventDeliveryErrorType, appName)
		logCtx.WithError(err).Warn("failed to send resource event, resuming")
		return nil
	}

	if err := s.cache.SetLastResourceEvent(parentApplicationToReport, rs, resourceEventCacheExpiration, getApplicationLatestRevision(parentApplicationToReport)); err != nil {
		logCtx.WithError(err).Warn("failed to cache resource event")
	}

	return nil
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
	cachedApp.Spec.Project = ae.Application.Spec.Project               //
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

	return false, false
}

func isApp(rs appv1.ResourceStatus) bool {
	return rs.GroupVersionKind().String() == appv1.ApplicationSchemaGroupVersionKind.String()
}

func logWithAppStatus(a *appv1.Application, logCtx *log.Entry, ts string) *log.Entry {
	return logCtx.WithFields(log.Fields{
		"sync":            a.Status.Sync.Status,
		"health":          a.Status.Health.Status,
		"resourceVersion": a.ResourceVersion,
		"ts":              ts,
	})
}

func logWithResourceStatus(logCtx *log.Entry, rs appv1.ResourceStatus) *log.Entry {
	logCtx = logCtx.WithField("sync", rs.Status)
	if rs.Health != nil {
		logCtx = logCtx.WithField("health", rs.Health.Status)
	}

	return logCtx
}

func getLatestAppHistoryItem(a *appv1.Application) *appv1.RevisionHistory {
	if a.Status.History != nil && len(a.Status.History) > 0 {
		return &a.Status.History[len(a.Status.History)-1]
	}

	return nil
}

func getApplicationLatestRevision(a *appv1.Application) string {
	revision := a.Status.Sync.Revision
	lastHistory := getLatestAppHistoryItem(a)

	if lastHistory != nil {
		revision = lastHistory.Revision
	}

	return revision
}

func getOperationRevision(a *appv1.Application) string {
	var revision string
	if a != nil {
		// this value will be used in case if application hasnt resources , like gitsource
		revision = a.Status.Sync.Revision
		if a.Status.OperationState != nil && a.Status.OperationState.Operation.Sync != nil && a.Status.OperationState.Operation.Sync.Revision != "" {
			revision = a.Status.OperationState.Operation.Sync.Revision
		} else if a.Operation != nil && a.Operation.Sync != nil && a.Operation.Sync.Revision != "" {
			revision = a.Operation.Sync.Revision
		}
	}

	return revision
}

func (s *applicationEventReporter) getApplicationRevisionDetails(ctx context.Context, a *appv1.Application, revision string) (*appv1.RevisionMetadata, error) {
	return s.applicationServiceClient.RevisionMetadata(ctx, &application.RevisionMetadataQuery{
		Name:     &a.Name,
		Revision: &revision,
	})
}

func getLatestAppHistoryId(a *appv1.Application) int64 {
	var id int64
	lastHistory := getLatestAppHistoryItem(a)

	if lastHistory != nil {
		id = lastHistory.ID
	}

	return id
}

func getResourceEventPayload(
	parentApplication *appv1.Application,
	rs *appv1.ResourceStatus,
	actualState *application.ApplicationResourceResponse,
	desiredState *apiclient.Manifest,
	apptree *appv1.ApplicationTree,
	manifestGenErr bool,
	ts string,
	originalApplication *appv1.Application, // passed when rs is application
	revisionMetadata *appv1.RevisionMetadata,
	originalAppRevisionMetadata *appv1.RevisionMetadata, // passed when rs is application
	appInstanceLabelKey string,
	trackingMethod appv1.TrackingMethod,
	applicationVersions *apiclient.ApplicationVersions,
) (*events.Event, error) {
	var (
		err          error
		syncStarted  = metav1.Now()
		syncFinished *metav1.Time
		errors       = []*events.ObjectError{}
		logCtx       *log.Entry
	)

	if originalApplication != nil {
		logCtx = log.WithField("application", originalApplication.Name)
	} else {
		logCtx = log.NewEntry(log.StandardLogger())
	}

	object := []byte(*actualState.Manifest)

	if originalAppRevisionMetadata != nil && len(object) != 0 {
		actualObject, err := appv1.UnmarshalToUnstructured(*actualState.Manifest)

		if err == nil {
			actualObject = addCommitDetailsToLabels(actualObject, originalAppRevisionMetadata)
			object, err = actualObject.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal unstructured object: %w", err)
			}
		}
	}
	if len(object) == 0 {
		if len(desiredState.CompiledManifest) == 0 {
			// no actual or desired state, don't send event
			u := &unstructured.Unstructured{}
			apiVersion := rs.Version
			if rs.Group != "" {
				apiVersion = rs.Group + "/" + rs.Version
			}

			u.SetAPIVersion(apiVersion)
			u.SetKind(rs.Kind)
			u.SetName(rs.Name)
			u.SetNamespace(rs.Namespace)
			if originalAppRevisionMetadata != nil {
				u = addCommitDetailsToLabels(u, originalAppRevisionMetadata)
			}

			object, err = u.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal unstructured object: %w", err)
			}
		} else {
			// no actual state, use desired state as event object
			unstructuredWithNamespace, err := addDestNamespaceToManifest([]byte(desiredState.CompiledManifest), rs)
			if err != nil {
				return nil, fmt.Errorf("failed to add destination namespace to manifest: %w", err)
			}
			if originalAppRevisionMetadata != nil {
				unstructuredWithNamespace = addCommitDetailsToLabels(unstructuredWithNamespace, originalAppRevisionMetadata)
			}

			object, _ = unstructuredWithNamespace.MarshalJSON()
		}
	} else if rs.RequiresPruning && !manifestGenErr {
		// resource should be deleted
		desiredState.CompiledManifest = ""
		manifest := ""
		actualState.Manifest = &manifest
	}

	if (originalApplication != nil && originalApplication.DeletionTimestamp != nil) || parentApplication.ObjectMeta.DeletionTimestamp != nil {
		// resource should be deleted in case if application in process of deletion
		desiredState.CompiledManifest = ""
		manifest := ""
		actualState.Manifest = &manifest
	}

	if parentApplication.Status.OperationState != nil {
		syncStarted = parentApplication.Status.OperationState.StartedAt
		syncFinished = parentApplication.Status.OperationState.FinishedAt
		errors = append(errors, parseResourceSyncResultErrors(rs, parentApplication.Status.OperationState)...)
	}

	// for primitive resources that are synced right away and don't require progression time (like configmap)
	if rs.Status == appv1.SyncStatusCodeSynced && rs.Health != nil && rs.Health.Status == health.HealthStatusHealthy {
		syncFinished = &syncStarted
	}

	// parent application not include errors in application originally was created with broken state, for example in destination missed namespace
	if originalApplication != nil && originalApplication.Status.OperationState != nil {
		errors = append(errors, parseApplicationSyncResultErrors(originalApplication.Status.OperationState)...)
	}

	if originalApplication != nil && originalApplication.Status.Conditions != nil {
		errors = append(errors, parseApplicationSyncResultErrorsFromConditions(originalApplication.Status)...)
	}

	if len(desiredState.RawManifest) == 0 && len(desiredState.CompiledManifest) != 0 {
		// for handling helm defined resources, etc...
		y, err := yaml.JSONToYAML([]byte(desiredState.CompiledManifest))
		if err == nil {
			desiredState.RawManifest = string(y)
		}
	}

	applicationVersionsEvents, err := repoAppVersionsToEvent(applicationVersions)
	if err != nil {
		logCtx.Errorf("failed to convert appVersions: %v", err)
	}

	source := events.ObjectSource{
		DesiredManifest:       desiredState.CompiledManifest,
		ActualManifest:        *actualState.Manifest,
		GitManifest:           desiredState.RawManifest,
		RepoURL:               parentApplication.Status.Sync.ComparedTo.Source.RepoURL,
		Path:                  desiredState.Path,
		Revision:              getApplicationLatestRevision(parentApplication),
		OperationSyncRevision: getOperationRevision(parentApplication),
		HistoryId:             getLatestAppHistoryId(parentApplication),
		AppName:               parentApplication.Name,
		AppNamespace:          parentApplication.Namespace,
		AppUID:                string(parentApplication.ObjectMeta.UID),
		AppLabels:             parentApplication.Labels,
		SyncStatus:            string(rs.Status),
		SyncStartedAt:         syncStarted,
		SyncFinishedAt:        syncFinished,
		Cluster:               parentApplication.Spec.Destination.Server,
		AppInstanceLabelKey:   appInstanceLabelKey,
		TrackingMethod:        string(trackingMethod),
	}

	if revisionMetadata != nil {
		source.CommitMessage = revisionMetadata.Message
		source.CommitAuthor = revisionMetadata.Author
		source.CommitDate = &revisionMetadata.Date
	}

	if rs.Health != nil {
		source.HealthStatus = (*string)(&rs.Health.Status)
		source.HealthMessage = &rs.Health.Message
		if rs.Health.Status != health.HealthStatusHealthy {
			errors = append(errors, parseAggregativeHealthErrors(rs, apptree)...)
		}
	}

	payload := events.EventPayload{
		Timestamp:   ts,
		Object:      object,
		Source:      &source,
		Errors:      errors,
		AppVersions: applicationVersionsEvents,
	}

	logCtx.Infof("AppVersion before encoding: %v", safeString(payload.AppVersions.AppVersion))

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", rs.Namespace, rs.Name, err)
	}

	return &events.Event{Payload: payloadBytes}, nil
}

func (s *applicationEventReporter) getApplicationEventPayload(
	ctx context.Context,
	a *appv1.Application,
	ts string,
	appInstanceLabelKey string,
	trackingMethod appv1.TrackingMethod,
	applicationVersions *apiclient.ApplicationVersions,
) (*events.Event, error) {
	var (
		syncStarted  = metav1.Now()
		syncFinished *metav1.Time
		logCtx       = log.WithField("application", a.Name)
	)

	obj := appv1.Application{}
	a.DeepCopyInto(&obj)

	// make sure there is type meta on object
	obj.TypeMeta = metav1.TypeMeta{
		Kind:       appv1reg.ApplicationKind,
		APIVersion: appv1.SchemeGroupVersion.String(),
	}

	if a.Status.OperationState != nil {
		syncStarted = a.Status.OperationState.StartedAt
		syncFinished = a.Status.OperationState.FinishedAt
	}

	applicationSource := a.Spec.GetSource()
	if !applicationSource.IsHelm() && (a.Status.Sync.Revision != "" || (a.Status.History != nil && len(a.Status.History) > 0)) {
		revisionMetadata, err := s.getApplicationRevisionDetails(ctx, a, getOperationRevision(a))

		if err != nil {
			if !strings.Contains(err.Error(), "not found") {
				return nil, fmt.Errorf("failed to get revision metadata: %w", err)
			}

			logCtx.Warnf("failed to get revision metadata: %s, reporting application deletion event", err.Error())
		} else {
			if obj.ObjectMeta.Labels == nil {
				obj.ObjectMeta.Labels = map[string]string{}
			}

			obj.ObjectMeta.Labels["app.meta.commit-date"] = revisionMetadata.Date.Format("2006-01-02T15:04:05.000Z")
			obj.ObjectMeta.Labels["app.meta.commit-author"] = revisionMetadata.Author
			obj.ObjectMeta.Labels["app.meta.commit-message"] = revisionMetadata.Message
		}
	}

	object, err := json.Marshal(&obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal application event")
	}

	actualManifest := string(object)
	if a.DeletionTimestamp != nil {
		actualManifest = "" // mark as deleted
		logCtx.Info("reporting application deletion event")
	}

	applicationVersionsEvents, err := repoAppVersionsToEvent(applicationVersions)
	if err != nil {
		logCtx.Errorf("failed to convert appVersions: %v", err)
	}

	hs := string(a.Status.Health.Status)
	source := &events.ObjectSource{
		DesiredManifest:       "",
		GitManifest:           "",
		ActualManifest:        actualManifest,
		RepoURL:               a.Spec.GetSource().RepoURL,
		CommitMessage:         "",
		CommitAuthor:          "",
		Path:                  "",
		Revision:              "",
		OperationSyncRevision: "",
		HistoryId:             0,
		AppName:               "",
		AppUID:                "",
		AppLabels:             map[string]string{},
		SyncStatus:            string(a.Status.Sync.Status),
		SyncStartedAt:         syncStarted,
		SyncFinishedAt:        syncFinished,
		HealthStatus:          &hs,
		HealthMessage:         &a.Status.Health.Message,
		Cluster:               a.Spec.Destination.Server,
		AppInstanceLabelKey:   appInstanceLabelKey,
		TrackingMethod:        string(trackingMethod),
	}

	payload := events.EventPayload{
		Timestamp:   ts,
		Object:      object,
		Source:      source,
		Errors:      parseApplicationSyncResultErrorsFromConditions(a.Status),
		AppVersions: applicationVersionsEvents,
	}

	logCtx.Infof("AppVersion before encoding: %v", safeString(payload.AppVersions.AppVersion))

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", a.Namespace, a.Name, err)
	}

	return &events.Event{Payload: payloadBytes}, nil
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

func addDestNamespaceToManifest(resourceManifest []byte, rs *appv1.ResourceStatus) (*unstructured.Unstructured, error) {
	u, err := appv1.UnmarshalToUnstructured(string(resourceManifest))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	if u.GetNamespace() == rs.Namespace {
		return u, nil
	}

	// need to change namespace
	u.SetNamespace(rs.Namespace)

	return u, nil
}

func addCommitDetailsToLabels(u *unstructured.Unstructured, revisionMetadata *appv1.RevisionMetadata) *unstructured.Unstructured {
	if revisionMetadata == nil || u == nil {
		return u
	}

	if field, _, _ := unstructured.NestedFieldCopy(u.Object, "metadata", "labels"); field == nil {
		_ = unstructured.SetNestedStringMap(u.Object, map[string]string{}, "metadata", "labels")
	}

	_ = unstructured.SetNestedField(u.Object, revisionMetadata.Date.Format("2006-01-02T15:04:05.000Z"), "metadata", "labels", "app.meta.commit-date")
	_ = unstructured.SetNestedField(u.Object, revisionMetadata.Author, "metadata", "labels", "app.meta.commit-author")
	_ = unstructured.SetNestedField(u.Object, revisionMetadata.Message, "metadata", "labels", "app.meta.commit-message")

	return u
}

func repoAppVersionsToEvent(applicationVersions *apiclient.ApplicationVersions) (*events.ApplicationVersions, error) {
	applicationVersionsEvents := &events.ApplicationVersions{}
	applicationVersionsData, _ := json.Marshal(applicationVersions)
	err := json.Unmarshal(applicationVersionsData, applicationVersionsEvents)
	if err != nil {
		return nil, err
	}
	return applicationVersionsEvents, nil
}

func safeString(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
