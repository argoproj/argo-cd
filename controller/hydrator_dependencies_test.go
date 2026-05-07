package controller

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v3/common"
	"github.com/argoproj/argo-cd/v3/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v3/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v3/test"
	"github.com/argoproj/argo-cd/v3/util/settings"
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

	ctrl := newFakeControllerWithResync(t.Context(), &data, time.Minute, nil, errors.New("this should not be called"))
	source := app.Spec.GetSource()
	source.RepoURL = "oci://example.com/argo/argo-cd"

	objs, resp, err := ctrl.GetRepoObjs(t.Context(), app, source, "abc123", &v1alpha1.AppProject{
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

func TestGetHydratorCommitMessageTemplate_WhenTemplateisNotDefined_FallbackToDefault(t *testing.T) {
	cm := test.NewConfigMap()
	cmBytes, _ := json.Marshal(cm)

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{string(cmBytes)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
	}

	ctrl := newFakeControllerWithResync(t.Context(), &data, time.Minute, nil, errors.New("this should not be called"))

	tmpl, err := ctrl.GetHydratorCommitMessageTemplate()
	require.NoError(t, err)
	assert.NotEmpty(t, tmpl) // should fallback to default
	assert.Equal(t, settings.CommitMessageTemplate, tmpl)
}

func TestGetHydratorCommitMessageTemplate(t *testing.T) {
	cm := test.NewFakeConfigMap()
	cm.Data["sourceHydrator.commitMessageTemplate"] = settings.CommitMessageTemplate
	cmBytes, _ := json.Marshal(cm)

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{string(cmBytes)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		configMapData: cm.Data,
	}

	ctrl := newFakeControllerWithResync(t.Context(), &data, time.Minute, nil, errors.New("this should not be called"))

	tmpl, err := ctrl.GetHydratorCommitMessageTemplate()
	require.NoError(t, err)
	assert.NotEmpty(t, tmpl)
}

func TestProcessAppHydrateQueueItem_ReconcileTimeout_EnqueuesHydrationOnDryRevisionChange(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        app.Spec.Source.RepoURL,
			TargetRevision: app.Spec.Source.TargetRevision,
			Path:           app.Spec.Source.Path,
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "hydrated/path",
		},
	}
	app.Status.SourceHydrator.CurrentOperation = &v1alpha1.HydrateOperation{
		Phase:          v1alpha1.HydrateOperationPhaseHydrated,
		DrySHA:         "old-sha",
		SourceHydrator: *app.Spec.SourceHydrator,
	}
	reconciledAt := metav1.NewTime(time.Now().Add(-2 * time.Minute))
	app.Status.ReconciledAt = &reconciledAt

	data := fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "new-sha",
		},
	}

	ctrl := newFakeHydratorControllerWithResync(t.Context(), &data, time.Minute, nil, nil)
	ctrl.appHydrateQueue.Add(app.QualifiedName())
	processed := ctrl.processAppHydrateQueueItem()
	require.True(t, processed)

	require.Eventually(t, func() bool {
		updatedApp, err := ctrl.appLister.Applications(app.Namespace).Get(app.Name)
		if err != nil {
			return false
		}
		op := updatedApp.Status.SourceHydrator.CurrentOperation
		return op != nil && op.Phase == v1alpha1.HydrateOperationPhaseHydrating
	}, 2*time.Second, 25*time.Millisecond)
}
