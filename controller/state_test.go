package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/repository"
	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util/kube"
)

// TestCompareAppStateEmpty tests comparison when both git and live have no objects
func TestCompareAppStateEmpty(t *testing.T) {
	app := newFakeApp()
	data := fakeData{
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)
	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, 0, len(compRes.resources))
	assert.Equal(t, 0, len(compRes.managedResources))
	assert.Equal(t, 0, len(compRes.conditions))
}

// TestCompareAppStateMissing tests when there is a manifest defined in git which doesn't exist in live
func TestCompareAppStateMissing(t *testing.T) {
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{string(test.PodManifest)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)
	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Equal(t, 1, len(compRes.resources))
	assert.Equal(t, 1, len(compRes.managedResources))
	assert.Equal(t, 0, len(compRes.conditions))
}

// TestCompareAppStateExtra tests when there is an extra object in live but not defined in git
func TestCompareAppStateExtra(t *testing.T) {
	pod := test.NewPod()
	pod.SetNamespace(test.FakeDestNamespace)
	app := newFakeApp()
	key := kube.ResourceKey{Group: "", Kind: "Pod", Namespace: test.FakeDestNamespace, Name: app.Name}
	data := fakeData{
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			key: pod,
		},
	}
	ctrl := newFakeController(&data)
	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Equal(t, 1, len(compRes.resources))
	assert.Equal(t, 1, len(compRes.managedResources))
	assert.Equal(t, 0, len(compRes.conditions))
}

// TestCompareAppStateHook checks that hooks are detected during manifest generation, and not
// considered as part of resources when assessing Synced status
func TestCompareAppStateHook(t *testing.T) {
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	podBytes, _ := json.Marshal(pod)
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{string(podBytes)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)
	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, 0, len(compRes.resources))
	assert.Equal(t, 0, len(compRes.managedResources))
	assert.Equal(t, 1, len(compRes.hooks))
	assert.Equal(t, 0, len(compRes.conditions))
}

// checks that ignore resources are detected, but excluded from status
func TestCompareAppStateCompareOptionIgnoreExtraneous(t *testing.T) {
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationCompareOptions: "IgnoreExtraneous"})
	app := newFakeApp()
	data := fakeData{
		apps: []runtime.Object{app},
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: make(map[kube.ResourceKey]*unstructured.Unstructured),
	}
	ctrl := newFakeController(&data)

	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)

	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 0)
	assert.Len(t, compRes.managedResources, 0)
	assert.Len(t, compRes.conditions, 0)
}

// TestCompareAppStateExtraHook tests when there is an extra _hook_ object in live but not defined in git
func TestCompareAppStateExtraHook(t *testing.T) {
	pod := test.NewPod()
	pod.SetAnnotations(map[string]string{common.AnnotationKeyHook: "PreSync"})
	pod.SetNamespace(test.FakeDestNamespace)
	app := newFakeApp()
	key := kube.ResourceKey{Group: "", Kind: "Pod", Namespace: test.FakeDestNamespace, Name: app.Name}
	data := fakeData{
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			key: pod,
		},
	}
	ctrl := newFakeController(&data)
	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, 1, len(compRes.resources))
	assert.Equal(t, 1, len(compRes.managedResources))
	assert.Equal(t, 0, len(compRes.hooks))
	assert.Equal(t, 0, len(compRes.conditions))
}

func toJSON(t *testing.T, obj *unstructured.Unstructured) string {
	data, err := json.Marshal(obj)
	assert.NoError(t, err)
	return string(data)
}

func TestCompareAppStateDuplicatedNamespacedResources(t *testing.T) {
	obj1 := test.NewPod()
	obj1.SetNamespace(test.FakeDestNamespace)
	obj2 := test.NewPod()
	obj3 := test.NewPod()
	obj3.SetNamespace("kube-system")

	app := newFakeApp()
	data := fakeData{
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{toJSON(t, obj1), toJSON(t, obj2), toJSON(t, obj3)},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(obj1): obj1,
			kube.GetResourceKey(obj3): obj3,
		},
	}
	ctrl := newFakeController(&data)
	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Contains(t, compRes.conditions, argoappv1.ApplicationCondition{
		Message: "Resource /Pod/fake-dest-ns/my-pod appeared 2 times among application resources.",
		Type:    argoappv1.ApplicationConditionRepeatedResourceWarning,
	})
	assert.Equal(t, 2, len(compRes.resources))
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

func TestSetHealth(t *testing.T) {
	app := newFakeApp()
	deployment := kube.MustToUnstructured(&v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
	})
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(deployment): deployment,
		},
	})

	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)

	assert.Equal(t, compRes.healthStatus.Status, argoappv1.HealthStatusHealthy)
}

func TestSetHealthSelfReferencedApp(t *testing.T) {
	app := newFakeApp()
	unstructuredApp := kube.MustToUnstructured(app)
	deployment := kube.MustToUnstructured(&v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1beta1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "default",
		},
	})
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app, &defaultProj},
		manifestResponse: &repository.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(deployment):      deployment,
			kube.GetResourceKey(unstructuredApp): unstructuredApp,
		},
	})

	compRes, err := ctrl.appStateManager.CompareAppState(app, "", app.Spec.Source, false)
	assert.NoError(t, err)

	assert.Equal(t, compRes.healthStatus.Status, argoappv1.HealthStatusHealthy)
}
