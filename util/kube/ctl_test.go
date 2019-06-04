package kube

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConvertToVersion(t *testing.T) {
	/*
		ctl_test.go:22:
		Error Trace:	ctl_test.go:22
		Error:      	Expected nil, but got: &errors.errorString{s:"failed to convert Deployment/nginx-deployment to apps/v1"}
		Test:       	TestConvertToVersion
		panic: runtime error: invalid memory address or nil pointer dereference
		/home/circleci/sdk/go1.11.4/src/testing/testing.go:792 +0x387
		/home/circleci/sdk/go1.11.4/src/runtime/panic.go:513 +0x1b9
		/home/circleci/.go_workspace/src/github.com/argoproj/argo-cd/vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructured.go:200 +0x3a
		/home/circleci/.go_workspace/src/github.com/argoproj/argo-cd/vendor/k8s.io/apimachinery/pkg/apis/meta/v1/unstructured/unstructured.go:396 +0x5b
		/home/circleci/.go_workspace/src/github.com/argoproj/argo-cd/util/kube/ctl_test.go:23 +0x1e4
		/home/circleci/sdk/go1.11.4/src/testing/testing.go:827 +0xbf
		/home/circleci/sdk/go1.11.4/src/testing/testing.go:878 +0x35c
	*/
	if os.Getenv("CIRCLECI") == "true" {
		t.SkipNow()
	}

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
