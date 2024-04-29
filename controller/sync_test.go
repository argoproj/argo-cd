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
	"github.com/argoproj/argo-cd/v2/util/argo/normalizers"
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

func TestSyncWindowDeniesSync(t *testing.T) {
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
				SyncWindows: v1alpha1.SyncWindows{{
					Kind:         "deny",
					Schedule:     "0 0 * * *",
					Duration:     "24h",
					Clusters:     []string{"*"},
					Namespaces:   []string{"*"},
					Applications: []string{"*"},
				}},
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

	t.Run("will keep the sync progressing if a sync window prevents the sync", func(t *testing.T) {
		// given a project with an active deny sync window and an operation in progress
		t.Parallel()
		f := setup()
		opMessage := "Sync operation blocked by sync window"

		opState := &v1alpha1.OperationState{Operation: v1alpha1.Operation{
			Sync: &v1alpha1.SyncOperation{
				Source: &v1alpha1.ApplicationSource{},
			}},
			Phase: common.OperationRunning,
		}
		// when
		f.controller.appStateManager.SyncAppState(f.application, opState)

		//then
		assert.Equal(t, common.OperationRunning, opState.Phase)
		assert.Contains(t, opState.Message, opMessage)
	})

}

func TestNormalizeTargetResources(t *testing.T) {
	type fixture struct {
		comparisonResult *comparisonResult
	}
	setup := func(t *testing.T, ignores []v1alpha1.ResourceIgnoreDifferences) *fixture {
		t.Helper()
		dc, err := diff.NewDiffConfigBuilder().
			WithDiffSettings(ignores, nil, true, normalizers.IgnoreNormalizerOpts{}).
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
		ctrl := newFakeController(&data, nil)
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
		ctrl := newFakeController(&data, nil)
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
		ctrl := newFakeController(&data, nil)
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
		ctrl := newFakeController(&data, nil)
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
		ctrl := newFakeController(&data, nil)
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
		ctrl := newFakeController(&data, nil)
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
		ctrl := newFakeController(&data, nil)

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
		ctrl := newFakeController(&data, nil)

		ready := ctrl.appStateManager.(*appStateManager).areDependenciesReady(p, s, syncOp)
		assert.True(t, ready)
		assert.Equal(t, common.OperationRunning, s.Phase)
		assert.Empty(t, s.Message)
	})
}

