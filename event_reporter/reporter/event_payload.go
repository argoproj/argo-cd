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
	appEventProcessingStartedAt string,
	originalApplication *appv1.Application, // passed when rs is application
	revisionsMetadata *utils.AppSyncRevisionsMetadata,
	originalAppRevisionsMetadata *utils.AppSyncRevisionsMetadata, // passed when rs is application
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

	if originalAppRevisionsMetadata != nil && len(object) != 0 {
		actualObject, err := appv1.UnmarshalToUnstructured(*actualState.Manifest)

		if err == nil {
			actualObject = utils.AddCommitsDetailsToAnnotations(actualObject, originalAppRevisionsMetadata)
			if originalApplication != nil {
				actualObject = utils.AddCommitDetailsToLabels(actualObject, getApplicationLegacyRevisionDetails(originalApplication, originalAppRevisionsMetadata))
			}

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
			if originalAppRevisionsMetadata != nil {
				u = utils.AddCommitsDetailsToAnnotations(u, originalAppRevisionsMetadata)
				if originalApplication != nil {
					u = utils.AddCommitDetailsToLabels(u, getApplicationLegacyRevisionDetails(originalApplication, originalAppRevisionsMetadata))
				}
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
			if originalAppRevisionsMetadata != nil {
				unstructuredWithNamespace = utils.AddCommitsDetailsToAnnotations(unstructuredWithNamespace, originalAppRevisionsMetadata)
				if originalApplication != nil {
					unstructuredWithNamespace = utils.AddCommitDetailsToLabels(unstructuredWithNamespace, getApplicationLegacyRevisionDetails(originalApplication, originalAppRevisionsMetadata))
				}
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

	if originalApplication != nil {
		errors = append(errors, parseAggregativeHealthErrorsOfApplication(originalApplication, apptree)...)
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

	if revisionsMetadata != nil && revisionsMetadata.SyncRevisions != nil {
		revisionMetadata := getApplicationLegacyRevisionDetails(parentApplication, revisionsMetadata)
		if revisionMetadata != nil {
			source.CommitMessage = revisionMetadata.Message
			source.CommitAuthor = revisionMetadata.Author
			source.CommitDate = &revisionMetadata.Date
		}
	}

	if rs.Health != nil {
		source.HealthStatus = (*string)(&rs.Health.Status)
		source.HealthMessage = &rs.Health.Message
		if rs.Health.Status != health.HealthStatusHealthy {
			errors = append(errors, parseAggregativeHealthErrors(rs, apptree, false)...)
		}
	}

	payload := events.EventPayload{
		Timestamp:   appEventProcessingStartedAt,
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
	appTree *appv1.ApplicationTree,
	eventProcessingStartedAt string,
	appInstanceLabelKey string,
	trackingMethod appv1.TrackingMethod,
	applicationVersions *apiclient.ApplicationVersions,
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
		AppInstanceLabelKey:   appInstanceLabelKey,
		TrackingMethod:        string(trackingMethod),
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
