package helm

import (
	"os"
	"testing"

	"github.com/Masterminds/semver"
	"github.com/argoproj/gitops-engine/pkg/utils/io"
	"github.com/stretchr/testify/assert"
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
	defer io.Close(closer)
	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.True(t, info.IsDir())
}

func Test_normalizeChartName(t *testing.T) {
	t.Run("Test non-slashed name", func(t *testing.T) {
		n := normalizeChartName("mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test single-slashed name", func(t *testing.T) {
		n := normalizeChartName("myorg/mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test chart name with suborg", func(t *testing.T) {
		n := normalizeChartName("myorg/mysuborg/mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test double-slashed name", func(t *testing.T) {
		n := normalizeChartName("myorg//mychart")
		assert.Equal(t, n, "mychart")
	})
	t.Run("Test invalid chart name - ends with slash", func(t *testing.T) {
		n := normalizeChartName("myorg/")
		assert.Equal(t, n, "myorg/")
	})
	t.Run("Test invalid chart name - is dot", func(t *testing.T) {
		n := normalizeChartName("myorg/.")
		assert.Equal(t, n, "myorg/.")
	})
	t.Run("Test invalid chart name - is two dots", func(t *testing.T) {
		n := normalizeChartName("myorg/..")
		assert.Equal(t, n, "myorg/..")
	})
}
