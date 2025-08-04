package controller

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/test"
)

func TestGetRepoObjs(t *testing.T) {
	cm := test.NewConfigMap()
	cm.SetAnnotations(map[string]string{
		"custom-annotation":             "custom-value",
		common.AnnotationInstallationID: "id",     // tracking annotation should be removed
		common.AnnotationKeyAppInstance: "my-app", // tracking annotation should be removed
	})
	cmBytes, _ := json.Marshal(cm)

	app := newFakeApp()
	// Enable the manifest-generate-paths annotation and set a synced revision
	app.SetAnnotations(map[string]string{v1alpha1.AnnotationKeyManifestGeneratePaths: "."})
	app.Status.Sync = v1alpha1.SyncStatus{
		Revision: "abc123",
		Status:   v1alpha1.SyncStatusCodeSynced,
	}

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{string(cmBytes)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
	}

	ctrl := newFakeControllerWithResync(&data, time.Minute, nil, errors.New("this should not be called"))
	source := app.Spec.GetSource()
	source.RepoURL = "oci://example.com/argo/argo-cd"

	objs, resp, err := ctrl.GetRepoObjs(app, source, "abc123", &v1alpha1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: test.FakeArgoCDNamespace,
		},
		Spec: v1alpha1.AppProjectSpec{
			SourceRepos: []string{"*"},
			Destinations: []v1alpha1.ApplicationDestination{
				{
					Server:    "*",
					Namespace: "*",
				},
			},
		},
	})
	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "abc123", resp.Revision)
	assert.Len(t, objs, 1)

	annotations := objs[0].GetAnnotations()

	// only the tracking annotations set by Argo CD should be removed
	// and not the custom annotations set by user
	require.NotNil(t, annotations)
	assert.Equal(t, "custom-value", annotations["custom-annotation"])
	assert.NotContains(t, annotations, common.AnnotationInstallationID)
	assert.NotContains(t, annotations, common.AnnotationKeyAppInstance)

	assert.Equal(t, "ConfigMap", objs[0].GetKind())
}