func TestNormalizeTargetResourcesWithList(t *testing.T) {
	type fixture struct {
		comparisonResult *comparisonResult
	}
	setupHttpProxy := func(t *testing.T, ignores []v1alpha1.ResourceIgnoreDifferences) *fixture {
		t.Helper()
		dc, err := diff.NewDiffConfigBuilder().
			WithDiffSettings(ignores, nil, true, normalizers.IgnoreNormalizerOpts{}).
			WithNoCache().
			Build()
		require.NoError(t, err)
		live := test.YamlToUnstructured(testdata.LiveHTTPProxy)
		target := test.YamlToUnstructured(testdata.TargetHTTPProxy)
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

	t.Run("will properly ignore nested fields within arrays", func(t *testing.T) {
		// given
		ignores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:             "projectcontour.io",
				Kind:              "HTTPProxy",
				JQPathExpressions: []string{".spec.routes[]"},
				//JSONPointers: []string{"/spec/routes"},
			},
		}
		f := setupHttpProxy(t, ignores)
		target := test.YamlToUnstructured(testdata.TargetHTTPProxy)
		f.comparisonResult.reconciliationResult.Target = []*unstructured.Unstructured{target}

		// when
		patchedTargets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(f.comparisonResult.reconciliationResult.Live))
		require.Equal(t, 1, len(f.comparisonResult.reconciliationResult.Target))
		require.Equal(t, 1, len(patchedTargets))

		// live should have 1 entry
		require.Equal(t, 1, len(dig[[]any](f.comparisonResult.reconciliationResult.Live[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors"})))
		// assert some arbitrary field to show `entries[0]` is not an empty object
		require.Equal(t, "sample-header", dig[string](f.comparisonResult.reconciliationResult.Live[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries", 0, "requestHeader", "headerName"}))

		// target has 2 entries
		require.Equal(t, 2, len(dig[[]any](f.comparisonResult.reconciliationResult.Target[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries"})))
		// assert some arbitrary field to show `entries[0]` is not an empty object
		require.Equal(t, "sample-header", dig[string](f.comparisonResult.reconciliationResult.Target[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries", 0, "requestHeaderValueMatch", "headers", 0, "name"}))

		// It should be *1* entries in the array
		require.Equal(t, 1, len(dig[[]any](patchedTargets[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors"})))
		// and it should NOT equal an empty object
		require.Len(t, dig[any](patchedTargets[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries", 0}), 1)

	})
	t.Run("will correctly set array entries if new entries have been added", func(t *testing.T) {
		// given
		ignores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:             "apps",
				Kind:              "Deployment",
				JQPathExpressions: []string{".spec.template.spec.containers[].env[] | select(.name == \"SOME_ENV_VAR\")"},
			},
		}
		f := setupHttpProxy(t, ignores)
		live := test.YamlToUnstructured(testdata.LiveDeploymentEnvVarsYaml)
		target := test.YamlToUnstructured(testdata.TargetDeploymentEnvVarsYaml)
		f.comparisonResult.reconciliationResult.Live = []*unstructured.Unstructured{live}
		f.comparisonResult.reconciliationResult.Target = []*unstructured.Unstructured{target}

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		containers, ok, err := unstructured.NestedSlice(targets[0].Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, 1, len(containers))

		ports := containers[0].(map[string]interface{})["ports"].([]interface{})
		assert.Equal(t, 1, len(ports))

		env := containers[0].(map[string]interface{})["env"].([]interface{})
		assert.Equal(t, 3, len(env))

		first := env[0]
		second := env[1]
		third := env[2]

		// Currently the defined order at this time is the insertion order of the target manifest.
		assert.Equal(t, "SOME_ENV_VAR", first.(map[string]interface{})["name"])
		assert.Equal(t, "some_value", first.(map[string]interface{})["value"])

		assert.Equal(t, "SOME_OTHER_ENV_VAR", second.(map[string]interface{})["name"])
		assert.Equal(t, "some_other_value", second.(map[string]interface{})["value"])

		assert.Equal(t, "YET_ANOTHER_ENV_VAR", third.(map[string]interface{})["name"])
		assert.Equal(t, "yet_another_value", third.(map[string]interface{})["value"])
	})

	t.Run("ignore-deployment-image-replicas-changes-additive", func(t *testing.T) {
		// given

		ignores := []v1alpha1.ResourceIgnoreDifferences{
			{
				Group:        "apps",
				Kind:         "Deployment",
				JSONPointers: []string{"/spec/replicas"},
			}, {
				Group:             "apps",
				Kind:              "Deployment",
				JQPathExpressions: []string{".spec.template.spec.containers[].image"},
			},
		}
		f := setupHttpProxy(t, ignores)
		live := test.YamlToUnstructured(testdata.MinimalImageReplicaDeploymentYaml)
		target := test.YamlToUnstructured(testdata.AdditionalImageReplicaDeploymentYaml)
		f.comparisonResult.reconciliationResult.Live = []*unstructured.Unstructured{live}
		f.comparisonResult.reconciliationResult.Target = []*unstructured.Unstructured{target}

		// when
		targets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Equal(t, 1, len(targets))
		metadata, ok, err := unstructured.NestedMap(targets[0].Object, "metadata")
		require.NoError(t, err)
		require.True(t, ok)
		labels, ok := metadata["labels"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(labels))
		assert.Equal(t, "web", labels["appProcess"])

		spec, ok, err := unstructured.NestedMap(targets[0].Object, "spec")
		require.NoError(t, err)
		require.True(t, ok)

		assert.Equal(t, int64(1), spec["replicas"])

		template, ok := spec["template"].(map[string]interface{})
		require.True(t, ok)

		tMetadata, ok := template["metadata"].(map[string]interface{})
		require.True(t, ok)
		tLabels, ok := tMetadata["labels"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, 2, len(tLabels))
		assert.Equal(t, "web", tLabels["appProcess"])

		tSpec, ok := template["spec"].(map[string]interface{})
		require.True(t, ok)
		containers, ok, err := unstructured.NestedSlice(tSpec, "containers")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, 1, len(containers))

		first := containers[0].(map[string]interface{})
		assert.Equal(t, "alpine:3", first["image"])

		resources, ok := first["resources"].(map[string]interface{})
		require.True(t, ok)
		requests, ok := resources["requests"].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "400m", requests["cpu"])

		env, ok, err := unstructured.NestedSlice(first, "env")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, 1, len(env))

		env0 := env[0].(map[string]interface{})
		assert.Equal(t, "EV", env0["name"])
		assert.Equal(t, "here", env0["value"])
	})
}

func dig[T any](obj interface{}, path []interface{}) T {
	i := obj

	for _, segment := range path {
		switch segment.(type) {
		case int:
			i = i.([]interface{})[segment.(int)]
		case string:
			i = i.(map[string]interface{})[segment.(string)]
		default:
			panic("invalid path for object")
		}
	}

	return i.(T)
}
