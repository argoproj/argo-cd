package kube

import (
	"regexp"
	"testing"

	"github.com/argoproj/argo-cd/test"
	"github.com/argoproj/argo-cd/util"

	"github.com/stretchr/testify/assert"
)

func TestConvertToVersion(t *testing.T) {
	kubectl := KubectlCmd{}
	t.Run("AppsDeployment", func(t *testing.T) {
		newObj, err := kubectl.ConvertToVersion(test.UnstructuredFromFile("testdata/appsdeployment.yaml"), "extensions", "v1beta1")
		if assert.NoError(t, err) {
			gvk := newObj.GroupVersionKind()
			assert.Equal(t, "extensions", gvk.Group)
			assert.Equal(t, "v1beta1", gvk.Version)
		}
	})
	t.Run("CustomResource", func(t *testing.T) {
		_, err := kubectl.ConvertToVersion(test.UnstructuredFromFile("testdata/cr.yaml"), "argoproj.io", "v1")
		assert.Error(t, err)
	})
	t.Run("ExtensionsDeployment", func(t *testing.T) {
		obj := test.UnstructuredFromFile("testdata/nginx.yaml")

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

func TestRunKubectl(t *testing.T) {
	callbackExecuted := false
	closerExecuted := false
	kubectl := KubectlCmd{
		func(command string) (util.Closer, error) {
			callbackExecuted = true
			return util.NewCloser(func() error {
				closerExecuted = true
				return nil
			}), nil
		},
	}

	_, _ = kubectl.runKubectl("/dev/null", "default", []string{"command-name"}, nil, false)
	assert.True(t, callbackExecuted)
	assert.True(t, closerExecuted)
}

func TestVersion(t *testing.T) {
	ver, err := Version()
	assert.NoError(t, err)
	SemverRegexValidation := `^v(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*)(\.(0|[1-9]\d*|\d*[a-zA-Z-][0-9a-zA-Z-]*))*)?(\+[0-9a-zA-Z-]+(\.[0-9a-zA-Z-]+)*)?$`
	re := regexp.MustCompile(SemverRegexValidation)
	assert.True(t, re.MatchString(ver))
}
