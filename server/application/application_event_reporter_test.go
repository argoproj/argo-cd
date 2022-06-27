package application

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

func TestGetResourceEventPayload(t *testing.T) {
	t.Run("Deleting timestamp is empty", func(t *testing.T) {

		app := v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}
		es := events.EventSource{}

		actualState := application.ApplicationResourceResponse{
			Manifest: "{ \"key\" : \"manifest\" }",
		}
		desiredState := apiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		manifestResponse := apiclient.ManifestResponse{}
		appTree := v1alpha1.ApplicationTree{}

		event, err := getResourceEventPayload(&app, &rs, &es, &actualState, &desiredState, &manifestResponse, &appTree, true, "", nil)
		assert.NoError(t, err)

		var eventPayload events.EventPayload

		err = json.Unmarshal(event.Payload, &eventPayload)
		assert.NoError(t, err)

		assert.Equal(t, "{ \"key\" : \"manifest\" }", eventPayload.Source.DesiredManifest)
		assert.Equal(t, "{ \"key\" : \"manifest\" }", eventPayload.Source.ActualManifest)
	})

	t.Run("Deleting timestamp is empty", func(t *testing.T) {

		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{},
			},
		}
		rs := v1alpha1.ResourceStatus{}
		es := events.EventSource{}

		actualState := application.ApplicationResourceResponse{
			Manifest: "{ \"key\" : \"manifest\" }",
		}
		desiredState := apiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		manifestResponse := apiclient.ManifestResponse{}
		appTree := v1alpha1.ApplicationTree{}

		event, err := getResourceEventPayload(&app, &rs, &es, &actualState, &desiredState, &manifestResponse, &appTree, true, "", nil)
		assert.NoError(t, err)

		var eventPayload events.EventPayload

		err = json.Unmarshal(event.Payload, &eventPayload)
		assert.NoError(t, err)

		assert.Equal(t, "", eventPayload.Source.DesiredManifest)
		assert.Equal(t, "", eventPayload.Source.ActualManifest)
	})
}

func TestGetResourceRevision(t *testing.T) {
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

		revisionResult := getResourceRevision(&noStatusHistoryAppMock)
		assert.Equal(t, revisionResult, appRevision)

		emptyStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
				History: []v1alpha1.RevisionHistory{},
			},
		}

		revision2Result := getResourceRevision(&emptyStatusHistoryAppMock)
		assert.Equal(t, revision2Result, appRevision)
	})

	t.Run("resource revision should be taken from latest history.revision", func(t *testing.T) {
		appMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: appRevision,
				},
				History: []v1alpha1.RevisionHistory{
					v1alpha1.RevisionHistory{
						Revision: history1Revision,
					},
					v1alpha1.RevisionHistory{
						Revision: history2Revision,
					},
				},
			},
		}

		revisionResult := getResourceRevision(&appMock)
		assert.Equal(t, revisionResult, history2Revision)
	})
}

func TestGetLatestAppHistoryId(t *testing.T) {
	history1Id := int64(1)
	history2Id := int64(2)

	t.Run("resource revision should be 0", func(t *testing.T) {
		noStatusHistoryAppMock := v1alpha1.Application{}

		idResult := getLatestAppHistoryId(&noStatusHistoryAppMock)
		assert.Equal(t, idResult, int64(0))

		emptyStatusHistoryAppMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				History: []v1alpha1.RevisionHistory{},
			},
		}

		id2Result := getLatestAppHistoryId(&emptyStatusHistoryAppMock)
		assert.Equal(t, id2Result, int64(0))
	})

	t.Run("resource revision should be taken from latest history.Id", func(t *testing.T) {
		appMock := v1alpha1.Application{
			Status: v1alpha1.ApplicationStatus{
				History: []v1alpha1.RevisionHistory{
					v1alpha1.RevisionHistory{
						ID: history1Id,
					},
					v1alpha1.RevisionHistory{
						ID: history2Id,
					},
				},
			},
		}

		revisionResult := getLatestAppHistoryId(&appMock)
		assert.Equal(t, revisionResult, history2Id)
	})
}
