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
	appEventProcessingStartedAt string,
	rr *ReportedResource,
	reportedEntityParentApp *ReportedEntityParentApp,
	argoTrackingMetadata *ArgoTrackingMetadata,
) (*events.Event, error) {
	var (
		err          error
		syncStarted  = metav1.Now()
		syncFinished *metav1.Time
		logCtx       *log.Entry
	)

	if rr.rsAsAppInfo != nil && rr.rsAsAppInfo.app != nil {
		logCtx = log.WithField("application", rr.rsAsAppInfo.app.Name)
	} else {
		logCtx = log.NewEntry(log.StandardLogger())
	}

	object := []byte(*rr.actualState.Manifest)

	if rr.rsAsAppInfo != nil && rr.rsAsAppInfo.revisionsMetadata != nil && len(object) != 0 {
		actualObject, err := appv1.UnmarshalToUnstructured(*rr.actualState.Manifest)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal manifest: %w", err)
		}

		object, err = addCommitDetailsToUnstructured(actualObject, rr)
		if err != nil {
			return nil, err
		}
	}
	if len(object) == 0 {
		if len(rr.desiredState.CompiledManifest) == 0 {
			object, err = buildEventObjectAsLiveAndCompiledManifestsEmpty(rr)
			if err != nil {
				return nil, err
			}
		} else {
			object, err = useCompiledManifestAsEventObject(rr)
			if err != nil {
				return nil, err
			}
		}
	} else if rr.rs.RequiresPruning && !rr.manifestGenErr {
		// resource should be deleted
		makeDesiredAndLiveManifestEmpty(rr.actualState, rr.desiredState)
	}

	if (rr.rsAsAppInfo != nil && rr.rsAsAppInfo.app != nil && rr.rsAsAppInfo.app.DeletionTimestamp != nil) || reportedEntityParentApp.app.ObjectMeta.DeletionTimestamp != nil {
		// resource should be deleted in case if application in process of deletion
		makeDesiredAndLiveManifestEmpty(rr.actualState, rr.desiredState)
	}

	if len(rr.desiredState.RawManifest) == 0 && len(rr.desiredState.CompiledManifest) != 0 {
		// for handling helm defined resources, etc...
		y, err := yaml.JSONToYAML([]byte(rr.desiredState.CompiledManifest))
		if err == nil {
			rr.desiredState.RawManifest = string(y)
		}
	}

	if reportedEntityParentApp.app.Status.OperationState != nil {
		syncStarted = reportedEntityParentApp.app.Status.OperationState.StartedAt
		syncFinished = reportedEntityParentApp.app.Status.OperationState.FinishedAt
	}

	// for primitive resources that are synced right away and don't require progression time (like configmap)
	if rr.rs.Status == appv1.SyncStatusCodeSynced && rr.rs.Health != nil && rr.rs.Health.Status == health.HealthStatusHealthy {
		syncFinished = &syncStarted
	}

	var applicationVersionsEvents *events.ApplicationVersions
	if rr.rsAsAppInfo != nil {
		applicationVersionsEvents, err = utils.RepoAppVersionsToEvent(rr.rsAsAppInfo.applicationVersions)
		if err != nil {
			logCtx.Errorf("failed to convert appVersions: %v", err)
		}
	}

	source := events.ObjectSource{
		DesiredManifest:       rr.desiredState.CompiledManifest,
		ActualManifest:        *rr.actualState.Manifest,
		GitManifest:           rr.desiredState.RawManifest,
		RepoURL:               reportedEntityParentApp.app.Status.Sync.ComparedTo.Source.RepoURL,
		Path:                  rr.desiredState.Path,
		Revision:              utils.GetApplicationLatestRevision(reportedEntityParentApp.app),
		OperationSyncRevision: utils.GetOperationRevision(reportedEntityParentApp.app),
		HistoryId:             utils.GetLatestAppHistoryId(reportedEntityParentApp.app),
		AppName:               reportedEntityParentApp.app.Name,
		AppNamespace:          reportedEntityParentApp.app.Namespace,
		AppUID:                string(reportedEntityParentApp.app.ObjectMeta.UID),
		AppLabels:             reportedEntityParentApp.app.Labels,
		SyncStatus:            string(rr.rs.Status),
		SyncStartedAt:         syncStarted,
		SyncFinishedAt:        syncFinished,
		Cluster:               reportedEntityParentApp.app.Spec.Destination.Server,
		AppInstanceLabelKey:   *argoTrackingMetadata.AppInstanceLabelKey,
		TrackingMethod:        string(*argoTrackingMetadata.TrackingMethod),
	}

	if reportedEntityParentApp.revisionsMetadata != nil && reportedEntityParentApp.revisionsMetadata.SyncRevisions != nil {
		revisionMetadata := getApplicationLegacyRevisionDetails(reportedEntityParentApp.app, reportedEntityParentApp.revisionsMetadata)
		if revisionMetadata != nil {
			source.CommitMessage = revisionMetadata.Message
			source.CommitAuthor = revisionMetadata.Author
			source.CommitDate = &revisionMetadata.Date
		}
	}

	if rr.rs.Health != nil {
		source.HealthStatus = (*string)(&rr.rs.Health.Status)
		source.HealthMessage = &rr.rs.Health.Message
	}

	payload := events.EventPayload{
		Timestamp:   appEventProcessingStartedAt,
		Object:      object,
		Source:      &source,
		Errors:      getResourceEventPayloadErrors(rr, reportedEntityParentApp),
		AppVersions: applicationVersionsEvents,
	}

	if payload.AppVersions != nil {
		logCtx.Infof("AppVersion before encoding: %v", utils.SafeString(payload.AppVersions.AppVersion))
	}

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", rr.rs.Namespace, rr.rs.Name, err)
	}

	return &events.Event{Payload: payloadBytes}, nil
}

