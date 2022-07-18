package application

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/argoproj/argo-cd/v2/common"

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

type applicationEventReporter struct {
	server *Server
}

func NewApplicationEventReporter(server *Server) *applicationEventReporter {
	return &applicationEventReporter{server}
}

func (s *applicationEventReporter) shouldSendResourceEvent(a *appv1.Application, rs appv1.ResourceStatus) bool {
	logCtx := logWithResourceStatus(log.WithFields(log.Fields{
		"app":      a.Name,
		"gvk":      fmt.Sprintf("%s/%s/%s", rs.Group, rs.Version, rs.Kind),
		"resource": fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
	}), rs)

	cachedRes, err := s.server.cache.GetLastResourceEvent(a, rs, getApplicationLatestRevision(a))
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

func isChildApp(a *appv1.Application) bool {
	if a.Labels != nil {
		return a.Labels[common.LabelKeyAppInstance] != ""
	}
	return false
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

func (s *applicationEventReporter) getDesiredManifests(ctx context.Context, a *appv1.Application, logCtx *log.Entry) (*apiclient.ManifestResponse, error, bool) {
	// get the desired state manifests of the application
	desiredManifests, err := s.server.GetManifests(ctx, &application.ApplicationManifestQuery{
		Name:     &a.Name,
		Revision: a.Status.Sync.Revision,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "Manifest generation error") {
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

func (s *applicationEventReporter) streamApplicationEvents(
	ctx context.Context,
	a *appv1.Application,
	es *events.EventSource,
	stream events.Eventing_StartEventSourceServer,
	ts string,
) error {
	var (
		logCtx = log.WithField("app", a.Name)
	)

	logCtx.Info("streaming application events")

	appTree, err := s.server.getAppResources(ctx, a)
	if err != nil {
		// we still need process app even without tree, it is in case if app yaml originally contain error,
		// we still want show it on codefresh ui and erors that related to it
		logCtx.WithError(err).Error("failed to get application tree")
	}

	if isChildApp(a) {
		parentApp := a.Labels[common.LabelKeyAppInstance]

		parentApplicationEntity, err := s.server.Get(ctx, &application.ApplicationQuery{
			Name: &parentApp,
		})
		if err != nil {
			return fmt.Errorf("failed to get application event: %w", err)
		}

		rs := getAppAsResource(a)

		desiredManifests, _, manifestGenErr := s.getDesiredManifests(ctx, parentApplicationEntity, logCtx)

		revisionMetadata, _ := s.getApplicationHistoryRevisionDetails(ctx, a)

		s.processResource(ctx, *rs, parentApplicationEntity, logCtx, ts, desiredManifests, stream, appTree, es, manifestGenErr, a, revisionMetadata)
	} else {
		// application events for child apps would be sent by its parent app
		// as resource event
		appEvent, err := s.getApplicationEventPayload(ctx, a, es, ts)
		if err != nil {
			return fmt.Errorf("failed to get application event: %w", err)
		}

		if appEvent == nil {
			// event did not have an OperationState - skip all events
			return nil
		}

		logWithAppStatus(a, logCtx, ts).Info("sending application event")
		if err := stream.Send(appEvent); err != nil {
			return fmt.Errorf("failed to send event for resource %s/%s: %w", a.Namespace, a.Name, err)
		}
	}

	desiredManifests, err, manifestGenErr := s.getDesiredManifests(ctx, a, logCtx)
	if err != nil {
		return err
	}

	revisionMetadata, _ := s.getApplicationHistoryRevisionDetails(ctx, a)
	// for each resource in the application get desired and actual state,
	// then stream the event
	for _, rs := range a.Status.Resources {
		if isApp(rs) {
			continue
		}
		s.processResource(ctx, rs, a, logCtx, ts, desiredManifests, stream, appTree, es, manifestGenErr, nil, revisionMetadata)
	}
	return nil
}

func (s *applicationEventReporter) processResource(
	ctx context.Context,
	rs appv1.ResourceStatus,
	parentApplication *appv1.Application,
	logCtx *log.Entry,
	ts string,
	desiredManifests *apiclient.ManifestResponse,
	stream events.Eventing_StartEventSourceServer,
	appTree *appv1.ApplicationTree,
	es *events.EventSource,
	manifestGenErr bool,
	originalApplication *appv1.Application,
	revisionMetadata *appv1.RevisionMetadata,
) {
	logCtx = logCtx.WithFields(log.Fields{
		"gvk":      fmt.Sprintf("%s/%s/%s", rs.Group, rs.Version, rs.Kind),
		"resource": fmt.Sprintf("%s/%s", rs.Namespace, rs.Name),
	})

	if !s.shouldSendResourceEvent(parentApplication, rs) {
		return
	}

	// get resource desired state
	desiredState := getResourceDesiredState(&rs, desiredManifests, logCtx)

	// get resource actual state
	actualState, err := s.server.GetResource(ctx, &application.ApplicationResourceRequest{
		Name:         &parentApplication.Name,
		Namespace:    rs.Namespace,
		ResourceName: rs.Name,
		Version:      rs.Version,
		Group:        rs.Group,
		Kind:         rs.Kind,
	})
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			logCtx.WithError(err).Error("failed to get actual state")
			return
		}

		// empty actual state
		actualState = &application.ApplicationResourceResponse{Manifest: ""}
	}

	ev, err := getResourceEventPayload(parentApplication, &rs, es, actualState, desiredState, appTree, manifestGenErr, ts, originalApplication, revisionMetadata)
	if err != nil {
		logCtx.WithError(err).Error("failed to get event payload")
		return
	}

	appRes := appv1.Application{}
	if isApp(rs) && actualState.Manifest != "" && json.Unmarshal([]byte(actualState.Manifest), &appRes) == nil {
		logWithAppStatus(&appRes, logCtx, ts).Info("streaming resource event")
	} else {
		logWithResourceStatus(logCtx, rs).Info("streaming resource event")
	}

	if err := stream.Send(ev); err != nil {
		logCtx.WithError(err).Error("failed to send event")
		return
	}

	if err := s.server.cache.SetLastResourceEvent(parentApplication, rs, resourceEventCacheExpiration, getApplicationLatestRevision(parentApplication)); err != nil {
		logCtx.WithError(err).Error("failed to cache resource event")
	}
}

func (s *applicationEventReporter) shouldSendApplicationEvent(ae *appv1.ApplicationWatchEvent) bool {
	logCtx := log.WithField("app", ae.Application.Name)

	if ae.Type == watch.Deleted {
		logCtx.Info("application deleted")
		return true
	}

	cachedApp, err := s.server.cache.GetLastApplicationEvent(&ae.Application)
	if err != nil || cachedApp == nil {
		return true
	}

	cachedApp.Status.ReconciledAt = ae.Application.Status.ReconciledAt // ignore those in the diff
	cachedApp.Spec.Project = ae.Application.Spec.Project               //
	for i := range cachedApp.Status.Conditions {
		cachedApp.Status.Conditions[i].LastTransitionTime = nil
	}
	for i := range ae.Application.Status.Conditions {
		ae.Application.Status.Conditions[i].LastTransitionTime = nil
	}

	if !reflect.DeepEqual(ae.Application.Spec, cachedApp.Spec) {
		logCtx.Info("application spec changed")
		return true
	}

	if !reflect.DeepEqual(ae.Application.Status, cachedApp.Status) {
		logCtx.Info("application status changed")
		return true
	}

	if !reflect.DeepEqual(ae.Application.Operation, cachedApp.Operation) {
		logCtx.Info("application operation changed")
		return true
	}

	return false
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

func (s *applicationEventReporter) getApplicationHistoryRevisionDetails(ctx context.Context, a *appv1.Application) (*appv1.RevisionMetadata, error) {
	revision := getApplicationLatestRevision(a)

	return s.server.RevisionMetadata(ctx, &application.RevisionMetadataQuery{
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
	es *events.EventSource,
	actualState *application.ApplicationResourceResponse,
	desiredState *apiclient.Manifest,
	apptree *appv1.ApplicationTree,
	manifestGenErr bool,
	ts string,
	originalApplication *appv1.Application,
	revisionMetadata *appv1.RevisionMetadata,
) (*events.Event, error) {
	var (
		err          error
		syncStarted  = metav1.Now()
		syncFinished *metav1.Time
		errors       = []*events.ObjectError{}
	)

	object := []byte(actualState.Manifest)
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
			// TODO maybe we need to check if resource is an application, and then set commit details in label as for parent apps ??
			object, err = u.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal unstructured object: %w", err)
			}
		} else {
			// no actual state, use desired state as event object
			manifestWithNamespace, err := addDestNamespaceToManifest([]byte(desiredState.CompiledManifest), rs)
			if err != nil {
				return nil, fmt.Errorf("failed to add destination namespace to manifest: %w", err)
			}

			object = manifestWithNamespace
		}
	} else if rs.RequiresPruning && !manifestGenErr {
		// resource should be deleted
		desiredState.CompiledManifest = ""
		actualState.Manifest = ""
	}

	if (originalApplication != nil && originalApplication.DeletionTimestamp != nil) || parentApplication.ObjectMeta.DeletionTimestamp != nil {
		// resource should be deleted in case if application in process of deletion
		desiredState.CompiledManifest = ""
		actualState.Manifest = ""
	}

	if parentApplication.Status.OperationState != nil {
		syncStarted = parentApplication.Status.OperationState.StartedAt
		syncFinished = parentApplication.Status.OperationState.FinishedAt
		errors = append(errors, parseResourceSyncResultErrors(rs, parentApplication.Status.OperationState)...)
	}

	// parent application not include errors in application originally was created with broken state, for example in destination missed namespace
	if originalApplication != nil && originalApplication.Status.OperationState != nil {
		errors = append(errors, parseApplicationSyncResultErrors(originalApplication.Status.OperationState)...)
	}

	if originalApplication != nil && originalApplication.Status.Conditions != nil {
		errors = append(errors, parseApplicationSyncResultErrorsFromConditions(originalApplication.Status.Conditions)...)
	}

	if len(desiredState.RawManifest) == 0 && len(desiredState.CompiledManifest) != 0 {
		// for handling helm defined resources, etc...
		y, err := yaml.JSONToYAML([]byte(desiredState.CompiledManifest))
		if err == nil {
			desiredState.RawManifest = string(y)
		}
	}

	source := events.ObjectSource{
		DesiredManifest: desiredState.CompiledManifest,
		ActualManifest:  actualState.Manifest,
		GitManifest:     desiredState.RawManifest,
		RepoURL:         parentApplication.Status.Sync.ComparedTo.Source.RepoURL,
		Path:            desiredState.Path,
		Revision:        getApplicationLatestRevision(parentApplication),
		HistoryId:       getLatestAppHistoryId(parentApplication),
		AppName:         parentApplication.Name,
		AppLabels:       parentApplication.Labels,
		SyncStatus:      string(rs.Status),
		SyncStartedAt:   syncStarted,
		SyncFinishedAt:  syncFinished,
		Cluster:         parentApplication.Spec.Destination.Server,
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
		Timestamp: ts,
		Object:    object,
		Source:    &source,
		Errors:    errors,
	}

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", rs.Namespace, rs.Name, err)
	}

	return &events.Event{Payload: payloadBytes, Name: es.Name}, nil
}

func (s *applicationEventReporter) getApplicationEventPayload(ctx context.Context, a *appv1.Application, es *events.EventSource, ts string) (*events.Event, error) {
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

	if a.Status.Sync.Revision != "" || (a.Status.History != nil && len(a.Status.History) > 0) {
		revisionMetadata, err := s.getApplicationHistoryRevisionDetails(ctx, a)

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

	hs := string(a.Status.Health.Status)
	source := &events.ObjectSource{
		DesiredManifest: "",
		GitManifest:     "",
		ActualManifest:  actualManifest,
		RepoURL:         a.Spec.Source.RepoURL,
		CommitMessage:   "",
		CommitAuthor:    "",
		Path:            "",
		Revision:        "",
		HistoryId:       0,
		AppName:         "",
		AppLabels:       map[string]string{},
		SyncStatus:      string(a.Status.Sync.Status),
		SyncStartedAt:   syncStarted,
		SyncFinishedAt:  syncFinished,
		HealthStatus:    &hs,
		HealthMessage:   &a.Status.Health.Message,
		Cluster:         a.Spec.Destination.Server,
	}

	payload := events.EventPayload{
		Timestamp: ts,
		Object:    object,
		Source:    source,
		Errors:    parseApplicationSyncResultErrorsFromConditions(a.Status.Conditions),
	}

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", a.Namespace, a.Name, err)
	}

	return &events.Event{Payload: payloadBytes, Name: es.Name}, nil
}

func getResourceDesiredState(rs *appv1.ResourceStatus, ds *apiclient.ManifestResponse, logger *log.Entry) *apiclient.Manifest {
	for _, m := range ds.Manifests {
		u, err := appv1.UnmarshalToUnstructured(m.CompiledManifest)
		if err != nil {
			logger.WithError(err).Warnf("failed to unmarshal compiled manifest")
			continue
		}

		if u == nil {
			logger.WithError(err).Warnf("no compiled manifest for: %s", m.Path)
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

func addDestNamespaceToManifest(resourceManifest []byte, rs *appv1.ResourceStatus) ([]byte, error) {
	u, err := appv1.UnmarshalToUnstructured(string(resourceManifest))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
	}

	if u.GetNamespace() == rs.Namespace {
		return resourceManifest, nil
	}

	// need to change namespace
	u.SetNamespace(rs.Namespace)

	return u.MarshalJSON()
}
