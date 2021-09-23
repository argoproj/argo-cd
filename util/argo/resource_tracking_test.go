package argo

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/test"

	"k8s.io/client-go/tools/cache"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"

	appsv1 "github.com/argoproj/argo-cd/v2/pkg/apis/application/v1alpha1"
	appsfake "github.com/argoproj/argo-cd/v2/pkg/client/clientset/versioned/fake"
	appinformers "github.com/argoproj/argo-cd/v2/pkg/client/informers/externalversions/application/v1alpha1"
)

const fakeApp = `
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: test-app
  namespace: default
spec:
  source:
    path: some/path
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    ksonnet:
      environment: default
  destination:
    namespace: ` + test.FakeDestNamespace + `
    server: https://cluster-api.com
`

func createTestApp(testApp string, opts ...func(app *appsv1.Application)) *appsv1.Application {
	var app appsv1.Application
	err := yaml.Unmarshal([]byte(testApp), &app)
	if err != nil {
		panic(err)
	}
	for i := range opts {
		opts[i](&app)
	}
	return &app
}

func TestSetAppInstanceLabel(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", TrackingMethodLabel)
	assert.Nil(t, err)

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, TrackingMethodLabel)
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotation(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	err = resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", TrackingMethodAnnotation)
	assert.Nil(t, err)

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, TrackingMethodAnnotation)
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotationNotFound(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, TrackingMethodAnnotation)
	assert.Equal(t, "", app)
}

func TestGetTrackingMethodFromApplicationInformer(t *testing.T) {
	appclientset := appsfake.NewSimpleClientset()
	appInformer := appinformers.NewApplicationInformer(appclientset, "", time.Minute, cache.Indexers{})

	appName, err := GetTrackingMethodFromApplicationInformer(appInformer, "", "test")
	assert.NoError(t, err)

}