func getResourceEventPayloadErrors(
	rr *ReportedResource,
	reportedEntityParentApp *ReportedEntityParentApp,
) []*events.ObjectError {
	var errors []*events.ObjectError

	if reportedEntityParentApp.app.Status.OperationState != nil {
		errors = append(errors, parseResourceSyncResultErrors(rr.rs, reportedEntityParentApp.app.Status.OperationState)...)
	}

	// parent application not include errors in application originally was created with broken state, for example in destination missed namespace
	if rr.rsAsAppInfo != nil && rr.rsAsAppInfo.app != nil {
		if rr.rsAsAppInfo.app.Status.OperationState != nil {
			errors = append(errors, parseApplicationSyncResultErrors(rr.rsAsAppInfo.app.Status.OperationState)...)
		}

		if rr.rsAsAppInfo.app.Status.Conditions != nil {
			errors = append(errors, parseApplicationSyncResultErrorsFromConditions(rr.rsAsAppInfo.app.Status)...)
		}

		errors = append(errors, parseAggregativeHealthErrorsOfApplication(rr.rsAsAppInfo.app, reportedEntityParentApp.appTree)...)
	}

	if rr.rs.Health != nil && rr.rs.Health.Status != health.HealthStatusHealthy {
		errors = append(errors, parseAggregativeHealthErrors(rr.rs, reportedEntityParentApp.appTree, false)...)
	}

	return errors
}

func useCompiledManifestAsEventObject(
	rr *ReportedResource,
) ([]byte, error) {
	// no actual state, use desired state as event object
	unstructuredWithNamespace, err := utils.AddDestNamespaceToManifest([]byte(rr.desiredState.CompiledManifest), rr.rs)
	if err != nil {
		return nil, fmt.Errorf("failed to add destination namespace to manifest: %w", err)
	}

	return addCommitDetailsToUnstructured(unstructuredWithNamespace, rr)
}

func buildEventObjectAsLiveAndCompiledManifestsEmpty(
	rr *ReportedResource,
) ([]byte, error) {
	// no actual or desired state, don't send event
	u := &unstructured.Unstructured{}

	u.SetAPIVersion(rr.GetApiVersion())
	u.SetKind(rr.rs.Kind)
	u.SetName(rr.rs.Name)
	u.SetNamespace(rr.rs.Namespace)

	return addCommitDetailsToUnstructured(u, rr)
}

// when empty minifests reported to codefresh they will get deleted
func makeDesiredAndLiveManifestEmpty(
	actualState *application.ApplicationResourceResponse,
	desiredState *apiclient.Manifest,
) {
	// resource should be deleted
	desiredState.CompiledManifest = ""
	manifest := ""
	actualState.Manifest = &manifest
}

func addCommitDetailsToUnstructured(
	u *unstructured.Unstructured,
	rr *ReportedResource,
) ([]byte, error) {
	if rr.rsAsAppInfo != nil && rr.rsAsAppInfo.revisionsMetadata != nil {
		u = utils.AddCommitsDetailsToAnnotations(u, rr.rsAsAppInfo.revisionsMetadata)
		if rr.rsAsAppInfo.app != nil {
			u = utils.AddCommitDetailsToLabels(u, getApplicationLegacyRevisionDetails(rr.rsAsAppInfo.app, rr.rsAsAppInfo.revisionsMetadata))
		}
	}

	object, err := u.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal unstructured object: %w", err)
	}

	return object, err
}

func (s *applicationEventReporter) getApplicationEventPayload(
	ctx context.Context,
	a *appv1.Application,
	appTree *appv1.ApplicationTree,
	eventProcessingStartedAt string,
	applicationVersions *apiclient.ApplicationVersions,
	argoTrackingMetadata *ArgoTrackingMetadata,
) (*events.Event, error) {
	var (
		syncStarted  = metav1.Now()
		syncFinished *metav1.Time
		logCtx       = log.WithField("application", a.Name)
		errors       = []*events.ObjectError{}
	)

	obj := appv1.Application{}
	a.DeepCopyInto(&obj)

	// make sure there is type meta on object
	obj.SetDefaultTypeMeta()

	if a.Status.OperationState != nil {
		syncStarted = a.Status.OperationState.StartedAt
		syncFinished = a.Status.OperationState.FinishedAt
	}

	revisionsMetadata, err := s.getApplicationRevisionsMetadata(ctx, logCtx, a)
	if err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return nil, fmt.Errorf("failed to get revision metadata: %w", err)
		}

		logCtx.Warnf("failed to get revision metadata: %s, reporting application deletion event", err.Error())
	}

	utils.AddCommitsDetailsToAppAnnotations(obj, revisionsMetadata)
	utils.AddCommitsDetailsToAppLabels(&obj, getApplicationLegacyRevisionDetails(&obj, revisionsMetadata))

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
		AppInstanceLabelKey:   *argoTrackingMetadata.AppInstanceLabelKey,
		TrackingMethod:        string(*argoTrackingMetadata.TrackingMethod),
	}

	errors = append(errors, parseApplicationSyncResultErrorsFromConditions(a.Status)...)
	errors = append(errors, parseAggregativeHealthErrorsOfApplication(a, appTree)...)

	payload := events.EventPayload{
		Timestamp:   eventProcessingStartedAt,
		Object:      object,
		Source:      source,
		Errors:      errors,
		AppVersions: applicationVersionsEvents,
	}

	logCtx.Infof("AppVersion before encoding: %v", utils.SafeString(payload.AppVersions.AppVersion))

	payloadBytes, err := json.Marshal(&payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload for resource %s/%s: %w", a.Namespace, a.Name, err)
	}

	return &events.Event{Payload: payloadBytes}, nil
}
