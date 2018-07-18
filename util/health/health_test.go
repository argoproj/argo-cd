package health

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
)

func TestDeploymentHealth(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("../kube/testdata/nginx.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := getDeploymentHealth(&obj)
	assert.Nil(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, appv1.HealthStatusHealthy, health.Status)
}

func TestDeploymentProgressing(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("./testdata/progressing.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := getDeploymentHealth(&obj)
	assert.Nil(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, appv1.HealthStatusProgressing, health.Status)
}

func TestDeploymentDegraded(t *testing.T) {
	yamlBytes, err := ioutil.ReadFile("./testdata/degraded.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)
	health, err := getDeploymentHealth(&obj)
	assert.Nil(t, err)
	assert.NotNil(t, health)
	assert.Equal(t, appv1.HealthStatusDegraded, health.Status)
}
