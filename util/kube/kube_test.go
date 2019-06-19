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
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/argoproj/argo-cd/common"
)

const depWithLabel = `
apiVersion: extensions/v1beta2
kind: Deployment
metadata:
  name: nginx-deployment
  labels:
    foo: bar
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

func TestUnsetLabels(t *testing.T) {
	for _, yamlStr := range []string{depWithLabel} {
		var obj unstructured.Unstructured
		err := yaml.Unmarshal([]byte(yamlStr), &obj)
		assert.Nil(t, err)

		UnsetLabel(&obj, "foo")

		manifestBytes, err := json.MarshalIndent(obj.Object, "", "  ")
		assert.Nil(t, err)
		log.Println(string(manifestBytes))

		var dep extv1beta1.Deployment
		err = json.Unmarshal(manifestBytes, &dep)
		assert.Nil(t, err)
		assert.Equal(t, 0, len(dep.ObjectMeta.Labels))
	}

}

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

func TestCleanKubectlOutput(t *testing.T) {
	testString := `error: error validating "STDIN": error validating data: ValidationError(Deployment.spec): missing required field "selector" in io.k8s.api.apps.v1beta2.DeploymentSpec; if you choose to ignore these errors, turn validation off with --validate=false`
	assert.Equal(t, cleanKubectlOutput(testString), `error validating data: ValidationError(Deployment.spec): missing required field "selector" in io.k8s.api.apps.v1beta2.DeploymentSpec`)
}

func TestInClusterKubeConfig(t *testing.T) {
	restConfig := &rest.Config{}
	kubeConfig := NewKubeConfig(restConfig, "")
	assert.NotEmpty(t, kubeConfig.AuthInfos[kubeConfig.CurrentContext].TokenFile)

	restConfig = &rest.Config{
		Password: "foo",
	}
	kubeConfig = NewKubeConfig(restConfig, "")
	assert.Empty(t, kubeConfig.AuthInfos[kubeConfig.CurrentContext].TokenFile)

	restConfig = &rest.Config{
		ExecProvider: &clientcmdapi.ExecConfig{
			APIVersion: "client.authentication.k8s.io/v1alpha1",
			Command:    "aws-iam-authenticator",
		},
	}
	kubeConfig = NewKubeConfig(restConfig, "")
	assert.Empty(t, kubeConfig.AuthInfos[kubeConfig.CurrentContext].TokenFile)
}

func TestGetDeploymentReplicas(t *testing.T) {
	manifest := []byte(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx-deployment
spec:
  replicas: 2
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: nginx
    spec:
      containers:
      - name: nginx
        image: nginx:1.7.9
        ports:
        - containerPort: 80	
`)
	deployment := unstructured.Unstructured{}
	err := yaml.Unmarshal(manifest, &deployment)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), *GetDeploymentReplicas(&deployment))
}

func TestGetNilDeploymentReplicas(t *testing.T) {
	manifest := []byte(`
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
  - image: nginx:1.7.9
    name: nginx
    resources:
      requests:
        cpu: 0.2
`)
	noDeployment := unstructured.Unstructured{}
	err := yaml.Unmarshal(manifest, &noDeployment)
	assert.NoError(t, err)
	assert.Nil(t, GetDeploymentReplicas(&noDeployment))
}
