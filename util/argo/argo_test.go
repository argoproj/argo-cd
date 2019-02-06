package argo

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/watch"
	testcore "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	argoappv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	appclientset "github.com/argoproj/argo-cd/pkg/client/clientset/versioned/fake"
	"github.com/argoproj/argo-cd/pkg/client/informers/externalversions/application/v1alpha1"
	applisters "github.com/argoproj/argo-cd/pkg/client/listers/application/v1alpha1"
)

func TestRefreshApp(t *testing.T) {
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset := appclientset.NewSimpleClientset(&testApp)
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	_, err := RefreshApp(appIf, "test-app", argoappv1.RefreshTypeNormal)
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	informer := v1alpha1.NewAppProjectInformer(appClientset, namespace, 0, cache.Indexers{})
	go informer.Run(ctx.Done())
	cache.WaitForCacheSync(ctx.Done(), informer.HasSynced)
	proj, err := GetAppProject(&testApp.Spec, applisters.NewAppProjectLister(informer.GetIndexer()), namespace)
	assert.Nil(t, err)
	assert.Equal(t, proj.Name, projName)
}

func TestWaitForRefresh(t *testing.T) {
	appClientset := appclientset.NewSimpleClientset()

	// Verify timeout
	appIf := appClientset.ArgoprojV1alpha1().Applications("default")
	oneHundredMs := 100 * time.Millisecond
	app, err := WaitForRefresh(context.Background(), appIf, "test-app", &oneHundredMs)
	assert.NotNil(t, err)
	assert.Nil(t, app)
	assert.Contains(t, strings.ToLower(err.Error()), "deadline exceeded")

	// Verify success
	var testApp argoappv1.Application
	testApp.Name = "test-app"
	testApp.Namespace = "default"
	appClientset = appclientset.NewSimpleClientset()

	appIf = appClientset.ArgoprojV1alpha1().Applications("default")
	watcher := watch.NewFake()
	appClientset.PrependWatchReactor("applications", testcore.DefaultWatchReactor(watcher, nil))
	// simulate add/update/delete watch events
	go watcher.Add(&testApp)
	app, err = WaitForRefresh(context.Background(), appIf, "test-app", &oneHundredMs)
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

func TestVerifyOneSourceTypeWithDirectory(t *testing.T) {
	src := argoappv1.ApplicationSource{
		Ksonnet: &argoappv1.ApplicationSourceKsonnet{
			Environment: "foo",
		},
		Directory: &argoappv1.ApplicationSourceDirectory{},
	}
	assert.NotNil(t, verifyOneSourceType(&src), "cannot add directory with any other types")
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
