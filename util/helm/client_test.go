package helm

import (
	"os"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/stretchr/testify/assert"

	"github.com/argoproj/argo-cd/util"
)

func TestIndex(t *testing.T) {
	t.Run("Invalid", func(t *testing.T) {
		client := NewClient("", Creds{})
		_, err := client.GetIndex()
		assert.Error(t, err)
	})
	t.Run("Stable", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", Creds{})
		index, err := client.GetIndex()
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
	t.Run("BasicAuth", func(t *testing.T) {
		client := NewClient("https://argoproj.github.io/argo-helm", Creds{
			Username: "my-password",
			Password: "my-username",
		})
		index, err := client.GetIndex()
		assert.NoError(t, err)
		assert.NotNil(t, index)
	})
}

func Test_nativeHelmChart_ExtractChart(t *testing.T) {
	client := NewClient("https://argoproj.github.io/argo-helm", Creds{})
	path, closer, err := client.ExtractChart("argo-cd", semver.MustParse("0.7.1"))
	assert.NoError(t, err)
	defer util.Close(closer)
	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}
