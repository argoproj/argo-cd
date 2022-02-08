package io

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPath_SameURLs(t *testing.T) {
	paths := NewTempPaths()
	res1, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.Equal(t, res1, res2)
}

func TestGetPath_DifferentURLs(t *testing.T) {
	paths := NewTempPaths()
	res1, err := paths.GetPath("https://localhost/test1.txt")
	require.NoError(t, err)
	res2, err := paths.GetPath("https://localhost/test2txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}

func TestGetPath_SameURLsDifferentInstances(t *testing.T) {
	paths1 := NewTempPaths()
	res1, err := paths1.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	paths2 := NewTempPaths()
	res2, err := paths2.GetPath("https://localhost/test.txt")
	require.NoError(t, err)
	assert.NotEqual(t, res1, res2)
}
