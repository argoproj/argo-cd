package health

import (
	"io/ioutil"
	"testing"

	appv1 "github.com/argoproj/argo-cd/pkg/apis/application/v1alpha1"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
	assert.Equal(t, health.Status, appv1.HealthStatusHealthy)
}
