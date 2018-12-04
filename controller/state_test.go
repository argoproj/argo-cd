package controller

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
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
	compRes, maniRes, resources, cond, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.Status)
	assert.Equal(t, 0, len(compRes.Resources))
	assert.Equal(t, data.manifestResponse, maniRes)
	assert.Equal(t, 0, len(resources))
	assert.Equal(t, 0, len(cond))
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
	compRes, maniRes, resources, cond, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.Status)
	assert.Equal(t, 1, len(compRes.Resources))
	assert.Equal(t, data.manifestResponse, maniRes)
	assert.Equal(t, 1, len(resources))
	assert.Equal(t, 0, len(cond))
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
	compRes, maniRes, resources, cond, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.Status)
	assert.Equal(t, 1, len(compRes.Resources))
	assert.Equal(t, data.manifestResponse, maniRes)
	assert.Equal(t, 1, len(resources))
	assert.Equal(t, 0, len(cond))
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
	compRes, maniRes, resources, cond, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.Status)
	assert.Equal(t, 0, len(compRes.Resources))
	assert.Equal(t, data.manifestResponse, maniRes)
	assert.Equal(t, 0, len(resources))
	assert.Equal(t, 0, len(cond))
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
	compRes, maniRes, resources, cond, err := ctrl.appStateManager.CompareAppState(app, "", nil)
	assert.NoError(t, err)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.Status)
	assert.Equal(t, 1, len(compRes.Resources))
	assert.Equal(t, data.manifestResponse, maniRes)
	assert.Equal(t, 1, len(resources))
	assert.Equal(t, 0, len(cond))
}
