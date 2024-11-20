package utils

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestGetLatestAppHistoryId(t *testing.T) {
	history1Id := int64(1)
	history2Id := int64(2)

	t.Run("resource revision should be 0", func(t *testing.T) {
		noStatusHistoryAppMock := v1alpha1.Application{}

		idResult := GetLatestAppHistoryId(&noStatusHistoryAppMock)
		assert.Equal(t, int64(0), idResult)

		emptyStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				History: []v1alpha1.RevisionHistory{},
			},
		}

		id2Result := GetLatestAppHistoryId(&emptyStatusHistoryAppMock)
		assert.Equal(t, int64(0), id2Result)
	})

	t.Run("resource revision should be taken from latest history.Id", func(t *testing.T) {
		appMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				History: []v1alpha1.RevisionHistory{
					{
						ID: history1Id,
					},
					{
						ID: history2Id,
					},
				},
			},
		}

		revisionResult := GetLatestAppHistoryId(&appMock)
		assert.Equal(t, revisionResult, history2Id)
	})
}

func TestGetApplicationLatestRevision(t *testing.T) {
	appRevision := "a-revision"
	history1Revision := "history-revision-1"
	history2Revision := "history-revision-2"

	t.Run("resource revision should be taken from sync.revision", func(t *testing.T) {
		noStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
			},
		}

		revisionResult := GetApplicationLatestRevision(&noStatusHistoryAppMock)
		assert.Equal(t, revisionResult, appRevision)

		emptyStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
				History: []v1alpha1.RevisionHistory{},
			},
		}

		revision2Result := GetApplicationLatestRevision(&emptyStatusHistoryAppMock)
		assert.Equal(t, revision2Result, appRevision)
	})

	t.Run("resource revision should be taken from latest history.revision", func(t *testing.T) {
		appMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
				History: []v1alpha1.RevisionHistory{
					{
						Revision: history1Revision,
					},
					{
						Revision: history2Revision,
					},
				},
			},
		}

		revisionResult := GetApplicationLatestRevision(&appMock)
		assert.Equal(t, revisionResult, history2Revision)
	})
}

func yamlToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func jsonToAppSyncRevision(jsonStr string) *AppSyncRevisionsMetadata {
	var obj AppSyncRevisionsMetadata
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &obj
}

func TestAddCommitsDetailsToAnnotations(t *testing.T) {
	revisionMetadata := AppSyncRevisionsMetadata{
		SyncRevisions: []*RevisionWithMetadata{{
			Metadata: &v1alpha1.RevisionMetadata{
				Author:  "demo usert",
				Date:    metav1.Time{},
				Message: "some message",
			},
		}},
	}

	t.Run("set annotation when annotations object missing", func(t *testing.T) {
		resource := yamlToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"
    uid: "4"
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`,
		)

		result := AddCommitsDetailsToAnnotations(resource, &revisionMetadata)

		revMetadatUnstructured := jsonToAppSyncRevision(result.GetAnnotations()[annotationRevisionKey])

		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Author, revMetadatUnstructured.SyncRevisions[0].Metadata.Author)
		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Message, revMetadatUnstructured.SyncRevisions[0].Metadata.Message)
	})

	t.Run("set annotation when annotations present", func(t *testing.T) {
		resource := yamlToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    annotations:
      link: http://my-grafana.com/pre-generated-link
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`,
		)

		result := AddCommitsDetailsToAnnotations(resource, &revisionMetadata)

		revMetadatUnstructured := jsonToAppSyncRevision(result.GetAnnotations()[annotationRevisionKey])

		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Author, revMetadatUnstructured.SyncRevisions[0].Metadata.Author)
		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Message, revMetadatUnstructured.SyncRevisions[0].Metadata.Message)
	})
}

func TestAddCommitsDetailsToAppAnnotations(t *testing.T) {
	revisionMetadata := AppSyncRevisionsMetadata{
		SyncRevisions: []*RevisionWithMetadata{{
			Metadata: &v1alpha1.RevisionMetadata{
				Author:  "demo usert",
				Date:    metav1.Time{},
				Message: "some message",
			},
		}},
	}

	t.Run("set annotation when annotations object missing", func(t *testing.T) {
		resource := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{},
		}

		result := AddCommitsDetailsToAppAnnotations(resource, &revisionMetadata)

		revMetadatUnstructured := jsonToAppSyncRevision(result.GetAnnotations()[annotationRevisionKey])

		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Author, revMetadatUnstructured.SyncRevisions[0].Metadata.Author)
		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Message, revMetadatUnstructured.SyncRevisions[0].Metadata.Message)
	})

	t.Run("set annotation when annotations present", func(t *testing.T) {
		resource := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"test": "value",
				},
			},
		}

		result := AddCommitsDetailsToAppAnnotations(resource, &revisionMetadata)

		revMetadatUnstructured := jsonToAppSyncRevision(result.GetAnnotations()[annotationRevisionKey])

		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Author, revMetadatUnstructured.SyncRevisions[0].Metadata.Author)
		assert.Equal(t, revisionMetadata.SyncRevisions[0].Metadata.Message, revMetadatUnstructured.SyncRevisions[0].Metadata.Message)
	})
}

