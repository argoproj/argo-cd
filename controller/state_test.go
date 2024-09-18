package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"github.com/imdario/mergo"
	"github.com/sirupsen/logrus"
	logrustest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/v2/common"
	"github.com/argoproj/argo-cd/v2/controller/testdata"
	"github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	argoappv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/v2/reposerver/apiclient"
	mockrepoclient "github.com/argoproj/argo-cd/v2/reposerver/apiclient/mocks"
	"github.com/argoproj/argo-cd/v2/test"
	"github.com/argoproj/argo-cd/v2/util/argo"
)

// TestCompareAppStateEmpty tests comparison when both git and live have no objects
func TestCompareAppStateEmpty(t *testing.T) {
	app := newFakeApp()
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Empty(t, compRes.resources)
	assert.Empty(t, compRes.managedResources)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateRepoError tests the case when CompareAppState notices a repo error
func TestCompareAppStateRepoError(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{manifestResponses: make([]*apiclient.ManifestResponse, 3)}, fmt.Errorf("test repo error"))
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	assert.Nil(t, compRes)
	require.EqualError(t, err, CompareStateRepoError.Error())

	// expect to still get compare state error to as inside grace period
	compRes, err = ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	assert.Nil(t, compRes)
	require.EqualError(t, err, CompareStateRepoError.Error())

	time.Sleep(10 * time.Second)
	// expect to not get error as outside of grace period, but status should be unknown
	compRes, err = ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	assert.NotNil(t, compRes)
	require.NoError(t, err)
	assert.Equal(t, argoappv1.SyncStatusCodeUnknown, compRes.syncStatus.Status)
}

