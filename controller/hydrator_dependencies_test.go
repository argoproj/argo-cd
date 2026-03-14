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

func TestValidateHydratedCommitFreshness_NoHydrator(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = nil

	data := fakeData{}
	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "abc123")
	require.NoError(t, err)
	assert.True(t, isValid)
	assert.Empty(t, rootDrySHA)
	assert.Empty(t, pathDrySHA)
}

func TestValidateHydratedCommitFreshness_Fresh(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        "https://github.com/example/dry-repo",
			TargetRevision: "main",
			Path:           ".",
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "app1",
		},
	}

	rootMetadata := map[string]any{
		"drySHA": "dry-sha-123",
	}
	pathMetadata := map[string]any{
		"drySHA": "dry-sha-123",
	}

	rootMetadataBytes, _ := json.Marshal(rootMetadata)
	pathMetadataBytes, _ := json.Marshal(pathMetadata)

	data := fakeData{
		gitFilesResponse: map[string][]byte{
			HydratorMetadataFile:           rootMetadataBytes,
			"app1/" + HydratorMetadataFile: pathMetadataBytes,
		},
	}

	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "hydrated-sha-456")
	require.NoError(t, err)
	assert.True(t, isValid)
	assert.Equal(t, "dry-sha-123", rootDrySHA)
	assert.Equal(t, "dry-sha-123", pathDrySHA)
}

func TestValidateHydratedCommitFreshness_Stale(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        "https://github.com/example/dry-repo",
			TargetRevision: "main",
			Path:           ".",
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "app1",
		},
	}

	rootMetadata := map[string]any{
		"drySHA": "dry-sha-new",
	}
	pathMetadata := map[string]any{
		"drySHA": "dry-sha-old",
	}

	rootMetadataBytes, _ := json.Marshal(rootMetadata)
	pathMetadataBytes, _ := json.Marshal(pathMetadata)

	data := fakeData{
		gitFilesResponse: map[string][]byte{
			HydratorMetadataFile:           rootMetadataBytes,
			"app1/" + HydratorMetadataFile: pathMetadataBytes,
		},
	}

	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "hydrated-sha-456")
	require.NoError(t, err)
	assert.False(t, isValid)
	assert.Equal(t, "dry-sha-new", rootDrySHA)
	assert.Equal(t, "dry-sha-old", pathDrySHA)
}

func TestValidateHydratedCommitFreshness_MissingRootMetadata(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        "https://github.com/example/dry-repo",
			TargetRevision: "main",
			Path:           ".",
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "app1",
		},
	}

	data := fakeData{
		gitFilesResponse: map[string][]byte{},
	}

	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "hydrated-sha-456")
	require.NoError(t, err) // Missing files are not errors, just stale
	assert.False(t, isValid)
	assert.Empty(t, rootDrySHA)
	assert.Empty(t, pathDrySHA)
}

func TestValidateHydratedCommitFreshness_MissingPathMetadata(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        "https://github.com/example/dry-repo",
			TargetRevision: "main",
			Path:           ".",
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "app1",
		},
	}

	rootMetadata := map[string]any{
		"drySHA": "dry-sha-123",
	}

	rootMetadataBytes, _ := json.Marshal(rootMetadata)

	data := fakeData{
		gitFilesResponse: map[string][]byte{
			HydratorMetadataFile: rootMetadataBytes,
		},
	}

	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "hydrated-sha-456")
	require.NoError(t, err) // Missing files are not errors, just stale
	assert.False(t, isValid)
	assert.Equal(t, "dry-sha-123", rootDrySHA)
	assert.Empty(t, pathDrySHA)
}

func TestValidateHydratedCommitFreshness_MalformedRootMetadata(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        "https://github.com/example/dry-repo",
			TargetRevision: "main",
			Path:           ".",
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "app1",
		},
	}

	data := fakeData{
		gitFilesResponse: map[string][]byte{
			HydratorMetadataFile: []byte("invalid json"),
		},
	}

	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "hydrated-sha-456")
	require.Error(t, err) // Malformed metadata is an error
	assert.False(t, isValid)
	assert.Empty(t, rootDrySHA)
	assert.Empty(t, pathDrySHA)
	assert.Contains(t, err.Error(), "failed to parse root metadata")
}

func TestValidateHydratedCommitFreshness_MalformedPathMetadata(t *testing.T) {
	app := newFakeApp()
	app.Spec.SourceHydrator = &v1alpha1.SourceHydrator{
		DrySource: v1alpha1.DrySource{
			RepoURL:        "https://github.com/example/dry-repo",
			TargetRevision: "main",
			Path:           ".",
		},
		SyncSource: v1alpha1.SyncSource{
			TargetBranch: "hydrated",
			Path:         "app1",
		},
	}

	rootMetadata := map[string]any{
		"drySHA": "dry-sha-123",
	}

	rootMetadataBytes, _ := json.Marshal(rootMetadata)

	data := fakeData{
		gitFilesResponse: map[string][]byte{
			HydratorMetadataFile:           rootMetadataBytes,
			"app1/" + HydratorMetadataFile: []byte("invalid json"),
		},
	}

	ctrl := newFakeController(t.Context(), &data, nil)

	isValid, rootDrySHA, pathDrySHA, err := ctrl.ValidateHydratedCommitFreshness(t.Context(), app, "hydrated-sha-456")
	require.Error(t, err) // Malformed metadata is an error
	assert.False(t, isValid)
	assert.Equal(t, "dry-sha-123", rootDrySHA)
	assert.Empty(t, pathDrySHA)
	assert.Contains(t, err.Error(), "failed to parse path metadata")
}
