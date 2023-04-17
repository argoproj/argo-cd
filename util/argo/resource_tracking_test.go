package argo

import (
	"os"
	"testing"

	"github.com/argoproj/argo-cd/v2/util/kube"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"
)

func TestSetAppInstanceLabel(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", "", TrackingMethodLabel)
	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, TrackingMethodLabel)
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotation(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	resourceTracking.SetAppInstance(&obj, common.AnnotationKeyAppInstance, "my-app", "", TrackingMethodAnnotation)

	app := resourceTracking.GetAppName(&obj, common.AnnotationKeyAppInstance, TrackingMethodAnnotation)
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotationAndLabel(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	resourceTracking.SetAppInstance(&obj, common.LabelKeyAppInstance, "my-app", "", TrackingMethodAnnotationAndLabel)

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, TrackingMethodAnnotationAndLabel)
	assert.Equal(t, "my-app", app)
}

func TestSetAppInstanceAnnotationNotFound(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)

	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	resourceTracking := NewResourceTracking()

	app := resourceTracking.GetAppName(&obj, common.LabelKeyAppInstance, TrackingMethodAnnotation)
	assert.Equal(t, "", app)
}

func TestParseAppInstanceValue(t *testing.T) {
	resourceTracking := NewResourceTracking()
	appInstanceValue, err := resourceTracking.ParseAppInstanceValue("app:<group>/<kind>:<namespace>/<name>")
	if !assert.NoError(t, err) {
		t.Fatal()
	}
	assert.Equal(t, appInstanceValue.ApplicationName, "app")
	assert.Equal(t, appInstanceValue.Group, "<group>")
	assert.Equal(t, appInstanceValue.Kind, "<kind>")
	assert.Equal(t, appInstanceValue.Namespace, "<namespace>")
	assert.Equal(t, appInstanceValue.Name, "<name>")
}

func TestParseAppInstanceValueWrongFormat1(t *testing.T) {
	resourceTracking := NewResourceTracking()
	_, err := resourceTracking.ParseAppInstanceValue("app")
	assert.Error(t, err, WrongResourceTrackingFormat)
}

func TestParseAppInstanceValueWrongFormat2(t *testing.T) {
	resourceTracking := NewResourceTracking()
	_, err := resourceTracking.ParseAppInstanceValue("app;group/kind/ns")
	assert.Error(t, err, WrongResourceTrackingFormat)
}

func TestParseAppInstanceValueCorrectFormat(t *testing.T) {
	resourceTracking := NewResourceTracking()
	_, err := resourceTracking.ParseAppInstanceValue("app:group/kind:test/ns")
	assert.NoError(t, err)
}

func sampleResource() *unstructured.Unstructured {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	if err != nil {
		panic(err)
	}
	var obj *unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	if err != nil {
		panic(err)
	}
	return obj
}

func TestResourceIdNormalizer_Normalize(t *testing.T) {
	rt := NewResourceTracking()

	// live object is a resource that has old style tracking label
	liveObj := sampleResource()
	rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "my-app", "", TrackingMethodLabel)

	// config object is a resource that has new style tracking annotation
	configObj := sampleResource()
	rt.SetAppInstance(configObj, common.AnnotationKeyAppInstance, "my-app2", "", TrackingMethodAnnotation)

	_ = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(TrackingMethodAnnotation))

	// the normalization should affect add the new style annotation and drop old tracking label from live object
	annotation := kube.GetAppInstanceAnnotation(configObj, common.AnnotationKeyAppInstance)
	assert.Equal(t, liveObj.GetAnnotations()[common.AnnotationKeyAppInstance], annotation)
	_, hasOldLabel := liveObj.GetLabels()[common.LabelKeyAppInstance]
	assert.False(t, hasOldLabel)
}

func TestResourceIdNormalizer_Normalize_ConfigHasOldLabel(t *testing.T) {
	rt := NewResourceTracking()

	// live object is a resource that has old style tracking label
	liveObj := sampleResource()
	rt.SetAppInstance(liveObj, common.LabelKeyAppInstance, "my-app", "", TrackingMethodLabel)

	// config object is a resource that has new style tracking annotation
	configObj := sampleResource()
	rt.SetAppInstance(configObj, common.AnnotationKeyAppInstance, "my-app2", "", TrackingMethodAnnotation)
	rt.SetAppInstance(configObj, common.LabelKeyAppInstance, "my-app", "", TrackingMethodLabel)

	_ = rt.Normalize(configObj, liveObj, common.LabelKeyAppInstance, string(TrackingMethodAnnotation))

	// the normalization should affect add the new style annotation and drop old tracking label from live object
	annotation := kube.GetAppInstanceAnnotation(configObj, common.AnnotationKeyAppInstance)
	assert.Equal(t, liveObj.GetAnnotations()[common.AnnotationKeyAppInstance], annotation)
	_, hasOldLabel := liveObj.GetLabels()[common.LabelKeyAppInstance]
	assert.True(t, hasOldLabel)
}

func TestIsOldTrackingMethod(t *testing.T) {
	assert.Equal(t, true, IsOldTrackingMethod(string(TrackingMethodLabel)))
}
