package controller

import (
	"context"
	"os"
	"testing"

	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v2/controller/testdata"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/argo/diff"
)

func TestPersistRevisionHistory(t *testing.T) {
	app := newFakeApp()
	app.Status.OperationState = nil
	app.Status.History = nil

	defaultProject := &v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Namespace: test.FakeArgoCDNamespace,
			Name:      "default",
		},
	}
	data := fakeData{
		apps: []runtime.Object{app, defaultProject},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	// Sync with source unspecified
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}}
	ctrl.appStateManager.SyncAppState(app, opState)
	// Ensure we record spec.source into sync result
	assert.Equal(t, app.Spec.Source, opState.SyncResult.Source)

	updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(context.Background(), app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.Equal(t, app.Spec.Source, updatedApp.Status.History[0].Source)
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)
}

func TestPersistRevisionHistoryRollback(t *testing.T) {
	app := newFakeApp()
	app.Status.OperationState = nil
	app.Status.History = nil
	defaultProject := &v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Namespace: test.FakeArgoCDNamespace,
			Name:      "default",
		},
	}
	data := fakeData{
		apps: []runtime.Object{app, defaultProject},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	// Sync with source specified
	source := v1alpha1.ApplicationSource{
		Helm: &v1alpha1.ApplicationSourceHelm{
			Parameters: []v1alpha1.HelmParameter{
				{
					Name:  "test",
					Value: "123",
				},
			},
		},
	}
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{
			Source: &source,
		},
	}}
	ctrl.appStateManager.SyncAppState(app, opState)
	// Ensure we record opState's source into sync result
	assert.Equal(t, source, opState.SyncResult.Source)

	updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(context.Background(), app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.Equal(t, source, updatedApp.Status.History[0].Source)
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)
}

func TestSyncComparisonError(t *testing.T) {
	app := newFakeApp()
	app.Status.OperationState = nil
	app.Status.History = nil

	defaultProject := &v1alpha1.AppProject{
		ObjectMeta: v1.ObjectMeta{
			Namespace: test.FakeArgoCDNamespace,
			Name:      "default",
		},
		Spec: v1alpha1.AppProjectSpec{
			SignatureKeys: []v1alpha1.SignatureKey{{KeyID: "test"}},
		},
	}
	data := fakeData{
		apps: []runtime.Object{app, defaultProject},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests:    []string{},
			Namespace:    test.FakeDestNamespace,
			Server:       test.FakeClusterURL,
			Revision:     "abc123",
			VerifyResult: "something went wrong",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	// Sync with source unspecified
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}}
	os.Setenv("ARGOCD_GPG_ENABLED", "true")
	defer os.Setenv("ARGOCD_GPG_ENABLED", "false")
	ctrl.appStateManager.SyncAppState(app, opState)

	conditions := app.Status.GetConditions(map[v1alpha1.ApplicationConditionType]bool{v1alpha1.ApplicationConditionComparisonError: true})
	assert.NotEmpty(t, conditions)
	assert.Equal(t, "abc123", opState.SyncResult.Revision)
}

func TestAppStateManager_SyncAppState(t *testing.T) {
	type fixture struct {
		project     *v1alpha1.AppProject
		application *v1alpha1.Application
		controller  *ApplicationController
	}

	setup := func() *fixture {
		app := newFakeApp()
		app.Status.OperationState = nil
		app.Status.History = nil

		project := &v1alpha1.AppProject{
			ObjectMeta: v1.ObjectMeta{
				Namespace: test.FakeArgoCDNamespace,
				Name:      "default",
			},
			Spec: v1alpha1.AppProjectSpec{
				SignatureKeys: []v1alpha1.SignatureKey{{KeyID: "test"}},
			},
		}
		data := fakeData{
			apps: []runtime.Object{app, project},
			manifestResponse: &apiclient.ManifestResponse{
				Manifests: []string{},
				Namespace: test.FakeDestNamespace,
				Server:    test.FakeClusterURL,
				Revision:  "abc123",
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data)

		return &fixture{
			project:     project,
			application: app,
			controller:  ctrl,
		}
	}

	t.Run("will fail the sync if finds shared resources", func(t *testing.T) {
		// given
		t.Parallel()
		f := setup()
		syncErrorMsg := "deployment already applied by another application"
		condition := v1alpha1.ApplicationCondition{
			Type:    v1alpha1.ApplicationConditionSharedResourceWarning,
			Message: syncErrorMsg,
		}
		f.application.Status.Conditions = append(f.application.Status.Conditions, condition)

		// Sync with source unspecified
		opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
			Sync: &v1alpha1.SyncOperation{
				Source:      &v1alpha1.ApplicationSource{},
				SyncOptions: []string{"FailOnSharedResource=true"},
			},
		}}

		// when
		f.controller.appStateManager.SyncAppState(f.application, opState)

		// then
		assert.Equal(t, common.OperationFailed, opState.Phase)
		assert.Contains(t, opState.Message, syncErrorMsg)
	})
}