// TestCompareAppStateNamespaceMetadataDiffers tests comparison when managed namespace metadata differs
func TestCompareAppStateNamespaceMetadataDiffers(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &argoappv1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	app.Status.OperationState = &argoappv1.OperationState{
		SyncResult: &argoappv1.SyncOperationResult{},
	}

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Empty(t, compRes.resources)
	assert.Empty(t, compRes.managedResources)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateNamespaceMetadataDiffers tests comparison when managed namespace metadata differs to live and manifest ns
func TestCompareAppStateNamespaceMetadataDiffersToManifest(t *testing.T) {
	ns := NewNamespace()
	ns.SetName(test.FakeDestNamespace)
	ns.SetNamespace(test.FakeDestNamespace)
	ns.SetAnnotations(map[string]string{"bar": "bat"})

	app := newFakeApp()
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &argoappv1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	app.Status.OperationState = &argoappv1.OperationState{
		SyncResult: &argoappv1.SyncOperationResult{},
	}

	liveNs := ns.DeepCopy()
	liveNs.SetAnnotations(nil)

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{toJSON(t, liveNs)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(ns): ns,
		},
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.NotNil(t, compRes.diffResultList)
	assert.Len(t, compRes.diffResultList.Diffs, 1)

	result := NewNamespace()
	require.NoError(t, json.Unmarshal(compRes.diffResultList.Diffs[0].PredictedLive, result))

	labels := result.GetLabels()
	delete(labels, "kubernetes.io/metadata.name")

	assert.Equal(t, map[string]string{}, labels)
	// Manifests override definitions in managedNamespaceMetadata
	assert.Equal(t, map[string]string{"bar": "bat"}, result.GetAnnotations())
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateNamespaceMetadata tests comparison when managed namespace metadata differs to live
func TestCompareAppStateNamespaceMetadata(t *testing.T) {
	ns := NewNamespace()
	ns.SetName(test.FakeDestNamespace)
	ns.SetNamespace(test.FakeDestNamespace)
	ns.SetAnnotations(map[string]string{"bar": "bat"})

	app := newFakeApp()
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &argoappv1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	app.Status.OperationState = &argoappv1.OperationState{
		SyncResult: &argoappv1.SyncOperationResult{},
	}

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(ns): ns,
		},
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.NotNil(t, compRes.diffResultList)
	assert.Len(t, compRes.diffResultList.Diffs, 1)

	result := NewNamespace()
	require.NoError(t, json.Unmarshal(compRes.diffResultList.Diffs[0].PredictedLive, result))

	labels := result.GetLabels()
	delete(labels, "kubernetes.io/metadata.name")

	assert.Equal(t, map[string]string{"foo": "bar"}, labels)
	assert.Equal(t, map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true", "bar": "bat", "foo": "bar"}, result.GetAnnotations())
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateNamespaceMetadataIsTheSame tests comparison when managed namespace metadata is the same
func TestCompareAppStateNamespaceMetadataIsTheSame(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy.ManagedNamespaceMetadata = &argoappv1.ManagedNamespaceMetadata{
		Labels: map[string]string{
			"foo": "bar",
		},
		Annotations: map[string]string{
			"foo": "bar",
		},
	}
	app.Status.OperationState = &argoappv1.OperationState{
		SyncResult: &argoappv1.SyncOperationResult{
			ManagedNamespaceMetadata: &argoappv1.ManagedNamespaceMetadata{
				Labels: map[string]string{
					"foo": "bar",
				},
				Annotations: map[string]string{
					"foo": "bar",
				},
			},
		},
	}

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Empty(t, compRes.resources)
	assert.Empty(t, compRes.managedResources)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateMissing tests when there is a manifest defined in the repo which doesn't exist in live
func TestCompareAppStateMissing(t *testing.T) {
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{PodManifest},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateExtra tests when there is an extra object in live but not defined in git
func TestCompareAppStateExtra(t *testing.T) {
	pod := NewPod()
	pod.SetNamespace(test.FakeDestNamespace)
	app := newFakeApp()
	key := kube.ResourceKey{Group: "", Kind: "Pod", Namespace: test.FakeDestNamespace, Name: app.Name}
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			key: pod,
		},
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateHook checks that hooks are detected during manifest generation, and not
// considered as part of resources when assessing Synced status
func TestCompareAppStateHook(t *testing.T) {
	pod := NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	podBytes, _ := json.Marshal(pod)
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{string(podBytes)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Empty(t, compRes.resources)
	assert.Empty(t, compRes.managedResources)
	assert.Len(t, compRes.reconciliationResult.Hooks, 1)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateSkipHook checks that skipped resources are detected during manifest generation, and not
// considered as part of resources when assessing Synced status
func TestCompareAppStateSkipHook(t *testing.T) {
	pod := NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "Skip"})
	podBytes, _ := json.Marshal(pod)
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{string(podBytes)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.Empty(t, compRes.reconciliationResult.Hooks)
	assert.Empty(t, app.Status.Conditions)
}

// checks that ignore resources are detected, but excluded from status
func TestCompareAppStateCompareOptionIgnoreExtraneous(t *testing.T) {
	pod := NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationCompareOptions: "IgnoreExtraneous"})
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)

	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Empty(t, compRes.resources)
	assert.Empty(t, compRes.managedResources)
	assert.Empty(t, app.Status.Conditions)
}

// TestCompareAppStateExtraHook tests when there is an extra _hook_ object in live but not defined in git
func TestCompareAppStateExtraHook(t *testing.T) {
	pod := NewPod()
	pod.SetAnnotations(map[string]string{synccommon.AnnotationKeyHook: "PreSync"})
	pod.SetNamespace(test.FakeDestNamespace)
	app := newFakeApp()
	key := kube.ResourceKey{Group: "", Kind: "Pod", Namespace: test.FakeDestNamespace, Name: app.Name}
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			key: pod,
		},
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.Empty(t, compRes.reconciliationResult.Hooks)
	assert.Empty(t, app.Status.Conditions)
}

// TestAppRevisions tests that revisions are properly propagated for a single source app
func TestAppRevisionsSingleSource(t *testing.T) {
	obj1 := NewPod()
	obj1.SetNamespace(test.FakeDestNamespace)
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{toJSON(t, obj1)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)

	app := newFakeApp()
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, app.Spec.GetSources(), false, false, nil, app.Spec.HasMultipleSources(), false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.NotEmpty(t, compRes.syncStatus.Revision)
	assert.Empty(t, compRes.syncStatus.Revisions)
}

// TestAppRevisions tests that revisions are properly propagated for a multi source app
func TestAppRevisionsMultiSource(t *testing.T) {
	obj1 := NewPod()
	obj1.SetNamespace(test.FakeDestNamespace)
	data := fakeData{
		manifestResponses: []*apiclient.ManifestResponse{
			{
				Manifests: []string{toJSON(t, obj1)},
				Namespace: test.FakeDestNamespace,
				Server:    test.FakeClusterURL,
				Revision:  "abc123",
			},
			{
				Manifests: []string{toJSON(t, obj1)},
				Namespace: test.FakeDestNamespace,
				Server:    test.FakeClusterURL,
				Revision:  "def456",
			},
			{
				Manifests: []string{},
				Namespace: test.FakeDestNamespace,
				Server:    test.FakeClusterURL,
				Revision:  "ghi789",
			},
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)

	app := newFakeMultiSourceApp()
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, app.Spec.GetSources(), false, false, nil, app.Spec.HasMultipleSources(), false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Empty(t, compRes.syncStatus.Revision)
	assert.Len(t, compRes.syncStatus.Revisions, 3)
	assert.Equal(t, "abc123", compRes.syncStatus.Revisions[0])
	assert.Equal(t, "def456", compRes.syncStatus.Revisions[1])
	assert.Equal(t, "ghi789", compRes.syncStatus.Revisions[2])
}

func toJSON(t *testing.T, obj *unstructured.Unstructured) string {
	data, err := json.Marshal(obj)
	require.NoError(t, err)
	return string(data)
}

func TestCompareAppStateDuplicatedNamespacedResources(t *testing.T) {
	obj1 := NewPod()
	obj1.SetNamespace(test.FakeDestNamespace)
	obj2 := NewPod()
	obj3 := NewPod()
	obj3.SetNamespace("kube-system")
	obj4 := NewPod()
	obj4.SetGenerateName("my-pod")
	obj4.SetName("")
	obj5 := NewPod()
	obj5.SetName("")
	obj5.SetGenerateName("my-pod")

	app := newFakeApp()
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{toJSON(t, obj1), toJSON(t, obj2), toJSON(t, obj3), toJSON(t, obj4), toJSON(t, obj5)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(obj1): obj1,
			kube.GetResourceKey(obj3): obj3,
		},
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.NotNil(t, compRes)
	assert.Len(t, app.Status.Conditions, 1)
	assert.NotNil(t, app.Status.Conditions[0].LastTransitionTime)
	assert.Equal(t, argoappv1.ApplicationConditionRepeatedResourceWarning, app.Status.Conditions[0].Type)
	assert.Equal(t, "Resource /Pod/fake-dest-ns/my-pod appeared 2 times among application resources.", app.Status.Conditions[0].Message)
	assert.Len(t, compRes.resources, 4)
}

func TestCompareAppStateManagedNamespaceMetadataWithLiveNsDoesNotGetPruned(t *testing.T) {
	app := newFakeApp()
	app.Spec.SyncPolicy = &argoappv1.SyncPolicy{
		ManagedNamespaceMetadata: &argoappv1.ManagedNamespaceMetadata{
			Labels:      nil,
			Annotations: nil,
		},
	}

	ns := NewNamespace()
	ns.SetName(test.FakeDestNamespace)
	ns.SetNamespace(test.FakeDestNamespace)
	ns.SetAnnotations(map[string]string{"argocd.argoproj.io/sync-options": "ServerSideApply=true"})

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(ns): ns,
		},
	}
	ctrl := newFakeController(&data, nil)
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, []string{}, app.Spec.Sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.NotNil(t, compRes)
	assert.Empty(t, app.Status.Conditions)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	// Ensure that ns does not get pruned
	assert.NotNil(t, compRes.reconciliationResult.Target[0])
	assert.Equal(t, compRes.reconciliationResult.Target[0].GetName(), ns.GetName())
	assert.Equal(t, compRes.reconciliationResult.Target[0].GetAnnotations(), ns.GetAnnotations())
	assert.Equal(t, compRes.reconciliationResult.Target[0].GetLabels(), ns.GetLabels())
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
}

var defaultProj = argoappv1.AppProject{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "default",
		Namespace: test.FakeArgoCDNamespace,
	},
	Spec: argoappv1.AppProjectSpec{
		SourceRepos: []string{"*"},
		Destinations: []argoappv1.ApplicationDestination{
			{
				Server:    "*",
				Namespace: "*",
			},
		},
	},
}

