package security

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestEnforceToCurrentRoot(t *testing.T) {
	cleanDir, err := EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/values.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "/home/argo/helmapp/values.yaml", cleanDir)

	// File is outside current working directory
	cleanDir, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/values.yaml")
	assert.Error(t, err)

	// File is outside current working directory
	cleanDir, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/../differentapp/values.yaml")
	assert.Error(t, err)

	// Goes back and forth, but still legal
	cleanDir, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/../../argo/helmapp/values.yaml")
	assert.NoError(t, err)
	assert.Equal(t, "/home/argo/helmapp/values.yaml", cleanDir)
}