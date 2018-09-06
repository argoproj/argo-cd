package kube

import (
	"io/ioutil"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConvertToVersion(t *testing.T) {
	kubectl := KubectlCmd{}
	yamlBytes, err := ioutil.ReadFile("testdata/nginx.yaml")
	assert.Nil(t, err)
	var obj unstructured.Unstructured
	err = yaml.Unmarshal(yamlBytes, &obj)
	assert.Nil(t, err)

	// convert an extensions/v1beta1 object into an apps/v1
	newObj, err := kubectl.ConvertToVersion(&obj, "apps", "v1")
	assert.Nil(t, err)
	gvk := newObj.GroupVersionKind()
	assert.Equal(t, "apps", gvk.Group)
	assert.Equal(t, "v1", gvk.Version)

	// converting it again should not have any affect
	newObj, err = kubectl.ConvertToVersion(&obj, "apps", "v1")
	assert.Nil(t, err)
	gvk = newObj.GroupVersionKind()
	assert.Equal(t, "apps", gvk.Group)
	assert.Equal(t, "v1", gvk.Version)
}
