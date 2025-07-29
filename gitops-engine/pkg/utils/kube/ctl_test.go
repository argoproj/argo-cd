package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/klog/v2/klogr"

	testingutils "github.com/argoproj/gitops-engine/pkg/utils/testing"
	"github.com/argoproj/gitops-engine/pkg/utils/tracing"
)

var (
	_ Kubectl = &KubectlCmd{}
)

func TestConvertToVersion(t *testing.T) {
	kubectl := KubectlCmd{
		Log:    klogr.New(),
		Tracer: tracing.NopTracer{},
	}
	t.Run("AppsDeployment", func(t *testing.T) {
		newObj, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/appsdeployment.yaml"), "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}
	})
	t.Run("CustomResource", func(t *testing.T) {
		_, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/cr.yaml"), "argoproj.io", "v1")
		assert.Error(t, err)
	})
	t.Run("ExtensionsDeployment", func(t *testing.T) {
		obj := testingutils.UnstructuredFromFile("testdata/nginx.yaml")

		// convert an extensions/v1beta1 object into itself
		newObj, err := kubectl.ConvertToVersion(obj, "extensions", "v1beta1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "extensions", gvk.Group)
			assert.Equal(t, "v1beta1", gvk.Version)
		}

		// convert an extensions/v1beta1 object into an apps/v1
		newObj, err = kubectl.ConvertToVersion(obj, "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}

		// converting it again should not have any affect
		newObj, err = kubectl.ConvertToVersion(obj, "apps", "v1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "apps", gvk.Group)
			assert.Equal(t, "v1", gvk.Version)
		}
	})
}
