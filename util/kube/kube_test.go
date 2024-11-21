package kube

import (
	"encoding/json"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiv1 "k8s.io/api/core/v1"
	extv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"

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
		require.NoError(t, err)

		err = SetAppInstanceLabel(&obj, common.LabelKeyAppInstance, "my-app")
		require.NoError(t, err)

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		require.NoError(t, err)
		log.Println(string(manifestBytes))

		var depV1Beta1 extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &depV1Beta1)
		require.NoError(t, err)

		// the following makes sure we are not falling into legacy code which injects labels
		if yamlStr == depWithoutSelector {
			assert.Nil(t, depV1Beta1.Spec.Selector)
		} else if yamlStr == depWithSelector {
			assert.Len(t, depV1Beta1.Spec.Selector.MatchLabels, 1)
			assert.Equal(t, "nginx", depV1Beta1.Spec.Selector.MatchLabels["app"])
		}
		assert.Len(t, depV1Beta1.Spec.Template.Labels, 1)
		assert.Equal(t, "nginx", depV1Beta1.Spec.Template.Labels["app"])
	}
}

func TestSetLegacyLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithoutSelector, depWithSelector} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		require.NoError(t, err)

		err = SetAppInstanceLabel(&obj, common.LabelKeyLegacyApplicationName, "my-app")
		require.NoError(t, err)

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		require.NoError(t, err)
		log.Println(string(manifestBytes))

		var depV1Beta1 extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &depV1Beta1)
		require.NoError(t, err)
		assert.Len(t, depV1Beta1.Spec.Selector.MatchLabels, 1)
		assert.Equal(t, "nginx", depV1Beta1.Spec.Selector.MatchLabels["app"])
		assert.Len(t, depV1Beta1.Spec.Template.Labels, 2)
		assert.Equal(t, "nginx", depV1Beta1.Spec.Template.Labels["app"])
		assert.Equal(t, "my-app", depV1Beta1.Spec.Template.Labels[common.LabelKeyLegacyApplicationName])
	}
}

func TestSetLegacyJobLabel(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/job.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	err = SetAppInstanceLabel(&obj, common.LabelKeyLegacyApplicationName, "my-app")
	require.NoError(t, err)

	manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	require.NoError(t, err)
	log.Println(string(manifestBytes))

	job := unstructured.Unstructured{}
	err = json.Unmarshal(manifestBytes, &job)
	require.NoError(t, err)

	labels := job.GetLabels()
	assert.Equal(t, "my-app", labels[common.LabelKeyLegacyApplicationName])

	templateLabels, ok, err := unstructured.NestedMap(job.UnstructuredContent(), "spec", "template", "metadata", "labels")
	assert.True(t, ok)
	require.NoError(t, err)
	assert.Equal(t, "my-app", templateLabels[common.LabelKeyLegacyApplicationName])
}

func TestSetSvcLabel(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	err = SetAppInstanceLabel(&obj, common.LabelKeyAppInstance, "my-app")
	require.NoError(t, err)

	manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	require.NoError(t, err)
	log.Println(string(manifestBytes))

	var s apiv1.Service
	err = json.Unmarshal(manifestBytes, &s)
	require.NoError(t, err)

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
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	err = SetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance, "my-app")
	require.NoError(t, err)

	manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
	require.NoError(t, err)
	log.Println(string(manifestBytes))

	var s apiv1.Service
	err = json.Unmarshal(manifestBytes, &s)
	require.NoError(t, err)

	log.Println(s.Name)
	log.Println(s.ObjectMeta)
	assert.Equal(t, "my-app", s.ObjectMeta.Annotations[common.LabelKeyAppInstance])
}

func TestSetAppInstanceAnnotationWithInvalidData(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc-with-invalid-data.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	err = SetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance, "my-app")
	require.Error(t, err)
	assert.Equal(t, "failed to get annotations from target object /v1, Kind=Service /my-service: .metadata.annotations accessor error: contains non-string value in the map under key \"invalid-annotation\": <nil> is of the type <nil>, expected string", err.Error())
}

func TestGetAppInstanceAnnotation(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	err = SetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance, "my-app")
	require.NoError(t, err)

	annotation, err := GetAppInstanceAnnotation(&obj, common.LabelKeyAppInstance)
	require.NoError(t, err)
	assert.Equal(t, "my-app", annotation)
}

func TestGetAppInstanceAnnotationWithInvalidData(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc-with-invalid-data.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	_, err = GetAppInstanceAnnotation(&obj, "valid-annotation")
	require.Error(t, err)
	assert.Equal(t, "failed to get annotations from target object /v1, Kind=Service /my-service: .metadata.annotations accessor error: contains non-string value in the map under key \"invalid-annotation\": <nil> is of the type <nil>, expected string", err.Error())
}

func TestGetAppInstanceLabel(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	err = SetAppInstanceLabel(&obj, common.LabelKeyAppInstance, "my-app")
	require.NoError(t, err)
	label, err := GetAppInstanceLabel(&obj, common.LabelKeyAppInstance)
	require.NoError(t, err)
	assert.Equal(t, "my-app", label)
}

func TestGetAppInstanceLabelWithInvalidData(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc-with-invalid-data.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	_, err = GetAppInstanceLabel(&obj, "valid-label")
	require.Error(t, err)
	assert.Equal(t, "failed to get labels for /v1, Kind=Service /my-service: .metadata.labels accessor error: contains non-string value in the map under key \"invalid-label\": <nil> is of the type <nil>, expected string", err.Error())
}

func TestRemoveLabel(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)
	obj.SetLabels(map[string]string{"test": "value"})

	err = RemoveLabel(&obj, "test")
	require.NoError(t, err)

	assert.Nil(t, obj.GetLabels())
}

func TestRemoveLabelWithInvalidData(t *testing.T) {
	yamlBytes, err := os.ReadFile("testdata/svc-with-invalid-data.yaml")
	require.NoError(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	require.NoError(t, err)

	err = RemoveLabel(&obj, "valid-label")
	require.Error(t, err)
	assert.Equal(t, "failed to get labels for /v1, Kind=Service /my-service: .metadata.labels accessor error: contains non-string value in the map under key \"invalid-label\": <nil> is of the type <nil>, expected string", err.Error())
}
