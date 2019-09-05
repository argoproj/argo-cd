package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscover(t *testing.T) {
	apps, err := Discover("./testdata")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Kustomize",
		"bar": "Ksonnet",
		"baz": "Helm",
	}, apps)
}

func TestAppType(t *testing.T) {
	appType, err := AppType("./testdata/foo")
	assert.NoError(t, err)
	assert.Equal(t, "Kustomize", appType)

	appType, err = AppType("./testdata/bar")
	assert.NoError(t, err)
	assert.Equal(t, "Ksonnet", appType)

	appType, err = AppType("./testdata/baz")
	assert.NoError(t, err)
	assert.Equal(t, "Helm", appType)

	appType, err = AppType("./testdata")
	assert.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}
