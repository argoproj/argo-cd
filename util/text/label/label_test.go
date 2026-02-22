package label

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseLabels(t *testing.T) {
	validLabels := []string{"key=value", "foo=bar", "intuit=inc"}

	result, err := Parse(validLabels)
	require.NoError(t, err)
	assert.Len(t, result, 3)
	assert.Equal(t, "value", result["key"])
	assert.Equal(t, "bar", result["foo"])
	assert.Equal(t, "inc", result["intuit"])

	invalidLabels := []string{"key=value", "too=many=equals"}
	_, err = Parse(invalidLabels)
	require.Error(t, err)

	emptyLabels := []string{}
	result, err = Parse(emptyLabels)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestParseRejectsDeletionSyntax(t *testing.T) {
	// Original Parse() should reject deletion syntax
	_, err := Parse([]string{"x-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deletion syntax not supported")

	_, err = Parse([]string{"x=5", "y-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deletion syntax not supported")

	// ParseMap should accept deletion syntax
	lm, err := ParseMap([]string{"x-"})
	require.NoError(t, err)
	assert.True(t, lm["x"].Delete)
}

func TestParseMap(t *testing.T) {
	// Test basic parsing
	result, err := ParseMap([]string{"x=5", "y=10"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "5", result["x"].Value)
	assert.False(t, result["x"].Delete)
	assert.Equal(t, "10", result["y"].Value)
	assert.False(t, result["y"].Delete)

	// Test deletion syntax
	result, err = ParseMap([]string{"x-"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
	assert.True(t, result["x"].Delete)

	// Test mixed syntax
	result, err = ParseMap([]string{"x=5", "y-"})
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "5", result["x"].Value)
	assert.False(t, result["x"].Delete)
	assert.True(t, result["y"].Delete)

	// Test invalid deletion syntax
	_, err = ParseMap([]string{"-"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid label deletion syntax")

	// Test nil input returns empty map
	result, err = ParseMap(nil)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)

	// Test empty slice returns empty map
	result, err = ParseMap([]string{})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestMapMerge(t *testing.T) {
	// Test deletion
	lm, _ := ParseMap([]string{"x-"})
	result := lm.Merge(map[string]string{"x": "5", "y": "10"})
	assert.Len(t, result, 1)
	assert.Equal(t, "10", result["y"])
	assert.NotContains(t, result, "x")

	// Test update and deletion
	lm, _ = ParseMap([]string{"x=6", "y-"})
	result = lm.Merge(map[string]string{"x": "5", "y": "10"})
	assert.Len(t, result, 1)
	assert.Equal(t, "6", result["x"])

	// Test deletion of non-existing key
	lm, _ = ParseMap([]string{"z-"})
	result = lm.Merge(map[string]string{"x": "5"})
	assert.Len(t, result, 1)
	assert.Equal(t, "5", result["x"])

	// Test update only
	lm, _ = ParseMap([]string{"x=6"})
	result = lm.Merge(map[string]string{"x": "5", "y": "10"})
	assert.Len(t, result, 2)
	assert.Equal(t, "6", result["x"])
	assert.Equal(t, "10", result["y"])
}

func TestMapPlain(t *testing.T) {
	lm, _ := ParseMap([]string{"x=5", "y-", "z=10"})

	result := lm.Plain()
	assert.Len(t, result, 2)
	assert.Equal(t, "5", result["x"])
	assert.Equal(t, "10", result["z"])
	assert.NotContains(t, result, "y")
}

func TestNewMap(t *testing.T) {
	lm := NewMap()
	assert.NotNil(t, lm)
	assert.Empty(t, lm)
}

func TestMapParse(t *testing.T) {
	// Test direct Parse method on Map
	lm := NewMap()
	err := lm.Parse([]string{"x=5", "y=10"})
	require.NoError(t, err)
	assert.Len(t, lm, 2)
	assert.Equal(t, "5", lm["x"].Value)
	assert.False(t, lm["x"].Delete)

	// Test Parse with nil
	lm = NewMap()
	err = lm.Parse(nil)
	require.NoError(t, err)
	assert.Empty(t, lm)

	// Test Parse with empty slice
	lm = NewMap()
	err = lm.Parse([]string{})
	require.NoError(t, err)
	assert.Empty(t, lm)
}
