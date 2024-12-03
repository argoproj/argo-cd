package utils

import appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

type AppRevisionsFieldNames string

var (
	AppRevisionFieldName  AppRevisionsFieldNames = "Revision"
	AppRevisionsFieldName AppRevisionsFieldNames = "Revisions"
)

type AppUtils struct {
	App *appv1.Application
}

func (au *AppUtils) operationStateSyncExists(fieldToCheck *AppRevisionsFieldNames) bool {
	result := au.App != nil && au.App.Status.OperationState != nil && au.App.Status.OperationState.Operation.Sync != nil
	if !result {
		return false
	}

	return revisionsToCheck(RevisionsData{
		Revision:  au.App.Status.OperationState.Operation.Sync.Revision,
		Revisions: au.App.Status.OperationState.Operation.Sync.Revisions,
	}, fieldToCheck)
}

func (au *AppUtils) operationSyncExists(fieldToCheck *AppRevisionsFieldNames) bool {
	result := au.App != nil && au.App.Operation != nil && au.App.Operation.Sync != nil
	if !result {
		return false
	}

	return revisionsToCheck(RevisionsData{
		Revision:  au.App.Operation.Sync.Revision,
		Revisions: au.App.Operation.Sync.Revisions,
	}, fieldToCheck)
}

func (au *AppUtils) operationSyncResultExists(fieldToCheck *AppRevisionsFieldNames) bool {
	result := au.App != nil && au.App.Status.OperationState != nil && au.App.Status.OperationState.SyncResult != nil
	if !result {
		return false
	}

	return revisionsToCheck(RevisionsData{
		Revision:  au.App.Status.OperationState.SyncResult.Revision,
		Revisions: au.App.Status.OperationState.SyncResult.Revisions,
	}, fieldToCheck)
}

// expected to return true if fieldToCheck == nil
func revisionsToCheck(obj RevisionsData, fieldToCheck *AppRevisionsFieldNames) bool {
	if fieldToCheck == nil {
		return true
	}
	if *fieldToCheck == AppRevisionFieldName {
		return obj.Revision != ""
	}

	if *fieldToCheck == AppRevisionsFieldName {
		return obj.Revisions != nil && len(obj.Revisions) > 0
	}
	return true
}
