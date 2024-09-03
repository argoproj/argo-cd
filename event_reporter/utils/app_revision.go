package utils

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

func GetLatestAppHistoryId(a *appv1.Application) int64 {
	if lastHistory := getLatestAppHistoryItem(a); lastHistory != nil {
		return lastHistory.ID
	}

	return 0
}

func getLatestAppHistoryItem(a *appv1.Application) *appv1.RevisionHistory {
	if a.Status.History != nil && len(a.Status.History) > 0 {
		return &a.Status.History[len(a.Status.History)-1]
	}

	return nil
}

func GetApplicationLatestRevision(a *appv1.Application) string {
	if lastHistory := getLatestAppHistoryItem(a); lastHistory != nil {
		return lastHistory.Revision
	}

	return a.Status.Sync.Revision
}

func GetOperationRevision(a *appv1.Application) string {
	if a == nil {
		return ""
	}

	// this value will be used in case if application hasn't resources , like gitsource
	revision := a.Status.Sync.Revision
	if a.Status.OperationState != nil && a.Status.OperationState.Operation.Sync != nil && a.Status.OperationState.Operation.Sync.Revision != "" {
		revision = a.Status.OperationState.Operation.Sync.Revision
	} else if a.Operation != nil && a.Operation.Sync != nil && a.Operation.Sync.Revision != "" {
		revision = a.Operation.Sync.Revision
	}

	return revision
}

func AddCommitDetailsToLabels(u *unstructured.Unstructured, revisionMetadata *appv1.RevisionMetadata) *unstructured.Unstructured {
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
