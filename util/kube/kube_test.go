package kube

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/argoproj/argo-cd/v2/common"
)

const depWithoutSelector = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
`

const depWithSelector = `
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - image: nginx:1.7.9
        name: nginx
        ports:
        - containerPort: 80
`

func TestSetLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithoutSelector, depWithSelector} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		assert.Nil(t, err)

		err = SetAppInstanceLabel(&obj, common.LabelKeyAppInstance, "my-app")
		assert.Nil(t, err)

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		assert.Nil(t, err)
		log.Println(string(manifestBytes))

		var depV1Beta1 extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &depV1Beta1)
		assert.Nil(t, err)

		// the following makes sure we are not falling into legacy code which injects labels
		if yamlStr == depWithoutSelector {
			assert.Nil(t, depV1Beta1.Spec.Selector)
		} else if yamlStr == depWithSelector {
			assert.Equal(t, 1, len(depV1Beta1.Spec.Selector.MatchLabels))
			assert.Equal(t, "nginx", depV1Beta1.Spec.Selector.MatchLabels["app"])
		}
		assert.Equal(t, 1, len(depV1Beta1.Spec.Template.Labels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Template.Labels["app"])
	}
}

func TestSetLegacyLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithoutSelector, depWithSelector} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		assert.Nil(t, err)

		err = SetAppInstanceLabel(&obj, common.LabelKeyLegacyApplicationName, "my-app")
		assert.Nil(t, err)

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		assert.Nil(t, err)
		log.Println(string(manifestBytes))

		var depV1Beta1 extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &depV1Beta1)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(depV1Beta1.Spec.Selector.MatchLabels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Selector.MatchLabels["app"])
		assert.Equal(t, 2, len(depV1Beta1.Spec.Template.Labels))
		assert.Equal(t, "nginx", depV1Beta1.Spec.Template.Labels["app"])
		assert.Equal(t, "my-app", depV1Beta1.Spec.Template.Labels[common.LabelKeyLegacyApplicationName])
	}
}

func TestSetLegacyJobLabel(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/job.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetAppInstanceLabel(&obj, common.LabelKeyLegacyApplicationName, "my-app")
	assert.Nil(t, err)

	manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	assert.Nil(t, err)
	log.Println(string(manifestBytes))

	job := unstructured.Unstructured{}
	err = json.Unmarshal(manifestBytes, &job)
	assert.Nil(t, err)

	labels := job.GetLabels()
	assert.Equal(t, "my-app", labels[common.LabelKeyLegacyApplicationName])

	templateLabels, ok, err := unstructured.NestedMap(job.UnstructuredContent(), "spec", "template", "metadata", "labels")
	assert.True(t, ok)
	assert.Nil(t, err)
	assert.Equal(t, "my-app", templateLabels[common.LabelKeyLegacyApplicationName])
}

func TestSetSvcLabel(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetAppInstanceLabel(&obj, common.LabelKeyAppInstance, "my-app")
	assert.Nil(t, err)

	manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	assert.Nil(t, err)
	log.Println(string(manifestBytes))

	var s apiv1.Service
	err = json.Unmarshal(manifestBytes, &s)
	assert.Nil(t, err)

	log.Println(s.Name)
	log.Println(s.ObjectMeta)
	assert.Equal(t, "my-app", s.ObjectMeta.Labels[common.LabelKeyAppInstance])
}

func TestIsValidResourceName(t *testing.T) {
	assert.True(t, IsValidResourceName("guestbook-ui"))
	assert.True(t, IsValidResourceName("guestbook-ui1"))
	assert.False(t, IsValidResourceName("Guestbook-ui"))
	assert.False(t, IsValidResourceName("-guestbook-ui"))
}

func TestSetAppInstanceAnnotation(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance, "my-app")
	assert.Nil(t, err)

	manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	assert.Nil(t, err)
	log.Println(string(manifestBytes))

	var s apiv1.Service
	err = json.Unmarshal(manifestBytes, &s)
	assert.Nil(t, err)

	log.Println(s.Name)
	log.Println(s.ObjectMeta)
	assert.Equal(t, "my-app", s.ObjectMeta.Annotations[common.LabelKeyAppInstance])
}

func TestGetAppInstanceAnnotation(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance, "my-app")
	assert.Nil(t, err)

	assert.Equal(t, "my-app", GetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance))
}

func TestGetAppInstanceLabel(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	err = SetAppInstanceLabel(&obj, common.LabelKeyAppInstance, "my-app")
	assert.Nil(t, err)
	assert.Equal(t, "my-app", GetAppInstanceLabel(&obj, common.LabelKeyAppInstance))
}

func TestRemoveLabel(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("testdata/svc.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	obj.SetLabels(map[string]string{"test": "value"})

	RemoveLabel(&obj, "test")

	assert.Nil(t, obj.GetLabels())
}