// TestCompareAppStateWithManifestGeneratePath tests that it compares revisions when the manifest-generate-path annotation is set.
func TestCompareAppStateWithManifestGeneratePath(t *testing.T) {
	app := newFakeApp()
	app.SetAnnotations(map[string]string{argoappv1.AnnotationKeyManifestGeneratePaths: "."})
	app.Status.Sync = argoappv1.SyncStatus{
		Revision: "abc123",
		Status:   argoappv1.SyncStatusCodeSynced,
	}

	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		updateRevisionForPathsResponse: &apiclient.UpdateRevisionForPathsResponse{},
	}

	ctrl := newFakeController(&data, nil)
	revisions := make([]string, 0)
	revisions = append(revisions, "abc123")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, app.Spec.GetSources(), false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, "abc123", compRes.syncStatus.Revision)
	ctrl.repoClientset.(*mockrepoclient.Clientset).RepoServerServiceClient.(*mockrepoclient.RepoServerServiceClient).AssertNumberOfCalls(t, "UpdateRevisionForPaths", 1)
}

func TestSetHealth(t *testing.T) {
	app := newFakeApp()
	deployment := kube.MustToUnstructured(&v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
	})
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(deployment): deployment,
		},
	}, nil)

	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.Equal(t, health.HealthStatusHealthy, compRes.healthStatus.Status)
}

func TestSetHealthSelfReferencedApp(t *testing.T) {
	app := newFakeApp()
	unstructuredApp := kube.MustToUnstructured(app)
	deployment := kube.MustToUnstructured(&v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
	})
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(deployment):      deployment,
			kube.GetResourceKey(unstructuredApp): unstructuredApp,
		},
	}, nil)

	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.Equal(t, health.HealthStatusHealthy, compRes.healthStatus.Status)
}

