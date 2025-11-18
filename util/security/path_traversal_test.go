package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnforceToCurrentRoot(t *testing.T) {
	cleanDir, err := EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/values.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/home/argo/helmapp/values.yaml", cleanDir)

	// File is outside current working directory
	_, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/values.yaml")
	require.Error(t, err)

	// File is outside current working directory
	_, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/../differentapp/values.yaml")
	require.Error(t, err)

	// Goes back and forth, but still legal
	cleanDir, err = EnforceToCurrentRoot("/home/argo/helmapp/", "/home/argo/helmapp/../../argo/helmapp/values.yaml")
	require.NoError(t, err)
	assert.Equal(t, "/home/argo/helmapp/values.yaml", cleanDir)
}
