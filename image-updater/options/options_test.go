package options

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_PlatformKey(t *testing.T) {
	t.Run("Without variant", func(t *testing.T) {
		assert.Equal(t, PlatformKey("linux", "amd64", ""), "linux/amd64")
	})
	t.Run("With variant", func(t *testing.T) {
		assert.Equal(t, PlatformKey("linux", "arm", "v6"), "linux/arm/v6")
	})
}

func Test_WantsPlatform(t *testing.T) {
	opts := NewManifestOptions()
	t.Run("Empty options", func(t *testing.T) {
		assert.True(t, opts.WantsPlatform("linux", "arm", "v7"))
		assert.True(t, opts.WantsPlatform("linux", "amd64", ""))
	})
	t.Run("Single platform", func(t *testing.T) {
		opts = opts.WithPlatform("linux", "arm", "v7")
		assert.True(t, opts.WantsPlatform("linux", "arm", "v7"))
	})
	t.Run("Platform appended", func(t *testing.T) {
		opts = opts.WithPlatform("linux", "arm", "v8")
		assert.True(t, opts.WantsPlatform("linux", "arm", "v7"))
		assert.True(t, opts.WantsPlatform("linux", "arm", "v8"))
	})
	t.Run("Uninitialized options", func(t *testing.T) {
		opts := &ManifestOptions{}
		assert.True(t, opts.WantsPlatform("linux", "amd64", ""))
		opts = (&ManifestOptions{}).WithPlatform("linux", "amd64", "")
		assert.True(t, opts.WantsPlatform("linux", "amd64", ""))
	})
}

func Test_WantsMetadata(t *testing.T) {
	opts := NewManifestOptions()
	t.Run("Empty options", func(t *testing.T) {
		assert.False(t, opts.WantsMetadata())
	})
	t.Run("Wants metadata", func(t *testing.T) {
		opts = opts.WithMetadata(true)
		assert.True(t, opts.WantsMetadata())
	})
	t.Run("Does not want metadata", func(t *testing.T) {
		opts = opts.WithMetadata(false)
		assert.False(t, opts.WantsMetadata())
	})
}

func Test_Platforms(t *testing.T) {
	opts := NewManifestOptions()
	t.Run("Empty platforms returns empty array", func(t *testing.T) {
		ps := opts.Platforms()
		assert.Len(t, ps, 0)
	})
	t.Run("Single platform without variant", func(t *testing.T) {
		ps := opts.WithPlatform("linux", "amd64", "").Platforms()
		require.Len(t, ps, 1)
		assert.Equal(t, ps[0], PlatformKey("linux", "amd64", ""))
	})
	t.Run("Single platform with variant", func(t *testing.T) {
		ps := opts.WithPlatform("linux", "arm", "v8").Platforms()
		require.Len(t, ps, 2)
		assert.Equal(t, ps[0], PlatformKey("linux", "amd64", ""))
		assert.Equal(t, ps[1], PlatformKey("linux", "arm", "v8"))
	})
}
