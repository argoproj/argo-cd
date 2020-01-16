package kube

import (
	"regexp"
	"testing"

	testingutils "github.com/argoproj/argo-cd/engine/pkg/utils/testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToVersion(t *testing.T) {
	kubectl := KubectlCmd{}
	t.Run("AppsDeployment", func(t *testing.T) {
		newObj, err := kubectl.ConvertToVersion(testingutils.UnstructuredFromFile("testdata/appsdeployment.yaml"), "extensions", "v1beta1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "extensions", gvk.Group)
			assert.Equal(t, "v1beta1", gvk.Version)
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

func TestVersion(t *testing.T) {
	ver, err := Version()
	assert.NoError(t, err)
	SemverRegexValidation := `^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)?(\+[0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*)?$`
	re := regexp.MustCompile(SemverRegexValidation)
	assert.True(t, re.MatchString(ver))
}
