package utils

import (
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func StrToUnstructured(jsonStr string) *unstructured.Unstructured {
	obj := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(jsonStr), &obj)
	if err != nil {
		panic(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestAddCommitDetailsToLabels(t *testing.T) {
	revisionMetadata := v1alpha1.RevisionMetadata{
		Author:  "demo usert",
		Date:    metav1.Time{},
		Message: "some message",
	}

	t.Run("set labels when lable object missing", func(t *testing.T) {
		resource := StrToUnstructured(`
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
		resource := StrToUnstructured(`
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
