package controller

import (
	"context"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	"github.com/argoproj/gitops-engine/pkg/sync"
	"github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

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
	ctrl := newFakeController(&data, nil)

	// Sync with source unspecified
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}}
	ctrl.appStateManager.SyncAppState(app, opState)
	// Ensure we record spec.source into sync result
	assert.Equal(t, app.Spec.GetSource(), opState.SyncResult.Source)

	updatedApp, err := ctrl.applicationClientset.ArgoprojV1alpha1().Applications(app.Namespace).Get(context.Background(), app.Name, v1.GetOptions{})
	assert.Nil(t, err)
	assert.Equal(t, 1, len(updatedApp.Status.History))
	assert.Equal(t, app.Spec.GetSource(), updatedApp.Status.History[0].Source)
	assert.Equal(t, "abc123", updatedApp.Status.History[0].Revision)
}

func TestPersistManagedNamespaceMetadataState(t *testing.T) {
	app := newFakeApp()
	app.Status.OperationState = nil
	app.Status.History = nil
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &v1alpha1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}

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
	ctrl := newFakeController(&data, nil)

	// Sync with source unspecified
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}}
	ctrl.appStateManager.SyncAppState(app, opState)
	// Ensure we record spec.syncPolicy.managedNamespaceMetadata into sync result
	assert.Equal(t, app.Spec.SyncPolicy.ManagedNamespaceMetadata, opState.SyncResult.ManagedNamespaceMetadata)
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
	ctrl := newFakeController(&data, nil)

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
	ctrl := newFakeController(&data, nil)

	// Sync with source unspecified
	opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
		Sync: &v1alpha1.SyncOperation{},
	}}
	t.Setenv("ARGOCD_GPG_ENABLED", "true")
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
		ctrl := newFakeController(&data, nil)

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

func Test_areDependenciesReady(t *testing.T) {
	parent := newFakeApp()
	parent.Name = "parent-app"
	parent.Spec.DependsOn = &v1alpha1.ApplicationDependency{
		Selectors: []v1alpha1.ApplicationSelector{
			{
				LabelSelector: &v1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
	}

	dep1 := newFakeApp()
	dep1.Name = "dep1"
	dep1.Labels = map[string]string{"foo": "bar"}

	state := &v1alpha1.OperationState{
		Phase: common.OperationRunning,
	}

	syncOp := v1alpha1.SyncOperation{SyncStrategy: &v1alpha1.SyncStrategy{}}
	t.Run("Ready to sync", func(t *testing.T) {
		p := parent.DeepCopy()
		d := dep1.DeepCopy()
		d.Status.Health = v1alpha1.HealthStatus{Status: health.HealthStatusHealthy}
		d.Status.Sync = v1alpha1.SyncStatus{Status: v1alpha1.SyncStatusCodeSynced}
		s := state.DeepCopy()
		data := fakeData{
			apps: []runtime.Object{p, d},
		}
		ctrl := newFakeController(&data)
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.True(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Empty(t, s.Message)
		assert.Empty(t, s.WaitingFor)
	})

	t.Run("Not ready to sync because dependency is out of sync", func(t *testing.T) {
		p := parent.DeepCopy()
		d := dep1.DeepCopy()
		s := state.DeepCopy()
		data := fakeData{
			apps: []runtime.Object{p, d},
		}
		ctrl := newFakeController(&data)
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.False(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Contains(t, s.Message, "Waiting for dependencies")
		assert.Contains(t, s.Message, "dep1")
		require.Len(t, s.WaitingFor, 1)
		assert.Equal(t, "dep1", s.WaitingFor[0].ApplicationName)
		assert.Equal(t, p.GetNamespace(), s.WaitingFor[0].ApplicationNamespace)
		assert.Nil(t, s.WaitingFor[0].RefreshedAt)
	})

	t.Run("Preserve refresh time for dependencies", func(t *testing.T) {
		p := parent.DeepCopy()
		d := dep1.DeepCopy()
		s := state.DeepCopy()
		data := fakeData{
			apps: []runtime.Object{p, d},
		}
		ctrl := newFakeController(&data)
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.False(t, ready)
		require.Len(t, s.WaitingFor, 1)
		s.WaitingFor[0].RefreshedAt = &v1.Time{Time: time.Now()}
		ready = ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.False(t, ready)
		require.Len(t, s.WaitingFor, 1)
		require.NotNil(t, s.WaitingFor[0].RefreshedAt)
	})

	t.Run("Not ready to sync because blocking for dependency creation", func(t *testing.T) {
		p := parent.DeepCopy()
		p.Spec.DependsOn.BlockOnEmpty = pointer.Bool(true)
		s := state.DeepCopy()
		data := fakeData{
			apps: []runtime.Object{p},
		}
		ctrl := newFakeController(&data)
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.False(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Contains(t, s.Message, "Waiting for any app to be created")
	})

	t.Run("Sync fails due to timeout", func(t *testing.T) {
		p := parent.DeepCopy()
		p.Spec.DependsOn.Timeout = pointer.Duration(10 * time.Second)
		s := state.DeepCopy()
		started := time.Now().Add(-20 * time.Second)
		s.StartedAt = v1.Time{Time: started}
		data := fakeData{
			apps: []runtime.Object{p},
		}
		ctrl := newFakeController(&data)
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.False(t, ready)
		assert.Equal(t, common.OperationFailed, s.Phase)
		assert.Contains(t, s.Message, "Timeout waiting")
	})

	t.Run("Sync proceeds because timeout not reached", func(t *testing.T) {
		p := parent.DeepCopy()
		p.Spec.DependsOn.Timeout = pointer.Duration(10 * time.Second)
		s := state.DeepCopy()
		started := time.Now()
		s.StartedAt = v1.Time{Time: started}
		data := fakeData{
			apps: []runtime.Object{p},
		}
		ctrl := newFakeController(&data)
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.True(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Empty(t, s.Message)
	})

	t.Run("Automatic sync start is delayed", func(t *testing.T) {
		// The fake app has auto-sync enabled by default
		p := parent.DeepCopy()
		p.Spec.DependsOn.SyncDelay = pointer.Duration(10 * time.Second)
		s := state.DeepCopy()
		started := time.Now()
		s.StartedAt = v1.Time{Time: started}
		data := fakeData{
			apps: []runtime.Object{p},
		}
		ctrl := newFakeController(&data)

		// Initially, we have a delay
		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.False(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Contains(t, s.Message, "Delaying sync start")

		// Second run without delay
		s.StartedAt = v1.Time{Time: started.Add(-20 * time.Second)}
		ready = ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.True(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Empty(t, s.Message)
	})

	t.Run("Sync delay does not affect manual sync", func(t *testing.T) {
		p := parent.DeepCopy()
		p.Spec.DependsOn.SyncDelay = pointer.Duration(10 * time.Second)
		p.Spec.SyncPolicy = nil
		s := state.DeepCopy()
		started := time.Now()
		s.StartedAt = v1.Time{Time: started}
		data := fakeData{
			apps: []runtime.Object{p},
		}
		ctrl := newFakeController(&data)

		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.True(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Empty(t, s.Message)
	})

}
