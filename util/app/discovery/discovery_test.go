package discovery

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDiscover(t *testing.T) {
	apps, err := Discover("./testdata")
	assert.NoError(t, err)
	assert.Equal(t, map[string]string{
		"foo": "Plugin",
		"bar": "Ksonnet",
		"baz": "Plugin",
	}, apps)
}

func TestAppType(t *testing.T) {
	appType, err := AppType("./testdata/foo")
	assert.NoError(t, err)
	assert.Equal(t, "Plugin", appType)

	appType, err = AppType("./testdata/bar")
	assert.NoError(t, err)
	assert.Equal(t, "Ksonnet", appType)

	appType, err = AppType("./testdata/baz")
	assert.NoError(t, err)
	assert.Equal(t, "Plugin", appType)

	appType, err = AppType("./testdata")
	assert.NoError(t, err)
	assert.Equal(t, "Directory", appType)
}