func TestSetManagedResourcesWithOrphanedResources(t *testing.T) {
	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &argoappv1.OrphanedResourcesMonitorSettings{}

	app := newFakeApp()
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, proj},
		namespacedResources: map[kube.ResourceKey]namespacedResource{
			kube.NewResourceKey("apps", kube.DeploymentKind, app.Namespace, "guestbook"): {
				ResourceNode: argoappv1.ResourceNode{
					ResourceRef: argoappv1.ResourceRef{Kind: kube.DeploymentKind, Name: "guestbook", Namespace: app.Namespace},
				},
				AppName: "",
			},
		},
	}, nil)

	tree, err := ctrl.setAppManagedResources(app, &comparisonResult{managedResources: make([]managedResource, 0)})

	require.NoError(t, err)
	assert.Len(t, tree.OrphanedNodes, 1)
	assert.Equal(t, "guestbook", tree.OrphanedNodes[0].Name)
	assert.Equal(t, app.Namespace, tree.OrphanedNodes[0].Namespace)
}

func TestSetManagedResourcesWithResourcesOfAnotherApp(t *testing.T) {
	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &argoappv1.OrphanedResourcesMonitorSettings{}

	app1 := newFakeApp()
	app1.Name = "app1"
	app2 := newFakeApp()
	app2.Name = "app2"

	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app1, app2, proj},
		namespacedResources: map[kube.ResourceKey]namespacedResource{
			kube.NewResourceKey("apps", kube.DeploymentKind, app2.Namespace, "guestbook"): {
				ResourceNode: argoappv1.ResourceNode{
					ResourceRef: argoappv1.ResourceRef{Kind: kube.DeploymentKind, Name: "guestbook", Namespace: app2.Namespace},
				},
				AppName: "app2",
			},
		},
	}, nil)

	tree, err := ctrl.setAppManagedResources(app1, &comparisonResult{managedResources: make([]managedResource, 0)})

	require.NoError(t, err)
	assert.Empty(t, tree.OrphanedNodes)
}

func TestReturnUnknownComparisonStateOnSettingLoadError(t *testing.T) {
	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &argoappv1.OrphanedResourcesMonitorSettings{}

	app := newFakeApp()

	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, proj},
		configMapData: map[string]string{
			"resource.customizations": "invalid setting",
		},
	}, nil)

	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)

	assert.Equal(t, health.HealthStatusUnknown, compRes.healthStatus.Status)
	assert.Equal(t, argoappv1.SyncStatusCodeUnknown, compRes.syncStatus.Status)
}

func TestSetManagedResourcesKnownOrphanedResourceExceptions(t *testing.T) {
	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &argoappv1.OrphanedResourcesMonitorSettings{}
	proj.Spec.SourceNamespaces = []string{"default"}

	app := newFakeApp()
	app.Namespace = "default"

	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, proj},
		namespacedResources: map[kube.ResourceKey]namespacedResource{
			kube.NewResourceKey("apps", kube.DeploymentKind, app.Namespace, "guestbook"): {
				ResourceNode: argoappv1.ResourceNode{ResourceRef: argoappv1.ResourceRef{Group: "apps", Kind: kube.DeploymentKind, Name: "guestbook", Namespace: app.Namespace}},
			},
			kube.NewResourceKey("", kube.ServiceAccountKind, app.Namespace, "default"): {
				ResourceNode: argoappv1.ResourceNode{ResourceRef: argoappv1.ResourceRef{Kind: kube.ServiceAccountKind, Name: "default", Namespace: app.Namespace}},
			},
			kube.NewResourceKey("", kube.ServiceKind, app.Namespace, "kubernetes"): {
				ResourceNode: argoappv1.ResourceNode{ResourceRef: argoappv1.ResourceRef{Kind: kube.ServiceAccountKind, Name: "kubernetes", Namespace: app.Namespace}},
			},
		},
	}, nil)

	tree, err := ctrl.setAppManagedResources(app, &comparisonResult{managedResources: make([]managedResource, 0)})

	require.NoError(t, err)
	assert.Len(t, tree.OrphanedNodes, 1)
	assert.Equal(t, "guestbook", tree.OrphanedNodes[0].Name)
}

