package controller

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/argoproj/gitops-engine/pkg/health"
	synccommon "github.com/argoproj/gitops-engine/pkg/sync/common"
	"github.com/argoproj/gitops-engine/pkg/utils/kube"
	. "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/argoproj/argo-cd/reposerver/apiclient"
	"github.com/argoproj/argo-cd/test"
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 0)
	assert.Len(t, compRes.managedResources, 0)
	assert.Len(t, app.Status.Conditions, 0)
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
	assert.NotNil(t, compRes)
	assert.NotNil(t, compRes.syncStatus)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 1)
	assert.Len(t, compRes.managedResources, 1)
	assert.Len(t, app.Status.Conditions, 0)
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeOutOfSync, compRes.syncStatus.Status)
	assert.Equal(t, 1, len(compRes.resources))
	assert.Equal(t, 1, len(compRes.managedResources))
	assert.Equal(t, 0, len(app.Status.Conditions))
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, 0, len(compRes.resources))
	assert.Equal(t, 0, len(compRes.managedResources))
	assert.Equal(t, 1, len(compRes.reconciliationResult.Hooks))
	assert.Equal(t, 0, len(app.Status.Conditions))
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, 1, len(compRes.resources))
	assert.Equal(t, 1, len(compRes.managedResources))
	assert.Equal(t, 0, len(compRes.reconciliationResult.Hooks))
	assert.Equal(t, 0, len(app.Status.Conditions))
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
	ctrl := newFakeController(&data)

	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)

	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Len(t, compRes.resources, 0)
	assert.Len(t, compRes.managedResources, 0)
	assert.Len(t, app.Status.Conditions, 0)
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)

	assert.NotNil(t, compRes)
	assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
	assert.Equal(t, 1, len(compRes.resources))
	assert.Equal(t, 1, len(compRes.managedResources))
	assert.Equal(t, 0, len(compRes.reconciliationResult.Hooks))
	assert.Equal(t, 0, len(app.Status.Conditions))
}

func toJSON(t *testing.T, obj *unstructured.Unstructured) string {
	data, err := json.Marshal(obj)
	assert.NoError(t, err)
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
	ctrl := newFakeController(&data)
	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)

	assert.NotNil(t, compRes)
	assert.Equal(t, 1, len(app.Status.Conditions))
	assert.NotNil(t, app.Status.Conditions[0].LastTransitionTime)
	assert.Equal(t, argoappv1.ApplicationConditionRepeatedResourceWarning, app.Status.Conditions[0].Type)
	assert.Equal(t, "Resource /Pod/fake-dest-ns/my-pod appeared 2 times among application resources.", app.Status.Conditions[0].Message)
	assert.Equal(t, 4, len(compRes.resources))
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
		manifestResponse: &apiclient.ManifestResponse{
			Manifests: []string{},
			Namespace: test.FakeDestNamespace,
			Server:    test.FakeClusterURL,
			Revision:  "abc123",
		},
		managedLiveObjs: map[kube.ResourceKey]*unstructured.Unstructured{
			kube.GetResourceKey(deployment): deployment,
		},
	})

	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)

	assert.Equal(t, compRes.healthStatus.Status, health.HealthStatusHealthy)
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
	})

	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)

	assert.Equal(t, compRes.healthStatus.Status, health.HealthStatusHealthy)
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
	})

	tree, err := ctrl.setAppManagedResources(app, &comparisonResult{managedResources: make([]managedResource, 0)})

	assert.NoError(t, err)
	assert.Equal(t, len(tree.OrphanedNodes), 1)
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
	})

	tree, err := ctrl.setAppManagedResources(app1, &comparisonResult{managedResources: make([]managedResource, 0)})

	assert.NoError(t, err)
	assert.Equal(t, len(tree.OrphanedNodes), 0)
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
	})

	compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)

	assert.Equal(t, health.HealthStatusUnknown, compRes.healthStatus.Status)
	assert.Equal(t, argoappv1.SyncStatusCodeUnknown, compRes.syncStatus.Status)
}

func TestSetManagedResourcesKnownOrphanedResourceExceptions(t *testing.T) {
	proj := defaultProj.DeepCopy()
	proj.Spec.OrphanedResources = &argoappv1.OrphanedResourcesMonitorSettings{}

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
	})

	tree, err := ctrl.setAppManagedResources(app, &comparisonResult{managedResources: make([]managedResource, 0)})

	assert.NoError(t, err)
	assert.Len(t, tree.OrphanedNodes, 1)
	assert.Equal(t, "guestbook", tree.OrphanedNodes[0].Name)
}

func Test_appStateManager_persistRevisionHistory(t *testing.T) {
	app := newFakeApp()
	ctrl := newFakeController(&fakeData{
		apps: []runtime.Object{app},
	})
	manager := ctrl.appStateManager.(*appStateManager)
	setRevisionHistoryLimit := func(value int) {
		i := int64(value)
		app.Spec.RevisionHistoryLimit = &i
	}
	addHistory := func() {
		err := manager.persistRevisionHistory(app, "my-revision", argoappv1.ApplicationSource{}, metav1.Time{})
		assert.NoError(t, err)
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
	err := manager.persistRevisionHistory(app, "my-revision", argoappv1.ApplicationSource{}, metav1NowTime)
	assert.NoError(t, err)
	assert.Equal(t, app.Status.History.LastRevisionHistory().DeployStartedAt, &metav1NowTime)
}

// helper function to read contents of a file to string
// panics on error
func mustReadFile(path string) string {
	b, err := ioutil.ReadFile(path)
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
	oldval := os.Getenv("ARGOCD_GPG_ENABLED")
	os.Setenv("ARGOCD_GPG_ENABLED", "true")
	defer os.Setenv("ARGOCD_GPG_ENABLED", oldval)
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
		assert.Len(t, app.Status.Conditions, 0)
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &defaultProj, "", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
		assert.Len(t, app.Status.Conditions, 0)
	}
}

func TestSignedResponseSignatureRequired(t *testing.T) {
	oldval := os.Getenv("ARGOCD_GPG_ENABLED")
	os.Setenv("ARGOCD_GPG_ENABLED", "true")
	defer os.Setenv("ARGOCD_GPG_ENABLED", oldval)

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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
		assert.Len(t, app.Status.Conditions, 0)
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "abc123", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "abc123", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "abc123", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
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
		ctrl := newFakeController(&data)
		testProj := signedProj
		testProj.Spec.SignatureKeys[0].KeyID = "4AEE18F83AFDEB24"
		compRes := ctrl.appStateManager.CompareAppState(app, &testProj, "abc123", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "abc123", app.Spec.Source, false, localManifests)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeUnknown, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
		assert.Len(t, app.Status.Conditions, 1)
		assert.Contains(t, app.Status.Conditions[0].Message, "Cannot use local manifests")
	}

	os.Setenv("ARGOCD_GPG_ENABLED", "false")
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "abc123", app.Spec.Source, false, nil)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
		assert.Len(t, app.Status.Conditions, 0)
	}

	// Signature required and local manifests supplied and GPG subystem is disabled - sync
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
		ctrl := newFakeController(&data)
		compRes := ctrl.appStateManager.CompareAppState(app, &signedProj, "abc123", app.Spec.Source, false, localManifests)
		assert.NotNil(t, compRes)
		assert.NotNil(t, compRes.syncStatus)
		assert.Equal(t, argoappv1.SyncStatusCodeSynced, compRes.syncStatus.Status)
		assert.Len(t, compRes.resources, 0)
		assert.Len(t, compRes.managedResources, 0)
		assert.Len(t, app.Status.Conditions, 0)
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
