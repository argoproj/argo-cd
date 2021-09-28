package argo

import (
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"
)

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
