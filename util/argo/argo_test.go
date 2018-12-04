package argo

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/argoproj/argo-cd/common"
	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
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
	testApp.Status.ObservedAt = metav1.Now()
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

func TestContainsSyncResource(t *testing.T) {
	var (
		blankUnstructured unstructured.Unstructured
		blankResource     argoappv1.SyncOperationResource
		helloResource     = argoappv1.SyncOperationResource{Name: "hello"}
	)
	tables := []struct {
		u        *unstructured.Unstructured
		rr       []argoappv1.SyncOperationResource
		expected bool
	}{
		{&blankUnstructured, []argoappv1.SyncOperationResource{}, false},
		{&blankUnstructured, []argoappv1.SyncOperationResource{blankResource}, true},
		{&blankUnstructured, []argoappv1.SyncOperationResource{helloResource}, false},
	}

	for _, table := range tables {
		if out := ContainsSyncResource(table.u.GetName(), table.u.GroupVersionKind(), table.rr); out != table.expected {
			t.Errorf("Expected %t for slice %+v conains resource %+v; instead got %t", table.expected, table.rr, table.u, out)
		}
	}
}

func TestVerifyOneSourceType(t *testing.T) {
	src := argoappv1.ApplicationSource{
		Ksonnet: &argoappv1.ApplicationSourceKsonnet{
			Environment: "foo",
		},
		Kustomize: &argoappv1.ApplicationSourceKustomize{
			NamePrefix: "foo",
		},
		Helm: &argoappv1.ApplicationSourceHelm{
			ValueFiles: []string{"foo"},
		},
	}
	assert.NotNil(t, verifyOneSourceType(&src))
	src = argoappv1.ApplicationSource{
		Helm: &argoappv1.ApplicationSourceHelm{
			ValueFiles: []string{"foo"},
		},
	}
	assert.Nil(t, verifyOneSourceType(&src))
}

func TestNormalizeApplicationSpec(t *testing.T) {
	spec := NormalizeApplicationSpec(&argoappv1.ApplicationSpec{})
	assert.Equal(t, spec.Project, "default")

	spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
		Source: argoappv1.ApplicationSource{
			Environment: "foo",
		},
	})
	assert.Equal(t, spec.Source.Ksonnet.Environment, "foo")

	spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
		Source: argoappv1.ApplicationSource{
			Ksonnet: &argoappv1.ApplicationSourceKsonnet{
				Environment: "foo",
			},
		},
	})
	assert.Equal(t, spec.Source.Environment, "foo")

	spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
		Source: argoappv1.ApplicationSource{
			ValuesFiles: []string{"values-prod.yaml"},
		},
	})
	assert.Equal(t, spec.Source.Helm.ValueFiles[0], "values-prod.yaml")

	spec = NormalizeApplicationSpec(&argoappv1.ApplicationSpec{
		Source: argoappv1.ApplicationSource{
			Helm: &argoappv1.ApplicationSourceHelm{
				ValueFiles: []string{"values-prod.yaml"},
			},
		},
	})
	assert.Equal(t, spec.Source.ValuesFiles[0], "values-prod.yaml")
}
