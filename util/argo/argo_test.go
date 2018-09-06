package argo

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/discovery/fake"
	testcore "k8s.io/client-go/testing"
)

func TestRefreshApp(t *testing.T) {
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset := appclientset.NewSimpleClientset(&testApp)
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	_, err := RefreshApp(appIf, "test-app")
	assert.Nil(t, err)
	// For some reason, the fake Application inferface doesn't reflect the patch status after Patch(),
	// so can't verify it was set in unit tests.
	//_, ok := newApp.Annotations[common.AnnotationKeyRefresh]
	//assert.True(t, ok)
}

func TestGetAppProjectWithNoProjDefined(t *testing.T) {
	projName := "default"
	namespace := "default"

	testProj := &argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{Name: projName, Namespace: namespace},
	}

	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = namespace
	appClientset := appclientset.NewSimpleClientset(testProj)
	proj, err := GetAppProject(&testApp.Spec, appClientset, namespace)
	assert.Nil(t, err)
	assert.Equal(t, proj.Name, projName)
}

func TestCheckValidParam(t *testing.T) {
	oldAppSet := make(map[string]map[string]bool)
	oldAppSet["testComponent"] = make(map[string]bool)
	oldAppSet["testComponent"]["overrideParam"] = true
	newParam := argoappv1.ComponentParameter{
		Component: "testComponent",
		Name:      "overrideParam",
		Value:     "new-value",
	}
	badParam := argoappv1.ComponentParameter{
		Component: "testComponent",
		Name:      "badParam",
		Value:     "new-value",
	}
	assert.True(t, CheckValidParam(oldAppSet, newParam))
	assert.False(t, CheckValidParam(oldAppSet, badParam))
}

func TestWaitForRefresh(t *testing.T) {
	appClientset := appclientset.NewSimpleClientset()

	// Verify timeout
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	oneHundredMs := 100 * time.Millisecond
	app, err := WaitForRefresh(appIf, "test-app", &oneHundredMs)
	assert.NotNil(t, err)
	assert.Nil(t, app)
	assert.Contains(t, strings.ToLower(err.Error()), "timed out")

	// Verify success
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	testApp.ObjectMeta.Annotations = map[string]string{
		common.AnnotationKeyRefresh: time.Now().UTC().Format(time.RFC3339),
	}
	testApp.Status.ComparisonResult.ComparedAt = metav1.Time{Time: time.Now().UTC()}
	appClientset = appclientset.NewSimpleClientset()

	appIf = appClientset.ArgoprojV1alpha1().Applications("default")
	watcher := watch.NewFake()
	appClientset.PrependWatchReactor("applications", testcore.DefaultWatchReactor(watcher, nil))
	// simulate add/update/delete watch events
	go watcher.Add(&testApp)
	app, err = WaitForRefresh(appIf, "test-app", &oneHundredMs)
	assert.Nil(t, err)
	assert.NotNil(t, app)
}

func TestFindRestrictedGroupKinds(t *testing.T) {
	proj := argoappv1.AppProject{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
		Spec: argoappv1.AppProjectSpec{
			ClusterResources: []metav1.GroupKind{
				{Group: "argoproj.io", Kind: "*"},
			},
		},
	}
	disco := &fake.FakeDiscovery{Fake: &testcore.Fake{}}
	disco.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: corev1.SchemeGroupVersion.String(),
			APIResources: []metav1.APIResource{
				{Name: "clusterroles", Namespaced: false, Kind: "ClusterRole", Group: "rbac.authorization.k8s.io"},
				{Name: "workflows", Namespaced: false, Kind: "Workflow", Group: "argoproj.io"},
				{Name: "application", Namespaced: false, Kind: "Application", Group: "argoproj.io"},
			},
		},
	}

	res, err := FindRestrictedGroupKinds(proj, &argoappv1.ComparisonResult{
		Resources: []argoappv1.ResourceState{{
			TargetState: `{"apiVersion": "rbac.authorization.k8s.io/v1", "kind": "ClusterRole", "metadata": {"name": "argo-ui-cluster-role" }}`,
			LiveState:   "null",
		}},
	}, "fake", disco)

	assert.Nil(t, err)

	assert.Equal(t, 1, len(res))
	assert.Equal(t, "ClusterRole", res[0].Kind)
}
