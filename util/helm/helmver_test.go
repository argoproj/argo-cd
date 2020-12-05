package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetHelmVersion_Helm3(t *testing.T) {
	ver, err := getHelmVersion("./testdata/minio")
	assert.NoError(t, err)
	assert.Equal(t, ver.binaryName, HelmV3.binaryName)
}

func TestGetHelmVersion_Helm2(t *testing.T) {
	ver, err := getHelmVersion("./testdata/helm2-dependency")
	assert.NoError(t, err)
	assert.Equal(t, ver.binaryName, HelmV2.binaryName)
}

func TestGetHelmVersion_InvalidVersion(t *testing.T) {
	_, err := getHelmVersion("./testdata/invalid-version")
	assert.Error(t, err)
}
