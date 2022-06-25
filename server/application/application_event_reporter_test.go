package application

import (
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-cd/v2/pkg/apis/application"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apppkg "github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
)

func TestGetResourceEventPayload(t *testing.T) {
	t.Run("Deleting timestamp is empty", func(t *testing.T) {

		app := v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}
		es := events.EventSource{}

		actualState := apppkg.ApplicationResourceResponse{
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

		actualState := apppkg.ApplicationResourceResponse{
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

func TestGetRevision(t *testing.T) {
	t.Run("application", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{},
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: "a-revision",
				},
			},
		}
		rs := v1alpha1.ResourceStatus{
			Group:   application.Group,
			Version: "v1alpha1",
			Kind:    application.ApplicationKind,
		}
		manifestResponse := apiclient.ManifestResponse{
			Revision: "mr-revision",
		}
		revision := getRevision(&app, &rs, &manifestResponse)
		assert.Equal(t, revision, "a-revision")
	})

	t.Run("application", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{},
			},
			Status: v1alpha1.ApplicationStatus{
				Sync: v1alpha1.SyncStatus{
					Revision: "a-revision",
				},
			},
		}
		rs := v1alpha1.ResourceStatus{}
		manifestResponse := apiclient.ManifestResponse{
			Revision: "mr-revision",
		}
		revision := getRevision(&app, &rs, &manifestResponse)
		assert.Equal(t, revision, "mr-revision")
	})
}
