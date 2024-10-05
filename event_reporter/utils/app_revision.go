package utils

import (
	"encoding/json"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
)

type AppSyncRevisionsMetadata struct {
	SyncRevisions   []*RevisionWithMetadata `json:"syncRevisions" protobuf:"bytes,1,name=syncRevisions"`
	ChangeRevisions []*RevisionWithMetadata `json:"changeRevisions" protobuf:"bytes,2,name=changeRevisions"`
}

type RevisionWithMetadata struct {
	Revision string                  `json:"revision" protobuf:"bytes,1,name=revision"`
	Metadata *appv1.RevisionMetadata `json:"metadata,omitempty" protobuf:"bytes,2,name=metadata"`
}

type RevisionsData struct {
	Revision  string   `json:"revision,omitempty" protobuf:"bytes,1,opt,name=revision"`
	Revisions []string `json:"revisions,omitempty" protobuf:"bytes,2,opt,name=revisions"`
}

const annotationRevisionKey = "app.meta.revisions-metadata"

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

func GetOperationSyncRevisions(a *appv1.Application) []string {
	if a == nil {
		return []string{}
	}

	// this value will be used in case if application hasn't resources, like empty gitsource
	revisions := getRevisions(RevisionsData{
		Revision:  a.Status.Sync.Revision,
		Revisions: a.Status.Sync.Revisions,
	})

	if a.Status.OperationState != nil && a.Status.OperationState.Operation.Sync != nil {
		revisions = getRevisions(RevisionsData{
			Revision:  a.Status.OperationState.Operation.Sync.Revision,
			Revisions: a.Status.OperationState.Operation.Sync.Revisions,
		})
	} else if a.Operation != nil && a.Operation.Sync != nil {
		revisions = getRevisions(RevisionsData{
			Revision:  a.Operation.Sync.Revision,
			Revisions: a.Operation.Sync.Revisions,
		})
	}

	return revisions
}

// for monorepo support: list with revisions where actual changes to source directory were committed
func GetOperationChangeRevisions(a *appv1.Application) []string {
	var revisions []string

	if a == nil {
		return revisions
	}

	// this value will be used in case if application hasn't resources, like empty gitsource
	if a.Status.OperationState != nil && a.Status.OperationState.Operation.Sync != nil {
		if a.Status.OperationState.Operation.Sync.ChangeRevision != "" || a.Status.OperationState.Operation.Sync.ChangeRevisions != nil {
			revisions = getRevisions(RevisionsData{
				Revision:  a.Status.OperationState.Operation.Sync.ChangeRevision,
				Revisions: a.Status.OperationState.Operation.Sync.ChangeRevisions,
			})
		}
	} else if a.Operation != nil && a.Operation.Sync != nil {
		if a.Operation.Sync.ChangeRevision != "" || a.Operation.Sync.ChangeRevisions != nil {
			revisions = getRevisions(RevisionsData{
				Revision:  a.Operation.Sync.ChangeRevision,
				Revisions: a.Operation.Sync.ChangeRevisions,
			})
		}
	}

	return revisions
}

func getRevisions(rd RevisionsData) []string {
	if rd.Revisions != nil {
		return rd.Revisions
	}

	return []string{rd.Revision}
}

func AddCommitsDetailsToAnnotations(unstrApp *unstructured.Unstructured, revisionsMetadata *AppSyncRevisionsMetadata) *unstructured.Unstructured {
	if revisionsMetadata == nil || unstrApp == nil {
		return unstrApp
	}

	if field, _, _ := unstructured.NestedFieldCopy(unstrApp.Object, "metadata", "annotations"); field == nil {
		_ = unstructured.SetNestedStringMap(unstrApp.Object, map[string]string{}, "metadata", "annotations")
	}

	jsonRevisionsMetadata, err := json.Marshal(revisionsMetadata)
	if err != nil {
		return unstrApp
	}

	_ = unstructured.SetNestedField(unstrApp.Object, string(jsonRevisionsMetadata), "metadata", "annotations", annotationRevisionKey)

	return unstrApp
}

func AddCommitsDetailsToAppAnnotations(app appv1.Application, revisionsMetadata *AppSyncRevisionsMetadata) appv1.Application {
	if revisionsMetadata == nil {
		return app
	}

	if app.ObjectMeta.Annotations == nil {
		app.ObjectMeta.Annotations = map[string]string{}
	}

	jsonRevisionsMetadata, err := json.Marshal(revisionsMetadata)
	if err != nil {
		return app
	}

	app.ObjectMeta.Annotations[annotationRevisionKey] = string(jsonRevisionsMetadata)

	return app
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

func AddCommitsDetailsToAppLabels(app *appv1.Application, revisionMetadata *appv1.RevisionMetadata) *appv1.Application {
	if revisionMetadata == nil {
		return app
	}

	if app.ObjectMeta.Labels == nil {
		app.ObjectMeta.Labels = map[string]string{}
	}

	app.ObjectMeta.Labels["app.meta.commit-date"] = revisionMetadata.Date.Format("2006-01-02T15:04:05.000Z")
	app.ObjectMeta.Labels["app.meta.commit-author"] = revisionMetadata.Author
	app.ObjectMeta.Labels["app.meta.commit-message"] = revisionMetadata.Message

	return app
}
