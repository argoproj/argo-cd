package reporter

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/application"
	"github.com/argoproj/argo-cd/v2/pkg/apiclient/events"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	repoApiclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/util/argo"
)

func TestGetResourceEventPayload(t *testing.T) {
	t.Run("Deleting timestamp is empty", func(t *testing.T) {
		app := v1alpha1.Application{}
		rs := v1alpha1.ResourceStatus{}

		man := "{ \"key\" : \"manifest\" }"

		actualState := application.ApplicationResourceResponse{
			Manifest: &man,
		}
		desiredState := repoApiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		appTree := v1alpha1.ApplicationTree{}
		revisionMetadata := v1alpha1.RevisionMetadata{
			Author:  "demo usert",
			Date:    metav1.Time{},
			Message: "some message",
		}

		event, err := getResourceEventPayload(&app, &rs, &actualState, &desiredState, &appTree, true, "", nil, &revisionMetadata, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &repoApiclient.ApplicationVersions{})
		require.NoError(t, err)

		var eventPayload events.EventPayload

		err = json.Unmarshal(event.Payload, &eventPayload)
		require.NoError(t, err)

		assert.Equal(t, "{ \"key\" : \"manifest\" }", eventPayload.Source.DesiredManifest)
		assert.Equal(t, "{ \"key\" : \"manifest\" }", eventPayload.Source.ActualManifest)
	})

	t.Run("Deleting timestamp is empty", func(t *testing.T) {
		app := v1alpha1.Application{
			ObjectMeta: metav1.ObjectMeta{
				DeletionTimestamp: &metav1.Time{},
			},
			Status: v1alpha1.ApplicationStatus{},
		}
		rs := v1alpha1.ResourceStatus{}
		man := "{ \"key\" : \"manifest\" }"
		actualState := application.ApplicationResourceResponse{
			Manifest: &man,
		}
		desiredState := repoApiclient.Manifest{
			CompiledManifest: "{ \"key\" : \"manifest\" }",
		}
		appTree := v1alpha1.ApplicationTree{}
		revisionMetadata := v1alpha1.RevisionMetadata{
			Author:  "demo usert",
			Date:    metav1.Time{},
			Message: "some message",
		}

		event, err := getResourceEventPayload(&app, &rs, &actualState, &desiredState, &appTree, true, "", nil, &revisionMetadata, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &repoApiclient.ApplicationVersions{})
		require.NoError(t, err)

		var eventPayload events.EventPayload

		err = json.Unmarshal(event.Payload, &eventPayload)
		require.NoError(t, err)

		assert.Equal(t, "", eventPayload.Source.DesiredManifest)
		assert.Equal(t, "", eventPayload.Source.ActualManifest)
	})
}

func TestGetResourceEventPayloadWithoutRevision(t *testing.T) {
	app := v1alpha1.Application{}
	rs := v1alpha1.ResourceStatus{}

	mf := "{ \"key\" : \"manifest\" }"

	actualState := application.ApplicationResourceResponse{
		Manifest: &mf,
	}
	desiredState := repoApiclient.Manifest{
		CompiledManifest: "{ \"key\" : \"manifest\" }",
	}
	appTree := v1alpha1.ApplicationTree{}

	_, err := getResourceEventPayload(&app, &rs, &actualState, &desiredState, &appTree, true, "", nil, nil, nil, common.LabelKeyAppInstance, argo.TrackingMethodLabel, &repoApiclient.ApplicationVersions{})
	assert.NoError(t, err)
}