func TestGetRevisions(t *testing.T) {
	t.Run("should return revisions when only they passed", func(t *testing.T) {
		val := "test"
		result := getRevisions(RevisionsData{
			Revisions: []string{val},
		})
		assert.Len(t, result, 1)
		assert.Equal(t, val, result[0])
	})
	t.Run("should return revisions when revision also passed", func(t *testing.T) {
		val := "test"
		result := getRevisions(RevisionsData{
			Revisions: []string{val, "test2"},
			Revision:  "fail",
		})
		assert.Len(t, result, 2)
		assert.Equal(t, val, result[0])
	})
	t.Run("should return revision", func(t *testing.T) {
		val := "test"
		result := getRevisions(RevisionsData{
			Revision: val,
		})
		assert.Len(t, result, 1)
		assert.Equal(t, val, result[0])
	})
}

func TestGetOperationSyncRevisions(t *testing.T) {
	t.Run("should return Status.Sync.Revision like for new apps", func(t *testing.T) {
		expectedResult := "test"
		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: expectedResult,
				},
			},
		}
		result := GetOperationSyncRevisions(&app)

		assert.Len(t, result, 1)
		assert.Equal(t, expectedResult, result[0])
	})

	t.Run("should return Status.Sync.Revisions like for new apps", func(t *testing.T) {
		expectedResult := "multi-1"
		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revisions: []string{expectedResult, "multi-2"},
					Revision:  "single",
				},
			},
		}

		result := GetOperationSyncRevisions(&app)

		assert.Len(t, result, 2)
		assert.Equal(t, expectedResult, result[0])
	})

	t.Run("should return a.Status.OperationState.Operation.Sync.Revision", func(t *testing.T) {
		expectedResult := "multi-1"
		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: "fallack",
				},
				OperationState: &v1alpha1.OperationState{
					Operation: v1alpha1.Operation{
						Sync: &v1alpha1.SyncOperation{
							Revision: expectedResult,
						},
					},
				},
			},
		}

		result := GetOperationSyncRevisions(&app)

		assert.Len(t, result, 1)
		assert.Equal(t, expectedResult, result[0])
	})

	t.Run("should return a.Status.OperationState.Operation.Sync.Revisions", func(t *testing.T) {
		expectedResult := "multi-1"

		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: "fallack",
				},
				OperationState: &v1alpha1.OperationState{
					Operation: v1alpha1.Operation{
						Sync: &v1alpha1.SyncOperation{
							Revisions: []string{expectedResult, "multi-2"},
							Revision:  "single",
						},
					},
				},
			},
		}

		result := GetOperationSyncRevisions(&app)

		assert.Len(t, result, 2)
		assert.Equal(t, expectedResult, result[0])
	})

	t.Run("should return a.Operation.Sync.Revision for first app sync", func(t *testing.T) {
		expectedResult := "multi-1"
		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: "fallack",
				},
			},
			Operation: &v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{
					Revision: expectedResult,
				},
			},
		}

		result := GetOperationSyncRevisions(&app)

		assert.Len(t, result, 1)
		assert.Equal(t, expectedResult, result[0])
	})

	t.Run("should return a.Operation.Sync.Revisions for first app sync", func(t *testing.T) {
		expectedResult := "multi-1"

		app := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: "fallack",
				},
			},
			Operation: &v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{
					Revisions: []string{expectedResult, "multi-2"},
					Revision:  "single",
				},
			},
		}

		result := GetOperationSyncRevisions(&app)

		assert.Len(t, result, 2)
		assert.Equal(t, expectedResult, result[0])
	})
}

func TestAddCommitDetailsToLabels(t *testing.T) {
	revisionMetadata := v1alpha1.RevisionMetadata{
		Author:  "demo usert",
		Date:    metav1.Time{},
		Message: "some message",
	}

	t.Run("set labels when lable object missing", func(t *testing.T) {
		resource := yamlToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    resourceVersion: "123"
    uid: "4"
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`,
		)

		result := AddCommitDetailsToLabels(resource, &revisionMetadata)
		labels := result.GetLabels()
		assert.Equal(t, revisionMetadata.Author, labels["app.meta.commit-author"])
		assert.Equal(t, revisionMetadata.Message, labels["app.meta.commit-message"])
	})

	t.Run("set labels when labels present", func(t *testing.T) {
		resource := yamlToUnstructured(`
  apiVersion: v1
  kind: Service
  metadata:
    name: helm-guestbook
    namespace: default
    labels:
      link: http://my-grafana.com/pre-generated-link
  spec:
    selector:
      app: guestbook
    type: LoadBalancer
  status:
    loadBalancer:
      ingress:
      - hostname: localhost`,
		)

		result := AddCommitDetailsToLabels(resource, &revisionMetadata)
		labels := result.GetLabels()
		assert.Equal(t, revisionMetadata.Author, labels["app.meta.commit-author"])
		assert.Equal(t, revisionMetadata.Message, labels["app.meta.commit-message"])
		assert.Equal(t, "http://my-grafana.com/pre-generated-link", result.GetLabels()["link"])
	})
}