func Test_appStateManager_persistRevisionHistory(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app},
	}, nil)
	manager := ctrl.appStateManager.(*appStateManager)
	setRevisionHistoryLimit := func(value int) {
		i := int64(value)
		app.Spec.RevisionHistoryLimit = &i
	}
	addHistory := func() {
		err := manager.persistRevisionHistory(app, "my-revision", argoappv1.ApplicationSource{}, []string{}, []argoappv1.ApplicationSource{}, false, metav1.Time{}, v1alpha1.OperationInitiator{})
		require.NoError(t, err)
	}
	addHistory()
	assert.Len(t, app.Status.History, 1)
	addHistory()
	assert.Len(t, app.Status.History, 2)
	addHistory()
	assert.Len(t, app.Status.History, 3)
	addHistory()
	assert.Len(t, app.Status.History, 4)
	addHistory()
	assert.Len(t, app.Status.History, 5)
	addHistory()
	assert.Len(t, app.Status.History, 6)
	addHistory()
	assert.Len(t, app.Status.History, 7)
	addHistory()
	assert.Len(t, app.Status.History, 8)
	addHistory()
	assert.Len(t, app.Status.History, 9)
	addHistory()
	assert.Len(t, app.Status.History, 10)
	// default limit is 10
	addHistory()
	assert.Len(t, app.Status.History, 10)
	// increase limit
	setRevisionHistoryLimit(11)
	addHistory()
	assert.Len(t, app.Status.History, 11)
	// decrease limit
	setRevisionHistoryLimit(9)
	addHistory()
	assert.Len(t, app.Status.History, 9)

	metav1NowTime := metav1.NewTime(time.Now())
	err := manager.persistRevisionHistory(app, "my-revision", argoappv1.ApplicationSource{}, []string{}, []argoappv1.ApplicationSource{}, false, metav1NowTime, v1alpha1.OperationInitiator{})
	require.NoError(t, err)
	assert.Equal(t, app.Status.History.LastRevisionHistory().DeployStartedAt, &metav1NowTime)
}

// helper function to read contents of a file to string
// panics on error
func mustReadFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		panic(err.Error())
	}
	return string(b)
}

var signedProj = argoappv1.AppProject{
	ObjectMeta: metav1.ObjectMeta{
		Name:      "default",
		Namespace: test.FakeArgoCDNamespace,
	},
	Spec: argoappv1.AppProjectSpec{
		SourceRepos: []string{"*"},
		Destinations: []argoappv1.ApplicationDestination{
			{
				Server:    "*",
				Namespace: "*",
			},
		},
		SignatureKeys: []argoappv1.SignatureKey{
			{
				KeyID: "4AEE18F83AFDEB23",
			},
		},
	},
}

func TestSignedResponseNoSignatureRequired(t *testing.T) {
	t.Setenv("ARGOCD_GPG_ENABLED", "true")

	// We have a good signature response, but project does not require signed commits
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/good_signature.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Empty(t, app.Status.Conditions)
	}
	// We have a bad signature response, but project does not require signed commits
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/bad_signature_bad.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Empty(t, app.Status.Conditions)
	}
}

func TestSignedResponseSignatureRequired(t *testing.T) {
	t.Setenv("ARGOCD_GPG_ENABLED", "true")

	// We have a good signature response, valid key, and signing is required - sync!
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/good_signature.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Empty(t, app.Status.Conditions)
	}
	// We have a bad signature response and signing is required - do not sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/bad_signature_bad.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Len(t, app.Status.Conditions, 1)
	}
	// We have a malformed signature response and signing is required - do not sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/bad_signature_malformed1.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Len(t, app.Status.Conditions, 1)
	}
	// We have no signature response (no signature made) and signing is required - do not sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: "",
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Len(t, app.Status.Conditions, 1)
	}

	// We have a good signature and signing is required, but key is not allowed - do not sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/good_signature.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		testProj := signedProj
		testProj.Spec.SignatureKeys[0].KeyID = "4AEE18F83AFDEB24"
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &testProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Len(t, app.Status.Conditions, 1)
		assert.Contains(t, app.Status.Conditions[0].Message, "key is not allowed")
	}
	// Signature required and local manifests supplied - do not sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: "",
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		// it doesn't matter for our test whether local manifests are valid
		localManifests := []string{"foobar"}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, localManifests, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeUnknown, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Len(t, app.Status.Conditions, 1)
		assert.Contains(t, app.Status.Conditions[0].Message, "Cannot use local manifests")
	}

	t.Setenv("ARGOCD_GPG_ENABLED", "false")
	// We have a bad signature response and signing would be required, but GPG subsystem is disabled - sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: mustReadFile("../util/gpg/testdata/bad_signature_bad.txt"),
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, nil, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Empty(t, app.Status.Conditions)
	}

	// Signature required and local manifests supplied and GPG subsystem is disabled - sync
	{
		app := newFakeApp()
		data := fakeData{
			manifestResponse: &apiclient.ManifestResponse{
				Manifests:    []string{},
				Namespace:    test.FakeDestNamespace,
				Server:       test.FakeClusterURL,
				Revision:     "abc123",
				VerifyResult: "",
			},
			managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
		}
		// it doesn't matter for our test whether local manifests are valid
		localManifests := []string{""}
		ctrl := newFakeController(&data, nil)
		sources := make([]argoappv1.ApplicationSource, 0)
		sources = append(sources, app.Spec.GetSource())
		revisions := make([]string, 0)
		revisions = append(revisions, "abc123")
		compRes, err := ctrl.appStateManager.CompareAppState(app, &signedProj, revisions, sources, false, false, localManifests, false, false)
		require.NoError(t, err)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Empty(t, compRes.resources)
		assert.Empty(t, compRes.managedResources)
		assert.Empty(t, app.Status.Conditions)
	}
}