func TestNormalizeTargetResources(t *testing.T) {
	type fixture struct {
		comparisonResult *comparisonResult
	}
	setup := func(t *testing.T, ignores []v1alpha1.ResourceIgnoreDifferences) *fixture {
		t.Helper()
		dc, err := diff.NewDiffConfigBuilder().
			WithDiffSettings(ignores, nil, true).
			WithNoCache().
			Build()
		require.NoError(t, err)
		live := test.YamlToUnstructured(testdata.LiveDeploymentYaml)
		target := test.YamlToUnstructured(testdata.TargetDeploymentYaml)
		return &fixture{
			&comparisonResult{
				reconciliationResult: sync.ReconciliationResult{
					Live:   []*unstructured.Unstructured{live},
					Target: []*unstructured.Unstructured{target},
				},
				diffConfig: dc,
			},
		}
	}
	t.Run("will modify target resource adding live state in fields it should ignore", func(t *testing.T) {
		// given
		ignore := v1alpha1.ResourceIgnoreDifferences{
			Group:                 "*",
			Kind:                  "*",
			ManagedFieldsManagers: []string{"janitor"},
		}
		ignores := []v1alpha1.ResourceIgnoreDifferences{ignore}
		f := setup(t, ignores)

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		iksmVersion := targets[0].GetAnnotations()["iksm-version"]
		assert.Equal(t, "2.0", iksmVersion)
	})
	t.Run("will not modify target resource if ignore difference is not configured", func(t *testing.T) {
		// given
		f := setup(t, []v1alpha1.ResourceIgnoreDifferences{})

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		iksmVersion := targets[0].GetAnnotations()["iksm-version"]
		assert.Equal(t, "1.0", iksmVersion)
	})
	t.Run("will remove fields from target if not present in live", func(t *testing.T) {
		ignore := v1alpha1.ResourceIgnoreDifferences{
			Group:        "apps",
			Kind:         "Deployment",
			JSONPointers: []string{"/metadata/annotations/iksm-version"},
		}
		ignores := []v1alpha1.ResourceIgnoreDifferences{ignore}
		f := setup(t, ignores)
		live := f.comparisonResult.reconciliationResult.Live[0]
		unstructured.RemoveNestedField(live.Object, "metadata", "annotations", "iksm-version")

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		_, ok := targets[0].GetAnnotations()["iksm-version"]
		assert.False(t, ok)
	})
	t.Run("will correctly normalize with multiple ignore configurations", func(t *testing.T) {
		// given
		ignores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:        "apps",
				Kind:         "Deployment",
				JSONPointers: []string{"/spec/replicas"},
			},
			{
				Group:                 "*",
				Kind:                  "*",
				ManagedFieldsManagers: []string{"janitor"},
			},
		}
		f := setup(t, ignores)

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		normalized := targets[0]
		iksmVersion, ok := normalized.GetAnnotations()["iksm-version"]
		require.True(t, ok)
		assert.Equal(t, "2.0", iksmVersion)
		replicas, ok, err := unstructured.NestedInt64(normalized.Object, "spec", "replicas")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, int64(4), replicas)
	})
	t.Run("will keep new array entries not found in live state if not ignored", func(t *testing.T) {
		t.Skip("limitation in the current implementation")
		// given
		ignores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:             "apps",
				Kind:              "Deployment",
				JQPathExpressions: []string{".spec.template.spec.containers[] | select(.name == \"guestbook-ui\")"},
			},
		}
		f := setup(t, ignores)
		target := test.YamlToUnstructured(testdata.TargetDeploymentNewEntries)
		f.comparisonResult.reconciliationResult.Target = []*unstructured.Unstructured{target}

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		containers, ok, err := unstructured.NestedSlice(targets[0].Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, 2, len(containers))
	})
}
