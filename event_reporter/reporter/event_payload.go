package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/argoproj/argo-cd/v2/event_reporter/utils"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"

	"github.com/argoproj/gitops-engine/pkg/health"
	log "github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

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
			actualObject = utils.AddCommitDetailsToLabels(actualObject, originalAppRevisionMetadata)
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
				u = utils.AddCommitDetailsToLabels(u, originalAppRevisionMetadata)
			}

			object, err = u.MarshalJSON()
			if err != nil {
				return nil, fmt.Errorf("failed to marshal unstructured object: %w", err)
			}
		} else {
			// no actual state, use desired state as event object
			unstructuredWithNamespace, err := utils.AddDestNamespaceToManifest([]byte(desiredState.CompiledManifest), rs)
			if err != nil {
				return nil, fmt.Errorf("failed to add destination namespace to manifest: %w", err)
			}
			if originalAppRevisionMetadata != nil {
				unstructuredWithNamespace = utils.AddCommitDetailsToLabels(unstructuredWithNamespace, originalAppRevisionMetadata)
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

	applicationVersionsEvents, err := utils.RepoAppVersionsToEvent(applicationVersions)
	if err != nil {
		logCtx.Errorf("failed to convert appVersions: %v", err)
	}

	source := events.ObjectSource{
		DesiredManifest:       desiredState.CompiledManifest,
		ActualManifest:        *actualState.Manifest,
		GitManifest:           desiredState.RawManifest,
		RepoURL:               parentApplication.Status.Sync.ComparedTo.Source.RepoURL,
		Path:                  desiredState.Path,
		Revision:              utils.GetApplicationLatestRevision(parentApplication),
		OperationSyncRevision: utils.GetOperationRevision(parentApplication),
		HistoryId:             utils.GetLatestAppHistoryId(parentApplication),
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

	logCtx.Infof("AppVersion before encoding: %v", utils.SafeString(payload.AppVersions.AppVersion))

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
	obj.SetDefaultTypeMeta()

	if a.Status.OperationState != nil {
		syncStarted = a.Status.OperationState.StartedAt
		syncFinished = a.Status.OperationState.FinishedAt
	}

	applicationSource := a.Spec.GetSource()
	if !applicationSource.IsHelm() && (a.Status.Sync.Revision != "" || (a.Status.History != nil && len(a.Status.History) > 0)) {
		revisionMetadata, err := s.getApplicationRevisionDetails(ctx, a, utils.GetOperationRevision(a))

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

	applicationVersionsEvents, err := utils.RepoAppVersionsToEvent(applicationVersions)
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

	logCtx.Infof("AppVersion before encoding: %v", utils.SafeString(payload.AppVersions.AppVersion))

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", a.Namespace, a.Name, err)
	}

	return &events.Event{Payload: payloadBytes}, nil
}