func TestComparisonResult_GetHealthStatus(t *testing.T) {
	status := &argoappv1.HealthStatus{Status: health.HealthStatusMissing}
	res := comparisonResult{
		healthStatus: status,
	}

	assert.Equal(t, status, res.GetHealthStatus())
}

func TestComparisonResult_GetSyncStatus(t *testing.T) {
	status := &argoappv1.SyncStatus{Status: argoappv1.SyncStatusCodeOutOfSync}
	res := comparisonResult{
		syncStatus: status,
	}

	assert.Equal(t, status, res.GetSyncStatus())
}

func TestIsLiveResourceManaged(t *testing.T) {
	managedObj := kube.MustToUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap1",
			Namespace: "default",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: "guestbook:/ConfigMap:default/configmap1",
			},
		},
	})
	managedObjWithLabel := kube.MustToUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap1",
			Namespace: "default",
			Labels: map[string]string{
				common.LabelKeyAppInstance: "guestbook",
			},
		},
	})
	unmanagedObjWrongName := kube.MustToUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap2",
			Namespace: "default",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: "guestbook:/ConfigMap:default/configmap1",
			},
		},
	})
	unmanagedObjWrongKind := kube.MustToUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap2",
			Namespace: "default",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: "guestbook:/Service:default/configmap2",
			},
		},
	})
	unmanagedObjWrongGroup := kube.MustToUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap2",
			Namespace: "default",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: "guestbook:apps/ConfigMap:default/configmap2",
			},
		},
	})
	unmanagedObjWrongNamespace := kube.MustToUnstructured(&corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "configmap2",
			Namespace: "default",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: "guestbook:/ConfigMap:fakens/configmap2",
			},
		},
	})
	managedWrongAPIGroup := kube.MustToUnstructured(&networkingv1.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking.k8s.io/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-ingress",
			Namespace: "default",
			Annotations: map[string]string{
				common.AnnotationKeyAppInstance: "guestbook:extensions/Ingress:default/some-ingress",
			},
		},
	})
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(managedObj):                 managedObj,
			kube.GetResourceKey(unmanagedObjWrongName):      unmanagedObjWrongName,
			kube.GetResourceKey(unmanagedObjWrongKind):      unmanagedObjWrongKind,
			kube.GetResourceKey(unmanagedObjWrongGroup):     unmanagedObjWrongGroup,
			kube.GetResourceKey(unmanagedObjWrongNamespace): unmanagedObjWrongNamespace,
		},
	}, nil)

	manager := ctrl.appStateManager.(*appStateManager)
	appName := "guestbook"

	t.Run("will return true if trackingid matches the resource", func(t *testing.T) {
		// given
		t.Parallel()
		configObj := managedObj.DeepCopy()

		// then
		assert.True(t, manager.isSelfReferencedObj(managedObj, configObj, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodLabel))
		assert.True(t, manager.isSelfReferencedObj(managedObj, configObj, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotation))
	})
	t.Run("will return true if tracked with label", func(t *testing.T) {
		// given
		t.Parallel()
		configObj := managedObjWithLabel.DeepCopy()

		// then
		assert.True(t, manager.isSelfReferencedObj(managedObjWithLabel, configObj, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodLabel))
	})
	t.Run("will handle if trackingId has wrong resource name and config is nil", func(t *testing.T) {
		// given
		t.Parallel()

		// then
		assert.True(t, manager.isSelfReferencedObj(unmanagedObjWrongName, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodLabel))
		assert.False(t, manager.isSelfReferencedObj(unmanagedObjWrongName, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotation))
	})
	t.Run("will handle if trackingId has wrong resource group and config is nil", func(t *testing.T) {
		// given
		t.Parallel()

		// then
		assert.True(t, manager.isSelfReferencedObj(unmanagedObjWrongGroup, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodLabel))
		assert.False(t, manager.isSelfReferencedObj(unmanagedObjWrongGroup, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotation))
	})
	t.Run("will handle if trackingId has wrong kind and config is nil", func(t *testing.T) {
		// given
		t.Parallel()

		// then
		assert.True(t, manager.isSelfReferencedObj(unmanagedObjWrongKind, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodLabel))
		assert.False(t, manager.isSelfReferencedObj(unmanagedObjWrongKind, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotation))
	})
	t.Run("will handle if trackingId has wrong namespace and config is nil", func(t *testing.T) {
		// given
		t.Parallel()

		// then
		assert.True(t, manager.isSelfReferencedObj(unmanagedObjWrongNamespace, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodLabel))
		assert.False(t, manager.isSelfReferencedObj(unmanagedObjWrongNamespace, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotationAndLabel))
	})
	t.Run("will return true if live is nil", func(t *testing.T) {
		t.Parallel()
		assert.True(t, manager.isSelfReferencedObj(nil, nil, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotation))
	})

	t.Run("will handle upgrade in desired state APIGroup", func(t *testing.T) {
		// given
		t.Parallel()
		config := managedWrongAPIGroup.DeepCopy()
		delete(config.GetAnnotations(), common.AnnotationKeyAppInstance)

		// then
		assert.True(t, manager.isSelfReferencedObj(managedWrongAPIGroup, config, appName, common.AnnotationKeyAppInstance, argo.TrackingMethodAnnotation))
	})
}

