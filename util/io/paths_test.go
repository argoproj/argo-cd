package io

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPath_SameURLs(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())
	res1, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetPath_DifferentURLs(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())
	res1, err := paths.GetPath("https://localhost/test1.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test2.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetPath_SameURLsDifferentInstances(t *testing.T) {
	paths1 := NewRandomizedTempPaths(os.TempDir())
	res1, err := paths1.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	paths2 := NewRandomizedTempPaths(os.TempDir())
	res2, err := paths2.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetPathIfExists(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())
	t.Run("does not exist", func(t *testing.T) {
		path := paths.GetPathIfExists("https://localhost/test.txt")
		assert.Empty(t, path)
	})
	t.Run("does exist", func(t *testing.T) {
		_, err := paths.GetPath("https://localhost/test.txt")
		require.NoError(t, err)
		path := paths.GetPathIfExists("https://localhost/test.txt")
		assert.NotEmpty(t, path)
	})
}

func TestGetPaths_no_race(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())
	go func() {
		path, err := paths.GetPath("https://localhost/test.txt")
		require.NoError(t, err)
		assert.NotEmpty(t, path)
	}()
	go func() {
		paths.GetPaths()
	}()
}

func TestRemove(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())

	// Add a path
	key := "https://localhost/test.txt"
	path, err := paths.GetPath(key)
	require.NoError(t, err)
	assert.NotEmpty(t, path)

	// Verify it exists
	existingPath := paths.GetPathIfExists(key)
	assert.Equal(t, path, existingPath)

	// Remove it
	paths.Remove(key)

	// Verify it's gone
	removedPath := paths.GetPathIfExists(key)
	assert.Empty(t, removedPath)
}

func TestRemove_NonExistent(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())

	// Removing a non-existent key should not panic
	paths.Remove("https://localhost/does-not-exist.txt")

	// Verify it still doesn't exist
	path := paths.GetPathIfExists("https://localhost/does-not-exist.txt")
	assert.Empty(t, path)
}

func TestRemove_no_race(t *testing.T) {
	paths := NewRandomizedTempPaths(os.TempDir())
	key := "https://localhost/test.txt"
	_, err := paths.GetPath(key)
	require.NoError(t, err)

	go func() {
		paths.Remove(key)
	}()
	go func() {
		paths.GetPathIfExists(key)
	}()
}
