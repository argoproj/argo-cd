package cache

import (
	"testing"
	"time"

	"github.com/argoproj/argo-cd/v2/image-updater/tag"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_MemCache(t *testing.T) {
	imageName := "foo/bar"
	imageTag := "v1.0.0"
	t.Run("Cache hit", func(t *testing.T) {
		mc := NewMemCache()
		newTag := tag.NewImageTag(imageTag, time.Unix(0, 0), "")
		mc.SetTag(imageName, newTag)
		cachedTag, err := mc.GetTag(imageName, imageTag)
		require.NoError(t, err)
		require.NotNil(t, cachedTag)
		assert.Equal(t, imageTag, cachedTag.TagName)
		assert.True(t, mc.HasTag(imageName, imageTag))
	})

	t.Run("Cache miss", func(t *testing.T) {
		mc := NewMemCache()
		newTag := tag.NewImageTag(imageTag, time.Unix(0, 0), "")
		mc.SetTag(imageName, newTag)
		cachedTag, err := mc.GetTag(imageName, "v1.0.1")
		require.NoError(t, err)
		require.Nil(t, cachedTag)
		assert.False(t, mc.HasTag(imageName, "v1.0.1"))
	})

	t.Run("Cache clear", func(t *testing.T) {
		mc := NewMemCache()
		newTag := tag.NewImageTag(imageTag, time.Unix(0, 0), "")
		mc.SetTag(imageName, newTag)
		cachedTag, err := mc.GetTag(imageName, imageTag)
		require.NoError(t, err)
		require.NotNil(t, cachedTag)
		assert.Equal(t, imageTag, cachedTag.TagName)
		assert.True(t, mc.HasTag(imageName, imageTag))
		mc.ClearCache()
		cachedTag, err = mc.GetTag(imageName, imageTag)
		require.NoError(t, err)
		require.Nil(t, cachedTag)
	})
}