func TestUseDiffCache(t *testing.T) {
	type fixture struct {
		testName             string
		noCache              bool
		manifestInfos        []*apiclient.ManifestResponse
		sources              []argoappv1.ApplicationSource
		app                  *argoappv1.Application
		manifestRevisions    []string
		statusRefreshTimeout time.Duration
		expectedUseCache     bool
		serverSideDiff       bool
	}

	manifestInfos := func(revision string) []*apiclient.ManifestResponse {
		return []*apiclient.ManifestResponse{
			{
				Manifests: []string{
					"{\"apiVersion\":\"v1\",\"kind\":\"Service\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"httpbin\"},\"name\":\"httpbin-svc\",\"namespace\":\"httpbin\"},\"spec\":{\"ports\":[{\"name\":\"http-port\",\"port\":7777,\"targetPort\":80},{\"name\":\"test\",\"port\":333}],\"selector\":{\"app\":\"httpbin\"}}}",
					"{\"apiVersion\":\"apps/v1\",\"kind\":\"Deployment\",\"metadata\":{\"labels\":{\"app.kubernetes.io/instance\":\"httpbin\"},\"name\":\"httpbin-deployment\",\"namespace\":\"httpbin\"},\"spec\":{\"replicas\":2,\"selector\":{\"matchLabels\":{\"app\":\"httpbin\"}},\"template\":{\"metadata\":{\"labels\":{\"app\":\"httpbin\"}},\"spec\":{\"containers\":[{\"image\":\"kennethreitz/httpbin\",\"imagePullPolicy\":\"Always\",\"name\":\"httpbin\",\"ports\":[{\"containerPort\":80}]}]}}}}",
				},
				Namespace:    "",
				Server:       "",
				Revision:     revision,
				SourceType:   "Kustomize",
				VerifyResult: "",
			},
		}
	}
	sources := func() []argoappv1.ApplicationSource {
		return []argoappv1.ApplicationSource{
			{
				RepoURL:        "https://some-repo.com",
				Path:           "argocd/httpbin",
				TargetRevision: "HEAD",
			},
		}
	}

	app := func(namespace string, revision string, refresh bool, a *argoappv1.Application) *argoappv1.Application {
		app := &argoappv1.Application{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "httpbin",
				Namespace: namespace,
			},
			Spec: argoappv1.ApplicationSpec{
				Source: &argoappv1.ApplicationSource{
					RepoURL:        "https://some-repo.com",
					Path:           "argocd/httpbin",
					TargetRevision: "HEAD",
				},
				Destination: argoappv1.ApplicationDestination{
					Server:    "https://kubernetes.default.svc",
					Namespace: "httpbin",
				},
				Project: "default",
				SyncPolicy: &argoappv1.SyncPolicy{
					SyncOptions: []string{
						"CreateNamespace=true",
						"ServerSideApply=true",
					},
				},
			},
			Status: argoappv1.ApplicationStatus{
				Resources: []argoappv1.ResourceStatus{},
				Sync: argoappv1.SyncStatus{
					Status: argoappv1.SyncStatusCodeSynced,
					ComparedTo: argoappv1.ComparedTo{
						Source: argoappv1.ApplicationSource{
							RepoURL:        "https://some-repo.com",
							Path:           "argocd/httpbin",
							TargetRevision: "HEAD",
						},
						Destination: argoappv1.ApplicationDestination{
							Server:    "https://kubernetes.default.svc",
							Namespace: "httpbin",
						},
					},
					Revision:  revision,
					Revisions: []string{},
				},
				ReconciledAt: &metav1.Time{
					Time: time.Now().Add(-time.Hour),
				},
			},
		}
		if refresh {
			annotations := make(map[string]string)
			annotations[argoappv1.AnnotationKeyRefresh] = string(argoappv1.RefreshTypeNormal)
			app.SetAnnotations(annotations)
		}
		if a != nil {
			err := mergo.Merge(app, a, mergo.WithOverride, mergo.WithOverwriteWithEmptyValue)
			if err != nil {
				t.Fatalf("error merging app: %s", err)
			}
		}
		return app
	}

	cases := []fixture{
		{
			testName:             "will use diff cache",
			noCache:              false,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  app("httpbin", "rev1", false, nil),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     true,
			serverSideDiff:       false,
		},
		{
			testName:             "will use diff cache with sync policy",
			noCache:              false,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  test.YamlToApplication(testdata.DiffCacheYaml),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     true,
			serverSideDiff:       true,
		},
		{
			testName:      "will use diff cache for multisource",
			noCache:       false,
			manifestInfos: manifestInfos("rev1"),
			sources:       sources(),
			app: app("httpbin", "", false, &argoappv1.Application{
				Spec: argoappv1.ApplicationSpec{
					Source: nil,
					Sources: argoappv1.ApplicationSources{
						{
							RepoURL: "multisource repo1",
						},
						{
							RepoURL: "multisource repo2",
						},
					},
				},
				Status: argoappv1.ApplicationStatus{
					Resources: []argoappv1.ResourceStatus{},
					Sync: argoappv1.SyncStatus{
						Status: argoappv1.SyncStatusCodeSynced,
						ComparedTo: argoappv1.ComparedTo{
							Source: argoappv1.ApplicationSource{},
							Sources: argoappv1.ApplicationSources{
								{
									RepoURL: "multisource repo1",
								},
								{
									RepoURL: "multisource repo2",
								},
							},
						},
						Revisions: []string{"rev1", "rev2"},
					},
					ReconciledAt: &metav1.Time{
						Time: time.Now().Add(-time.Hour),
					},
				},
			}),
			manifestRevisions:    []string{"rev1", "rev2"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     true,
			serverSideDiff:       false,
		},
		{
			testName:             "will return false if nocache is true",
			noCache:              true,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  app("httpbin", "rev1", false, nil),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     false,
			serverSideDiff:       false,
		},
		{
			testName:             "will return false if requested refresh",
			noCache:              false,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  app("httpbin", "rev1", true, nil),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     false,
			serverSideDiff:       false,
		},
		{
			testName:             "will return false if status expired",
			noCache:              false,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  app("httpbin", "rev1", false, nil),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Minute,
			expectedUseCache:     false,
			serverSideDiff:       false,
		},
		{
			testName:             "will return true if status expired and server-side diff",
			noCache:              false,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  app("httpbin", "rev1", false, nil),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Minute,
			expectedUseCache:     true,
			serverSideDiff:       true,
		},
		{
			testName:             "will return false if there is a new revision",
			noCache:              false,
			manifestInfos:        manifestInfos("rev1"),
			sources:              sources(),
			app:                  app("httpbin", "rev1", false, nil),
			manifestRevisions:    []string{"rev2"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     false,
			serverSideDiff:       false,
		},
		{
			testName:      "will return false if app spec repo changed",
			noCache:       false,
			manifestInfos: manifestInfos("rev1"),
			sources:       sources(),
			app: app("httpbin", "rev1", false, &argoappv1.Application{
				Spec: argoappv1.ApplicationSpec{
					Source: &argoappv1.ApplicationSource{
						RepoURL: "new-repo",
					},
				},
			}),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     false,
			serverSideDiff:       false,
		},
		{
			testName:      "will return false if app spec IgnoreDifferences changed",
			noCache:       false,
			manifestInfos: manifestInfos("rev1"),
			sources:       sources(),
			app: app("httpbin", "rev1", false, &argoappv1.Application{
				Spec: argoappv1.ApplicationSpec{
					IgnoreDifferences: []argoappv1.ResourceIgnoreDifferences{
						{
							Group:             "app/v1",
							Kind:              "application",
							Name:              "httpbin",
							Namespace:         "httpbin",
							JQPathExpressions: []string{"."},
						},
					},
				},
			}),
			manifestRevisions:    []string{"rev1"},
			statusRefreshTimeout: time.Hour * 24,
			expectedUseCache:     false,
			serverSideDiff:       false,
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.testName, func(t *testing.T) {
			// Given
			t.Parallel()
			logger, _ := logrustest.NewNullLogger()
			log := logrus.NewEntry(logger)

			// When
			useDiffCache := useDiffCache(tc.noCache, tc.manifestInfos, tc.sources, tc.app, tc.manifestRevisions, tc.statusRefreshTimeout, tc.serverSideDiff, log)

			// Then
			assert.Equal(t, tc.expectedUseCache, useDiffCache)
		})
	}
}

func TestCompareAppStateDefaultRevisionUpdated(t *testing.T) {
	app := newFakeApp()
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.True(t, compRes.revisionUpdated)
}

func TestCompareAppStateRevisionUpdatedWithHelmSource(t *testing.T) {
	app := newFakeMultiSourceApp()
	data := fakeData{
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data, nil)
	sources := make([]argoappv1.ApplicationSource, 0)
	sources = append(sources, app.Spec.GetSource())
	revisions := make([]string, 0)
	revisions = append(revisions, "")
	compRes, err := ctrl.appStateManager.CompareAppState(app, &defaultProj, revisions, sources, false, false, nil, false, false)
	require.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.True(t, compRes.revisionUpdated)
}
