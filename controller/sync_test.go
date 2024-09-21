package controller

import (
	"context"
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
	require.NoError(t, err)
	assert.Len(t, updatedApp.Status.History, 1)
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
	require.NoError(t, err)
	assert.Len(t, updatedApp.Status.History, 1)
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

		opState := &v1alpha1.OperationState{
			Operation: v1alpha1.Operation{
				Sync: &v1alpha1.SyncOperation{
					Source: &v1alpha1.ApplicationSource{},
				},
			},
			Phase: common.OperationRunning,
		}
		// when
		f.controller.appStateManager.SyncAppState(f.application, opState)

		// then
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
		require.Len(t, targets, 1)
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
		require.Len(t, targets, 1)
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
		require.Len(t, targets, 1)
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
		require.Len(t, targets, 1)
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
		require.Len(t, targets, 1)
		containers, ok, err := unstructured.NestedSlice(targets[0].Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Len(t, containers, 2)
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
				// JSONPointers: []string{"/spec/routes"},
			},
		}
		f := setupHttpProxy(t, ignores)
		target := test.YamlToUnstructured(testdata.TargetHTTPProxy)
		f.comparisonResult.reconciliationResult.Target = []*unstructured.Unstructured{target}

		// when
		patchedTargets, err := normalizeTargetResources(f.comparisonResult)

		// then
		require.NoError(t, err)
		require.Len(t, f.comparisonResult.reconciliationResult.Live, 1)
		require.Len(t, f.comparisonResult.reconciliationResult.Target, 1)
		require.Len(t, patchedTargets, 1)

		// live should have 1 entry
		require.Len(t, dig[[]any](f.comparisonResult.reconciliationResult.Live[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors"}), 1)
		// assert some arbitrary field to show `entries[0]` is not an empty object
		require.Equal(t, "sample-header", dig[string](f.comparisonResult.reconciliationResult.Live[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries", 0, "requestHeader", "headerName"}))

		// target has 2 entries
		require.Len(t, dig[[]any](f.comparisonResult.reconciliationResult.Target[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries"}), 2)
		// assert some arbitrary field to show `entries[0]` is not an empty object
		require.Equal(t, "sample-header", dig[string](f.comparisonResult.reconciliationResult.Target[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors", 0, "entries", 0, "requestHeaderValueMatch", "headers", 0, "name"}))

		// It should be *1* entries in the array
		require.Len(t, dig[[]any](patchedTargets[0].Object, []interface{}{"spec", "routes", 0, "rateLimitPolicy", "global", "descriptors"}), 1)
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
		require.Len(t, targets, 1)
		containers, ok, err := unstructured.NestedSlice(targets[0].Object, "spec", "template", "spec", "containers")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Len(t, containers, 1)

		ports := containers[0].(map[string]interface{})["ports"].([]interface{})
		assert.Len(t, ports, 1)

		env := containers[0].(map[string]interface{})["env"].([]interface{})
		assert.Len(t, env, 3)

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
		require.Len(t, targets, 1)
		metadata, ok, err := unstructured.NestedMap(targets[0].Object, "metadata")
		require.NoError(t, err)
		require.True(t, ok)
		labels, ok := metadata["labels"].(map[string]interface{})
		require.True(t, ok)
		assert.Len(t, labels, 2)
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
		assert.Len(t, tLabels, 2)
		assert.Equal(t, "web", tLabels["appProcess"])

		tSpec, ok := template["spec"].(map[string]interface{})
		require.True(t, ok)
		containers, ok, err := unstructured.NestedSlice(tSpec, "containers")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Len(t, containers, 1)

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
		assert.Len(t, env, 1)

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
